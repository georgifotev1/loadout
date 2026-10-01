package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// page is what we read from a public vali.bg product page.
type page struct {
	SKU          string
	Brand        string
	Name         string
	Images       []string
	CategoryPath []string
	PriceCents   int64
	// Rows from the "Технически характеристики" table, keys and values exactly as shown
	// (only trimmed). Normalization is Phase 2's job, so e.g. the Latin "o" in "Слотoве" stays.
	Rows map[string]string
}

// parsePage extracts the product from the JSON-LD blocks (Product, BreadcrumbList) and the
// parameters table.
func parsePage(r io.Reader) (*page, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	p := &page{Rows: map[string]string{}}
	var sawProduct, sawTable bool

	for n := range doc.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		switch {
		case n.Data == "script" && attr(n, "type") == "application/ld+json":
			ok, err := p.readLD(text(n))
			if err != nil {
				return nil, fmt.Errorf("json-ld: %w", err)
			}
			sawProduct = sawProduct || ok
		case n.Data == "table" && hasClass(n, "parameters-table"):
			p.readTable(n)
			sawTable = true
		}
	}
	if !sawProduct {
		return nil, errors.New("no JSON-LD Product block")
	}
	if !sawTable {
		return nil, errors.New("no parameters-table")
	}
	return p, nil
}

type ldBlock struct {
	Type   string                `json:"@type"`
	Name   string                `json:"name"`
	SKU    string                `json:"sku"`
	Image  []string              `json:"image"`
	Brand  struct{ Name string } `json:"brand"`
	Offers struct {
		Price string `json:"price"`
	} `json:"offers"`
	Items []struct {
		Position int    `json:"position"`
		Name     string `json:"name"`
	} `json:"itemListElement"`
}

// readLD reports whether the block was the Product block.
func (p *page) readLD(src string) (bool, error) {
	var b ldBlock
	if err := json.Unmarshal([]byte(src), &b); err != nil {
		return false, err
	}
	switch b.Type {
	case "Product":
		p.SKU = strings.TrimSpace(b.SKU)
		p.Name = strings.TrimSpace(b.Name)
		p.Brand = strings.TrimSpace(b.Brand.Name)
		p.Images = b.Image
		// The JSON-LD says priceCurrency "BGN", but the page renders the same number through
		// <price-eur> and Bulgaria uses the euro since 2026-01-01, so the value is EUR.
		cents, err := parseCents(b.Offers.Price)
		if err != nil {
			return false, fmt.Errorf("price %q: %w", b.Offers.Price, err)
		}
		p.PriceCents = cents
		return true, nil
	case "BreadcrumbList":
		// Drop "Начало" (home) at the front and the product itself at the end.
		for i, it := range b.Items {
			if i == 0 || i == len(b.Items)-1 {
				continue
			}
			p.CategoryPath = append(p.CategoryPath, strings.TrimSpace(it.Name))
		}
	}
	return false, nil
}

// readTable reads <tr><td>key</td><td>value</td></tr> rows. A value made of several <div>s
// (e.g. "USB портове") is joined with ", ", matching the raw format in AGENT_PLAN.md §4.1.
func (p *page) readTable(t *html.Node) {
	for tr := range t.Descendants() {
		if tr.Type != html.ElementNode || tr.Data != "tr" {
			continue
		}
		var cells []*html.Node
		for c := range tr.ChildNodes() {
			if c.Type == html.ElementNode && c.Data == "td" {
				cells = append(cells, c)
			}
		}
		if len(cells) != 2 {
			continue
		}
		key := strings.TrimSpace(text(cells[0]))
		if key == "" {
			continue
		}
		var parts []string
		for d := range cells[1].ChildNodes() {
			if d.Type == html.ElementNode && d.Data == "div" {
				if s := strings.TrimSpace(text(d)); s != "" {
					parts = append(parts, s)
				}
			}
		}
		if parts == nil {
			if s := strings.TrimSpace(text(cells[1])); s != "" {
				parts = []string{s}
			}
		}
		p.Rows[key] = strings.Join(parts, ", ")
	}
}

// parseCents turns "62.33", "1234.5" or "1234" into cents without going through float64.
func parseCents(s string) (int64, error) {
	whole, frac, _ := strings.Cut(strings.TrimSpace(s), ".")
	if len(frac) > 2 {
		return 0, errors.New("more than 2 decimals")
	}
	frac += strings.Repeat("0", 2-len(frac))
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, err
	}
	return w*100 + f, nil
}

func text(n *html.Node) string {
	var sb strings.Builder
	for d := range n.Descendants() {
		if d.Type == html.TextNode {
			sb.WriteString(d.Data)
		}
	}
	return sb.String()
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	return strings.Contains(" "+attr(n, "class")+" ", " "+class+" ")
}
