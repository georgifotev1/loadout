package specs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// The canonical specs per builder category: what the builder's compatibility rules need. JSON
// tags are the canonical field names, used in Input.Values, ProductSpecs.Specs and
// spec_fields.field. A nil pointer or slice means unknown; an empty, non-nil slice means known
// to be empty (a case that takes no radiators).
//
// Add a field when a feature needs it; adapters fill what they can and admins the rest.
// Lengths are whole millimetres.

type CPU struct {
	Socket         *string  `json:"socket"`
	TDPW           *int     `json:"tdp_w"`
	IGPU           *bool    `json:"igpu"`
	CoolerIncluded *bool    `json:"cooler_included"`
	MemoryTypes    []string `json:"memory_types"` // derived from the socket when not given
	Generation     *string  `json:"generation"`   // e.g. ryzen-7000, core-14, core-ultra-2
}

type Motherboard struct {
	Socket                  *string  `json:"socket"`
	Chipset                 *string  `json:"chipset"`
	FormFactor              *string  `json:"form_factor"` // ITX, mATX, ATX, E-ATX
	MemoryType              *string  `json:"memory_type"` // DDR4, DDR5
	MemorySlots             *int     `json:"memory_slots"`
	MaxMemoryGB             *int     `json:"max_memory_gb"`
	MaxMemoryMHz            *int     `json:"max_memory_mhz"`
	M2Slots                 *int     `json:"m2_slots"`
	SupportedCPUGenerations []string `json:"supported_cpu_generations"`
}

type RAM struct {
	Type            *string `json:"type"`
	CapacityGBTotal *int    `json:"capacity_gb_total"` // the whole kit
	Modules         *int    `json:"modules"`
	SpeedMHz        *int    `json:"speed_mhz"`
}

type GPU struct {
	LengthMM        *int     `json:"length_mm"`
	TDPW            *int     `json:"tdp_w"`
	PowerConnectors []string `json:"power_connectors"` // one entry per plug: [8-pin 8-pin]
}

type PSU struct {
	WattageW        *int    `json:"wattage_w"`
	FormFactor      *string `json:"form_factor"` // ATX, SFX, SFX-L
	PCIe12V2x6Count *int    `json:"pcie_12v2x6_count"`
}

type Case struct {
	FormFactorsSupported []string `json:"form_factors_supported"`
	MaxGPULengthMM       *int     `json:"max_gpu_length_mm"`
	MaxCoolerHeightMM    *int     `json:"max_cooler_height_mm"`
	PSUFormFactor        *string  `json:"psu_form_factor"`  // the largest PSU it takes
	RadiatorSupport      []string `json:"radiator_support"` // position:size, e.g. top:360
}

type Cooler struct {
	Type       *string  `json:"type"` // air, aio
	Sockets    []string `json:"sockets"`
	HeightMM   *int     `json:"height_mm"`   // air coolers
	RadiatorMM *int     `json:"radiator_mm"` // AIOs: 240, 360, ...
	TDPRatingW *int     `json:"tdp_rating_w"`
}

type Storage struct {
	Interface  *string `json:"interface"`   // nvme, sata
	FormFactor *string `json:"form_factor"` // m2_2280, 2.5, ...
	CapacityGB *int    `json:"capacity_gb"`
}

// Cooler types.
const (
	CoolerAir = "air"
	CoolerAIO = "aio"
)

// Decode converts a product's specs map into its category's struct, e.g. Decode[CPU](specs).
// Unknown fields and wrong types are an error, so the map and the struct can't drift apart.
func Decode[T any](specs map[string]any) (T, error) {
	var v T
	b, err := json.Marshal(specs)
	if err != nil {
		return v, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, fmt.Errorf("specs: decode %T: %w", v, err)
	}
	return v, nil
}

