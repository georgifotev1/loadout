package fixtures

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var builderCategories = []string{"cpu", "motherboard", "ram", "gpu", "psu", "case", "cooler", "storage"}

// minPerCategory is the coverage the fixtures must have, so the builder rules have
// something to bite on.
var minPerCategory = map[string]int{
	"cpu": 4, "motherboard": 4, "ram": 2, "gpu": 5, "psu": 4, "case": 4, "cooler": 2, "storage": 2,
}

type product struct {
	file     string
	category string
	SKU      string `json:"sku"`
	URL      string `json:"url"`
	Dealer   int64  `json:"dealer_price_cents"`
	RRP      int64  `json:"rrp_cents"`
}

func loadAll(t *testing.T) (fs.FS, []product) {
	t.Helper()
	fsys := os.DirFS("vali")
	files, err := fs.Glob(fsys, "*/*.json")
	if err != nil {
		t.Fatal(err)
	}
	var ps []product
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			t.Fatal(err)
		}
		p := product{file: f, category: path.Dir(f)}
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		ps = append(ps, p)
	}
	return fsys, ps
}

func TestFixturesMatchSchema(t *testing.T) {
	fsys, ps := loadAll(t)

	c := jsonschema.NewCompiler()
	c.AssertFormat()
	schemaFile, err := os.Open("vali/product.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	defer schemaFile.Close()
	doc, err := jsonschema.UnmarshalJSON(schemaFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddResource("product.schema.json", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile("product.schema.json")
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range ps {
		b, err := fs.ReadFile(fsys, p.file)
		if err != nil {
			t.Fatal(err)
		}
		inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: %v", p.file, err)
		}
		if err := schema.Validate(inst); err != nil {
			t.Errorf("%s: %v", p.file, err)
		}
	}
}

func TestFixturesAreConsistent(t *testing.T) {
	_, ps := loadAll(t)

	known := map[string]bool{}
	for _, c := range builderCategories {
		known[c] = true
	}
	seen := map[string]string{}
	for _, p := range ps {
		if !known[p.category] && !strings.HasPrefix(p.category, "peripheral.") {
			t.Errorf("%s: unknown category directory %q", p.file, p.category)
		}
		if want := p.category + "/" + p.SKU + ".json"; p.file != want {
			t.Errorf("%s: file name should be %s", p.file, want)
		}
		if other, dup := seen[p.SKU]; dup {
			t.Errorf("%s: duplicate sku, also in %s", p.file, other)
		}
		seen[p.SKU] = p.file
		// Pricing tests assume the dealer price is below the public one.
		if p.Dealer >= p.RRP {
			t.Errorf("%s: dealer_price_cents %d >= rrp_cents %d", p.file, p.Dealer, p.RRP)
		}
	}
}

func TestFixtureCoverage(t *testing.T) {
	_, ps := loadAll(t)

	counts := map[string]int{}
	var builder, peripherals int
	for _, p := range ps {
		counts[p.category]++
		if strings.HasPrefix(p.category, "peripheral.") {
			peripherals++
		} else {
			builder++
		}
	}
	for c, min := range minPerCategory {
		if counts[c] < min {
			t.Errorf("%s: %d fixtures, want at least %d", c, counts[c], min)
		}
	}
	if builder < 40 {
		t.Errorf("%d builder fixtures, want at least 40", builder)
	}
	if peripherals < 20 {
		t.Errorf("%d peripheral fixtures, want at least 20", peripherals)
	}
}

// Every approved URL has exactly one fixture and every fixture comes from an approved URL.
func TestFixturesMatchSources(t *testing.T) {
	_, ps := loadAll(t)

	f, err := os.Open("vali/sources.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sources := map[string]string{} // url -> category
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		if fields := strings.Fields(line); len(fields) == 2 {
			sources[fields[1]] = fields[0]
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}

	byURL := map[string]product{}
	for _, p := range ps {
		cat, ok := sources[p.URL]
		switch {
		case !ok:
			t.Errorf("%s: url %s is not in sources.txt", p.file, p.URL)
		case cat != p.category:
			t.Errorf("%s: sources.txt says category %q", p.file, cat)
		}
		byURL[p.URL] = p
	}
	for url := range sources {
		if _, ok := byURL[url]; !ok {
			t.Errorf("sources.txt: no fixture for %s (run go run ./tools/vali-fixture)", url)
		}
	}
}
