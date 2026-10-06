package specs

import (
	"errors"
	"slices"
	"testing"

	"github.com/gfotev/loadout/internal/platform/validate"
)

func TestSchemasAreConsistent(t *testing.T) {
	for name, sc := range schemas {
		for _, f := range append(sc.required(map[string]any{"type": CoolerAir}), sc.required(map[string]any{"type": CoolerAIO})...) {
			if !slices.Contains(sc.fields, f) {
				t.Errorf("%s: required field %q isn't a field of the category struct", name, f)
			}
		}
	}
}

func TestValidate(t *testing.T) {
	cpu := schemas["cpu"]
	tests := []struct {
		name   string
		values map[string]any
		ok     bool
	}{
		{"nothing", nil, true},
		{"known fields, right types", map[string]any{"socket": "AM4", "tdp_w": 65, "igpu": false, "memory_types": []string{"DDR4"}}, true},
		{"unknown field", map[string]any{"cores": 4}, false},
		{"wrong type", map[string]any{"tdp_w": "65 W"}, false},
		{"fraction for an int", map[string]any{"tdp_w": 65.5}, false},
	}
	for _, tt := range tests {
		if err := cpu.validate(tt.values); (err == nil) != tt.ok {
			t.Errorf("%s: err = %v, want ok=%v", tt.name, err, tt.ok)
		}
	}
}