// schema is how one builder category's specs are checked and completed.
type schema struct {
	fields []string // the JSON field names of the category's struct
	// validate decodes specs into the category's struct: a wrong name or type is an error.
	validate func(specs map[string]any) error
	// required lists the fields the builder can't work without, given what's known (a cooler
	// needs a height if it's an air cooler, a radiator size if it's an AIO).
	required func(specs map[string]any) []string
	// derive fills fields that follow from others. It must not overwrite a value.
	derive func(specs map[string]any)
}

func schemaOf[T any](required func(map[string]any) []string, derive func(map[string]any)) schema {
	var zero T
	return schema{
		fields:   jsonFields(zero),
		validate: func(m map[string]any) error { _, err := Decode[T](m); return err },
		required: required,
		derive:   derive,
	}
}

func always(fields ...string) func(map[string]any) []string {
	return func(map[string]any) []string { return fields }
}

// schemas holds the builder categories. Other categories (peripherals) have no canonical specs.
var schemas = map[string]schema{
	"cpu":         schemaOf[CPU](always("socket", "tdp_w", "igpu", "cooler_included", "memory_types"), deriveCPU),
	"motherboard": schemaOf[Motherboard](always("socket", "chipset", "form_factor", "memory_type", "memory_slots", "m2_slots"), nil),
	"ram":         schemaOf[RAM](always("type", "capacity_gb_total", "modules", "speed_mhz"), nil),
	"gpu":         schemaOf[GPU](always("length_mm", "tdp_w"), nil),
	"psu":         schemaOf[PSU](always("wattage_w", "form_factor"), nil),
	"case":        schemaOf[Case](always("form_factors_supported", "max_gpu_length_mm", "max_cooler_height_mm", "psu_form_factor"), nil),
	"cooler": schemaOf[Cooler](func(specs map[string]any) []string {
		req := []string{"type", "sockets"}
		switch specs["type"] {
		case CoolerAir:
			req = append(req, "height_mm")
		case CoolerAIO:
			req = append(req, "radiator_mm")
		}
		return req
	}, nil),
	"storage": schemaOf[Storage](always("interface", "form_factor", "capacity_gb"), nil),
}

// Categories lists the categories with canonical specs: the PC builder's parts.
func Categories() []string {
	return slices.Sorted(maps.Keys(schemas))
}

// socketMemory is the memory a CPU socket's processors support. LGA1700 CPUs do both DDR4 and
// DDR5; the motherboard decides which one a build uses.
var socketMemory = map[string][]string{
	"AM4":     {"DDR4"},
	"AM5":     {"DDR5"},
	"LGA1200": {"DDR4"},
	"LGA1700": {"DDR4", "DDR5"},
	"LGA1851": {"DDR5"},
}

// deriveCPU fills memory_types from the socket, so neither an adapter nor an admin has to.
func deriveCPU(specs map[string]any) {
	if _, ok := specs["memory_types"]; ok {
		return
	}
	if socket, ok := specs["socket"].(string); ok {
		if mem, ok := socketMemory[socket]; ok {
			specs["memory_types"] = slices.Clone(mem)
		}
	}
}

// resolve combines a product's field values into its effective specs: per field the admin
// value, else the supplier's. Then derivations fill what follows from those, and the status
// says whether every required field has a value. Fields the category doesn't define (left over
// from a category change) are ignored.
func resolve(category string, rows []FieldValue) (specs map[string]any, missing []string, status Status) {
	sc := schemas[category]
	specs = map[string]any{}
	for _, r := range rows {
		if !slices.Contains(sc.fields, r.Field) {
			continue
		}
		if _, set := specs[r.Field]; !set || r.Source == SourceAdmin {
			specs[r.Field] = r.Value
		}
	}
	if sc.derive != nil {
		sc.derive(specs)
	}
	for _, f := range sc.required(specs) {
		if _, ok := specs[f]; !ok {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return specs, missing, StatusIncomplete
	}
	return specs, nil, StatusApproved
}

// jsonFields lists the JSON field names of a struct.
func jsonFields(v any) []string {
	t := reflect.TypeOf(v)
	names := make([]string, 0, t.NumField())
	for f := range t.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		names = append(names, name)
	}
	return names
}
