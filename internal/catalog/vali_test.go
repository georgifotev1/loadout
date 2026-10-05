package catalog

import (
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/gfotev/loadout/fixtures"
	"github.com/gfotev/loadout/internal/vali"
)

func TestValiCategoriesAreCanonical(t *testing.T) {
	for valiName, c := range valiCategories {
		if !IsCategory(c) {
			t.Errorf("%q maps to %q, which isn't in Categories", valiName, c)
		}
	}
}

// Every fixture lives in a directory named after its canonical category, so the mapping must
// agree with the directory.
func TestFixturesMapToTheirCategory(t *testing.T) {
	files, err := fs.Glob(fixtures.Vali, vali.FixtureGlob)
	if err != nil {
		t.Fatal(err)
	}
	dirs := make(map[string]string, len(files))
	for _, f := range files {
		dirs[strings.TrimSuffix(path.Base(f), ".json")] = path.Base(path.Dir(f))
	}
	for _, p := range loadFixtures(t) {
		got, ok := valiCategory(p)
		if want := dirs[p.SKU]; !ok || got != want {
			t.Errorf("%s: category %q, %v; fixture is in %q (path %v)", p.SKU, got, ok, want, p.CategoryPath)
		}
	}
}

func TestValiCategoryUnknown(t *testing.T) {
	for _, cp := range [][]string{
		nil,
		{"Компютърни компоненти"},
		{"Компютърни компоненти", "Твърди дискове (HDD)"},
	} {
		if c, ok := valiCategory(vali.RawProduct{CategoryPath: cp}); ok {
			t.Errorf("%v: got %q, want not ok", cp, c)
		}
	}
}

func TestValiWarrantyMonths(t *testing.T) {
	tests := []struct {
		in   string
		want int // 0 = nil
	}{
		{"36 месеца", 36},
		{"24 месеца", 24},
		{" 1 месец ", 1},
		{"Доживотна", 0},
		{"", 0},
		{"0 месеца", 0},
		{"abc месеца", 0},
		{"2 години", 0},
	}
	for _, tt := range tests {
		got := valiWarrantyMonths(tt.in)
		switch {
		case tt.want == 0 && got != nil:
			t.Errorf("%q: got %d, want nil", tt.in, *got)
		case tt.want != 0 && (got == nil || *got != tt.want):
			t.Errorf("%q: got %v, want %d", tt.in, got, tt.want)
		}
	}
}