func TestCheckAdmin(t *testing.T) {
	cpu := schemas["cpu"]
	tests := []struct {
		name   string
		values map[string]any
		want   string // the field error, "" for none
	}{
		{"right types from JSON", map[string]any{"socket": "AM5", "tdp_w": 65.0, "igpu": true, "memory_types": []any{"DDR5"}}, ""},
		{"nil clears", map[string]any{"socket": nil}, ""},
		{"zero is a value", map[string]any{"tdp_w": 0}, ""},
		{"empty list is a value", map[string]any{"memory_types": []any{}}, ""},
		{"unknown field", map[string]any{"cores": 8}, "cores: unknown"},
		{"wrong type", map[string]any{"tdp_w": "65 W"}, "tdp_w: invalid_type"},
		{"fraction", map[string]any{"tdp_w": 65.5}, "tdp_w: invalid_type"},
		{"negative", map[string]any{"tdp_w": -1.0}, "tdp_w: out_of_range"},
		{"blank string", map[string]any{"socket": "  "}, "socket: required"},
		{"blank list entry", map[string]any{"memory_types": []any{"DDR5", ""}}, "memory_types: required"},
	}
	for _, tt := range tests {
		err := cpu.checkAdmin(tt.values)
		var got string
		var verr *validate.Error
		if errors.As(err, &verr) {
			got = verr.Fields[0].Field + ": " + verr.Fields[0].Code
		} else if err != nil {
			got = err.Error()
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFieldsOf(t *testing.T) {
	ps := ProductSpecs{
		Category: "cooler",
		Specs:    map[string]any{"type": CoolerAir, "sockets": []any{"AM5"}},
	}
	rows := []FieldValue{
		{Field: "type", Source: SourceSupplier, Value: CoolerAIO},
		{Field: "type", Source: SourceAdmin, Value: CoolerAir},
		{Field: "sockets", Source: SourceSupplier, Value: []any{"AM5"}},
	}
	fields := fieldsOf(ps, rows)
	if len(fields.Fields) != len(schemas["cooler"].fields) || fields.Fields[0].Name != "type" {
		t.Fatalf("fields %+v", fields.Fields)
	}
	byName := map[string]Field{}
	for _, f := range fields.Fields {
		byName[f.Name] = f
	}
	if f := byName["type"]; f.Kind != KindString || !f.Required || f.Supplier != CoolerAIO || f.Admin != CoolerAir || f.Value != CoolerAir {
		t.Errorf("type %+v", f)
	}
	// An air cooler needs a height, not a radiator size.
	if f := byName["height_mm"]; f.Kind != KindInteger || !f.Required || f.Value != nil {
		t.Errorf("height_mm %+v", f)
	}
	if f := byName["radiator_mm"]; f.Required {
		t.Errorf("radiator_mm %+v", f)
	}
	if f := byName["sockets"]; f.Kind != KindStringList || f.Admin != nil {
		t.Errorf("sockets %+v", f)
	}
}

func TestResolve(t *testing.T) {
	v := func(field string, src Source, val any) FieldValue {
		return FieldValue{Field: field, Source: src, Value: val}
	}
	// A CPU with every required field from the supplier (memory types are derived).
	complete := []FieldValue{
		v("socket", SourceSupplier, "AM4"),
		v("tdp_w", SourceSupplier, 65),
		v("igpu", SourceSupplier, false),
		v("cooler_included", SourceSupplier, true),
	}
	without := func(field string) []FieldValue {
		return slices.DeleteFunc(slices.Clone(complete), func(f FieldValue) bool { return f.Field == field })
	}

	tests := []struct {
		name        string
		category    string
		rows        []FieldValue
		wantStatus  Status
		wantMissing []string
		wantSpecs   map[string]any // only the fields checked
	}{
		{
			name:       "all required present",
			category:   "cpu",
			rows:       complete,
			wantStatus: StatusApproved,
			wantSpecs:  map[string]any{"socket": "AM4"},
		},
		{
			name:        "required field missing",
			category:    "cpu",
			rows:        without("tdp_w"),
			wantStatus:  StatusIncomplete,
			wantMissing: []string{"tdp_w"},
		},
		{
			name:       "admin fills a gap",
			category:   "cpu",
			rows:       append(without("tdp_w"), v("tdp_w", SourceAdmin, 65)),
			wantStatus: StatusApproved,
			wantSpecs:  map[string]any{"tdp_w": 65},
		},
		{
			name:       "admin beats supplier, in either order",
			category:   "cpu",
			rows:       append([]FieldValue{v("tdp_w", SourceAdmin, 105)}, complete...),
			wantStatus: StatusApproved,
			wantSpecs:  map[string]any{"tdp_w": 105},
		},
		{
			name:        "memory types follow from the socket",
			category:    "cpu",
			rows:        []FieldValue{v("socket", SourceSupplier, "LGA1700")},
			wantStatus:  StatusIncomplete,
			wantMissing: []string{"tdp_w", "igpu", "cooler_included"},
		},
		{
			name:        "unknown socket derives nothing",
			category:    "cpu",
			rows:        without("socket"),
			wantStatus:  StatusIncomplete,
			wantMissing: []string{"socket", "memory_types"},
		},
		{
			name:       "a given value isn't overwritten by derivation",
			category:   "cpu",
			rows:       append(slices.Clone(complete), v("memory_types", SourceAdmin, []string{"DDR4", "DDR5"})),
			wantStatus: StatusApproved,
		},
		{
			name:       "fields from another category are ignored",
			category:   "cpu",
			rows:       append(slices.Clone(complete), v("chipset", SourceAdmin, "B550")),
			wantStatus: StatusApproved,
			wantSpecs:  map[string]any{"chipset": nil},
		},
		{
			name:        "air cooler needs a height",
			category:    "cooler",
			rows:        []FieldValue{v("type", SourceSupplier, CoolerAir), v("sockets", SourceSupplier, []string{"AM4"}), v("radiator_mm", SourceSupplier, 240)},
			wantStatus:  StatusIncomplete,
			wantMissing: []string{"height_mm"},
		},
		{
			name:       "AIO needs a radiator size, not a height",
			category:   "cooler",
			rows:       []FieldValue{v("type", SourceSupplier, CoolerAIO), v("sockets", SourceSupplier, []string{"AM4"}), v("radiator_mm", SourceSupplier, 240)},
			wantStatus: StatusApproved,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs, missing, status := resolve(tt.category, tt.rows)
			if status != tt.wantStatus {
				t.Errorf("status = %s, want %s", status, tt.wantStatus)
			}
			if !slices.Equal(missing, tt.wantMissing) {
				t.Errorf("missing = %v, want %v", missing, tt.wantMissing)
			}
			for f, want := range tt.wantSpecs {
				if got := specs[f]; got != want {
					t.Errorf("specs[%s] = %v, want %v", f, got, want)
				}
			}
		})
	}

	t.Run("derived memory types", func(t *testing.T) {
		specs, _, _ := resolve("cpu", []FieldValue{v("socket", SourceSupplier, "LGA1700")})
		if got, _ := specs["memory_types"].([]string); !slices.Equal(got, []string{"DDR4", "DDR5"}) {
			t.Errorf("memory_types = %v, want [DDR4 DDR5]", specs["memory_types"])
		}
	})
}
