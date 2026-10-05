package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/gfotev/loadout/fixtures"
	"github.com/gfotev/loadout/internal/configurator"
)

// newBuilderApp returns an app over every fixture product, with the seed specs stored.
func newBuilderApp(t *testing.T) *application {
	t.Helper()
	app := newCatalogApp(t)
	var out bytes.Buffer
	if err := seedSpecs(context.Background(), fixtures.Specs, app.catalog, app.specs, &out); err != nil {
		t.Fatal(err)
	}
	// Every builder part in the fixtures has seed specs, and they're complete.
	if !strings.HasSuffix(out.String(), "seeded 47 products: 47 approved, 0 incomplete\n") {
		t.Fatalf("seed output:\n%s", out.String())
	}
	app.configurator = configurator.NewService(app.db, app.catalog, app.specs)
	return app
}

// slugs maps fixture SKUs to product slugs.
func slugs(t *testing.T, app *application) map[string]string {
	t.Helper()
	rows, err := app.db.Query(context.Background(), `SELECT provider_sku, slug FROM catalog.products`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var sku, slug string
		if err := rows.Scan(
			&sku,
			&slug,
		); err != nil {
			t.Fatal(err)
		}
		m[sku] = slug
	}
	return m
}

func decodeBody[T any](t *testing.T, target string, app *application) T {
	t.Helper()
	rec := get(t, app, target)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: status %d: %s", target, rec.Code, rec.Body)
	}
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func rules(fs []Finding) []FindingRule {
	ids := []FindingRule{}
	for _, f := range fs {
		ids = append(ids, f.Rule)
	}
	return ids
}

