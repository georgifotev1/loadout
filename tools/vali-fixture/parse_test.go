package main

import (
	"strings"
	"testing"
)

// A trimmed copy of the structure of a real vali.bg product page (2026-09-30).
const samplePage = `<html><head>
<script type="application/ld+json">{"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[
 {"@type":"ListItem","position":1,"name":"Начало","item":"https://www.vali.bg"},
 {"@type":"ListItem","position":2,"name":"Компютърни компоненти","item":"x"},
 {"@type":"ListItem","position":3,"name":"Дънни платки","item":"x"},
 {"@type":"ListItem","position":4,"name":"Дънна платка GIGABYTE A520M K V2","item":"x"}]}</script>
<script type="application/ld+json">{"@context":"https://schema.org/","@type":"Product",
 "name":"Дънна платка GIGABYTE A520M K V2, Socket AM4","image":["https://www.vali.bg/a.jpg"],
 "sku":"GA-MB-A520M-K-V2","brand":{"@type":"Thing","name":"GIGABYTE"},
 "offers":{"@type":"Offer","priceCurrency":"BGN","price":"61.5"}}</script>
</head><body>
<table class="parameters-table"><tbody>
<tr><td> Производител </td><td> GIGABYTE </td></tr>
<tr><td>Гаранция</td><td>36 месеца</td></tr>
<tr><td>Продуктов код</td><td>A520M K V2</td></tr>
<tr><td>Баркод</td><td>4719331852771</td></tr>
<tr><td>Слотoве PCI-E x16</td><td> <div>1 x PCIe 3.0 x16</div> </td></tr>
<tr><td>USB портове</td><td> <div>4 x USB 3.2 Gen 1</div> <div>2 x USB 2.0 </div> </td></tr>
<tr><td>Конектор захранване</td><td> </td></tr>
<tr><td>Тегло</td><td>1.000 kg</td></tr>
</tbody></table></body></html>`

func TestParsePageAndFixture(t *testing.T) {
	p, err := parsePage(strings.NewReader(samplePage))
	if err != nil {
		t.Fatal(err)
	}
	f, err := toFixture(p, "https://www.vali.bg/dienna-platka")
	if err != nil {
		t.Fatal(err)
	}

	if f.SKU != "GA-MB-A520M-K-V2" || f.Brand != "GIGABYTE" || f.EAN != "4719331852771" ||
		f.ManufacturerCode != "A520M K V2" || f.Warranty != "36 месеца" {
		t.Errorf("top-level fields = %+v", f)
	}
	if got := strings.Join(f.CategoryPath, " > "); got != "Компютърни компоненти > Дънни платки" {
		t.Errorf("CategoryPath = %q", got)
	}
	if f.RRPCents != 6150 {
		t.Errorf("RRPCents = %d, want 6150", f.RRPCents)
	}
	if f.DealerPriceCents <= 0 || f.DealerPriceCents >= f.RRPCents {
		t.Errorf("DealerPriceCents = %d, want 0 < x < %d", f.DealerPriceCents, f.RRPCents)
	}

	wantSpecs := map[string]string{
		"Слотoве PCI-E x16":   "1 x PCIe 3.0 x16", // Latin "o" kept as-is
		"USB портове":         "4 x USB 3.2 Gen 1, 2 x USB 2.0",
		"Конектор захранване": "",
		"Тегло":               "1.000 kg",
	}
	if len(f.Specs) != len(wantSpecs) {
		t.Errorf("Specs = %v, want %v", f.Specs, wantSpecs)
	}
	for k, want := range wantSpecs {
		if got, ok := f.Specs[k]; !ok || got != want {
			t.Errorf("Specs[%q] = %q (present %v), want %q", k, got, ok, want)
		}
	}
}

func TestMockCommercialIsDeterministic(t *testing.T) {
	a := &fixture{SKU: "X-1", RRPCents: 10000}
	b := &fixture{SKU: "X-1", RRPCents: 10000}
	mockCommercial(a)
	mockCommercial(b)
	if a.DealerPriceCents != b.DealerPriceCents || a.Stock["Sofia-Central"] != b.Stock["Sofia-Central"] {
		t.Errorf("same SKU gave different mock values: %+v vs %+v", a, b)
	}
}

func TestParseCents(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"62.33", 6233, false},
		{"61.5", 6150, false},
		{"1234", 123400, false},
		{" 7.05 ", 705, false},
		{"1.234", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseCents(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("parseCents(%q) = %d, %v; want %d, err %v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}