func TestBuilderAPI(t *testing.T) {
	app := newBuilderApp(t)
	slug := slugs(t, app)

	// A Ryzen 5 7600 gaming build that fits together.
	good := url.Values{
		"cpu":         {slug["AMD-AM5-R5-7600-BOX"]},
		"motherboard": {slug["GA-MB-B650-EAGLE-AX"]},
		"ram":         {slug["KIN-RAM-KF560C40BBK2-32"]},
		"gpu":         {slug["GA-VC-N5070WF3-12GD"]},
		"storage":     {slug["KIN-SSD-SNV3S-1000G"]},
		"psu":         {slug["CRS-PSU-9020296-EU"]},
		"case":        {slug["LL-CASE-GELI-216X"]},
		"cooler":      {slug["ARCTIC-FAN-ACFRE00123A"]},
	}
	with := func(v url.Values, key string, values ...string) url.Values {
		c := cloneValues(v)
		if values == nil {
			c.Del(key)
		} else {
			c[key] = values
		}
		return c
	}

	t.Run("a complete build that fits", func(t *testing.T) {
		body := decodeBody[BuildCheck](t, "/api/builder/check?"+good.Encode(), app)
		if len(body.Errors) != 0 || len(body.Warnings) != 0 {
			t.Fatalf("errors %+v warnings %+v", body.Errors, body.Warnings)
		}
		if len(body.Parts) != 8 || body.Parts[0].Slot != SlotCpu || body.Parts[0].Product.Slug != good.Get("cpu") {
			t.Errorf("parts %+v", body.Parts)
		}
		var total int64
		for _, p := range body.Parts {
			total += p.Product.Price.AmountCents
		}
		if body.Total.AmountCents != total || total == 0 {
			t.Errorf("total %d, parts add up to %d", body.Total.AmountCents, total)
		}
		// 98 (65 W × 1.5) + 250 + 50 + 10.
		if body.EstimatedPowerW != 408 || body.RecommendedPsuW != 550 {
			t.Errorf("power %d W, recommended %d W", body.EstimatedPowerW, body.RecommendedPsuW)
		}
	})

	t.Run("Ryzen 3 4100 without a GPU", func(t *testing.T) {
		q := with(with(good, "cpu", slug["AMD-AM4-R3-RYZ-4100-BOX"]), "gpu")
		body := decodeBody[BuildCheck](t, "/api/builder/check?"+q.Encode(), app)
		// The B650 board is AM5, and the 4100 has no integrated graphics.
		if got := rules(body.Errors); !slices.Equal(got, []FindingRule{FindingRuleCPUMBSOCKET, FindingRuleGPUREQUIRED}) {
			t.Errorf("errors %v", got)
		}
	})

	t.Run("two kits of one stick, repeated slug", func(t *testing.T) {
		q := with(good, "ram", slug["KIN-RAM-KF552C36BBE-16"])
		body := decodeBody[BuildCheck](t, "/api/builder/check?"+q.Encode(), app)
		if got := rules(body.Warnings); !slices.Equal(got, []FindingRule{FindingRuleRAMDUALCHANNEL}) {
			t.Errorf("warnings %v", got)
		}
		q = with(good, "ram", slug["KIN-RAM-KF552C36BBE-16"], slug["KIN-RAM-KF552C36BBE-16"])
		body = decodeBody[BuildCheck](t, "/api/builder/check?"+q.Encode(), app)
		if len(body.Warnings) != 0 || len(body.Parts) != 9 {
			t.Errorf("warnings %+v, %d parts", body.Warnings, len(body.Parts))
		}
	})

	t.Run("empty build", func(t *testing.T) {
		body := decodeBody[BuildCheck](t, "/api/builder/check", app)
		if len(body.Parts) != 0 || len(body.Errors) != 6 || body.EstimatedPowerW != 0 || body.Total.AmountCents != 0 {
			t.Errorf("body %+v", body)
		}
	})

	t.Run("unavailable part", func(t *testing.T) {
		q := with(good, "gpu", "no-such-gpu")
		body := decodeBody[BuildCheck](t, "/api/builder/check?"+q.Encode(), app)
		if len(body.Unavailable) != 1 || body.Unavailable[0].Slug != "no-such-gpu" || len(body.Parts) != 7 {
			t.Errorf("unavailable %+v, %d parts", body.Unavailable, len(body.Parts))
		}
	})

	t.Run("bad builds are 422", func(t *testing.T) {
		for _, target := range []string{
			"/api/builder/check?" + with(good, "cpu", good.Get("gpu")).Encode(),         // a GPU as the CPU
			"/api/builder/check?" + with(good, "ram", "a", "b", "c", "d", "e").Encode(), // too many kits
			"/api/builder/check?" + with(good, "storage", "").Encode(),                  // empty slug
			"/api/builder/options?" + with(good, "slot", "toaster").Encode(),
			"/api/builder/options?" + good.Encode(), // no slot
		} {
			if rec := get(t, app, target); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status %d, want 422", target, rec.Code)
			}
		}
	})

	optionSKUs := func(t *testing.T, q url.Values) []string {
		t.Helper()
		body := decodeBody[BuilderOptions](t, "/api/builder/options?"+q.Encode(), app)
		bySlug := map[string]string{}
		for sku, s := range slug {
			bySlug[s] = sku
		}
		skus := []string{}
		for _, o := range body.Options {
			skus = append(skus, bySlug[o.Product.Slug])
		}
		return skus
	}

	t.Run("options: every CPU for an empty build, including ones that need a GPU", func(t *testing.T) {
		got := optionSKUs(t, url.Values{"slot": {"cpu"}})
		if len(got) != 7 || !slices.Contains(got, "AMD-AM4-R3-RYZ-4100-BOX") {
			t.Errorf("options %v", got)
		}
	})

	t.Run("options: AM5 boards for a Ryzen 5 7600", func(t *testing.T) {
		got := optionSKUs(t, url.Values{"slot": {"motherboard"}, "cpu": {good.Get("cpu")}})
		slices.Sort(got)
		if !slices.Equal(got, []string{"GA-MB-B650-EAGLE-AX", "GA-MB-B650M-D3HP"}) {
			t.Errorf("options %v", got)
		}
	})

	t.Run("options: the slot's current part is replaced, not kept", func(t *testing.T) {
		got := optionSKUs(t, with(good, "slot", "motherboard"))
		slices.Sort(got)
		if !slices.Equal(got, []string{"GA-MB-B650-EAGLE-AX", "GA-MB-B650M-D3HP"}) {
			t.Errorf("options %v", got)
		}
	})

	t.Run("options: GPUs that fit a Jonsbo NV10", func(t *testing.T) {
		got := optionSKUs(t, url.Values{"slot": {"gpu"}, "case": {slug["JONSBO-GEJB-161"]}})
		if !slices.Equal(got, []string{"GA-VC-N5050-OC-8GL"}) {
			t.Errorf("options %v", got)
		}
	})

	t.Run("options: cheapest first, with their warnings", func(t *testing.T) {
		body := decodeBody[BuilderOptions](t, "/api/builder/options?"+with(good, "slot", "ram").Encode(), app)
		for i := 1; i < len(body.Options); i++ {
			if body.Options[i].Product.Price.AmountCents < body.Options[i-1].Product.Price.AmountCents {
				t.Errorf("not sorted by price: %+v", body.Options)
			}
		}
		for _, o := range body.Options {
			if o.Product.Slug == slug["KIN-RAM-KF552C36BBE-16"] && !slices.Equal(rules(o.Findings), []FindingRule{FindingRuleRAMDUALCHANNEL}) {
				t.Errorf("single stick findings %+v", o.Findings)
			}
			if o.Product.Slug == slug["KIN-RAM-KF432C16RB2AK2-16"] {
				t.Error("DDR4 kit offered for a DDR5 board")
			}
		}
	})
}

func cloneValues(v url.Values) url.Values {
	c := url.Values{}
	for k, vs := range v {
		c[k] = slices.Clone(vs)
	}
	return c
}

func TestFindingRuleEnumCoversRules(t *testing.T) {
	for _, r := range configurator.Rules {
		if !FindingRule(r.ID).Valid() {
			t.Errorf("rule %q is missing from the FindingRule enum in api/openapi.yaml", r.ID)
		}
	}
	if !FindingRule(configurator.MissingSpec).Valid() {
		t.Error("MISSING_SPEC is missing from the FindingRule enum")
	}
	for _, s := range configurator.Slots {
		if !Slot(s).Valid() {
			t.Errorf("slot %q is missing from the Slot enum", s)
		}
	}
}
