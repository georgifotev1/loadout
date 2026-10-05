// Command vali-fixture turns the public vali.bg product pages listed in
// fixtures/vali/sources.txt into mock-vali fixtures.
//
// It is a one-off helper the developer runs by hand, not a scraping job: it fetches one page at
// a time with a delay and skips URLs that already have a fixture unless -force is set.
//
//	go run ./tools/vali-fixture
//	go run ./tools/vali-fixture -force   # re-capture everything
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// fixture is the mock-vali raw format. Field order here is the order in the JSON file.
type fixture struct {
	SKU              string            `json:"sku"`
	ManufacturerCode string            `json:"manufacturer_code,omitempty"`
	EAN              string            `json:"ean,omitempty"`
	Brand            string            `json:"brand"`
	Name             string            `json:"name"`
	CategoryPath     []string          `json:"category_path"`
	URL              string            `json:"url"`
	Warranty         string            `json:"warranty,omitempty"`
	Images           []string          `json:"images"`
	Specs            map[string]string `json:"specs"`
	DealerPriceCents int64             `json:"dealer_price_cents"`
	RRPCents         int64             `json:"rrp_cents"`
	Currency         string            `json:"currency"`
	Stock            map[string]int    `json:"stock"`
	UpdatedAt        string            `json:"updated_at"`
	Mock             string            `json:"_mock"`
}

// Table rows that become top-level fields instead of specs.
const (
	rowBrand    = "Производител"
	rowWarranty = "Гаранция"
	rowMPN      = "Продуктов код"
	rowEAN      = "Баркод"
)

var skuRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func main() {
	sources := flag.String("sources", "fixtures/vali/sources.txt", "list of `<category> <url>` lines")
	out := flag.String("out", "fixtures/vali", "fixture root directory")
	force := flag.Bool("force", false, "overwrite fixtures that already exist")
	delay := flag.Duration("delay", 1500*time.Millisecond, "pause between requests")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, *sources, *out, *force, *delay); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type source struct{ category, url string }

func run(ctx context.Context, sourcesPath, out string, force bool, delay time.Duration) error {
	srcs, err := readSources(sourcesPath)
	if err != nil {
		return err
	}
	have, err := existingURLs(out)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var failed, fetched int
	for _, s := range srcs {
		if !force && have[s.url] != "" {
			continue
		}
		if fetched++; fetched > 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		path, err := capture(ctx, client, s, out)
		if err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", s.url, err)
			continue
		}
		if old := have[s.url]; old != "" && old != path {
			// The SKU changed since the last capture; drop the stale file.
			if err := os.Remove(old); err != nil {
				return err
			}
		}
		fmt.Println("wrote", path)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d pages failed", failed, len(srcs))
	}
	return nil
}

func capture(ctx context.Context, client *http.Client, s source, out string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; loadout-fixtures/1.0; one-off capture)")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %s", resp.Status)
	}

	p, err := parsePage(resp.Body)
	if err != nil {
		return "", err
	}
	f, err := toFixture(p, s.url)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return "", err
	}
	path := filepath.Join(out, s.category, f.SKU+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, buf.Bytes(), 0o644)
}

func toFixture(p *page, srcURL string) (*fixture, error) {
	if !skuRe.MatchString(p.SKU) {
		return nil, fmt.Errorf("sku %q is not safe to use as a file name", p.SKU)
	}
	if p.PriceCents <= 0 {
		return nil, fmt.Errorf("no price")
	}
	if p.Brand == "" {
		p.Brand = p.Rows[rowBrand]
	}
	f := &fixture{
		SKU:              p.SKU,
		ManufacturerCode: p.Rows[rowMPN],
		EAN:              p.Rows[rowEAN],
		Brand:            p.Brand,
		Name:             p.Name,
		CategoryPath:     p.CategoryPath,
		URL:              srcURL, // the approved URL, so re-runs can match it to this file
		Warranty:         p.Rows[rowWarranty],
		Images:           p.Images,
		Specs:            map[string]string{},
		RRPCents:         p.PriceCents,
		Currency:         "EUR",
		UpdatedAt:        "2026-09-30T08:00:00Z",
		Mock: "Copied from the public vali.bg page on " + time.Now().Format(time.DateOnly) +
			". rrp_cents is the public price. dealer_price_cents, stock and updated_at are invented.",
	}
	for k, v := range p.Rows {
		switch k {
		case rowBrand, rowWarranty, rowMPN, rowEAN:
		default:
			f.Specs[k] = v
		}
	}
	mockCommercial(f)
	return f, nil
}

// mockCommercial invents the dealer-only fields, derived from the SKU so that re-capturing a
// page gives the same numbers.
// ASSUMPTION: Vali's dealer API exposes a dealer price and per-warehouse stock; shape unknown.
func mockCommercial(f *fixture) {
	h := sha256.Sum256([]byte(f.SKU))
	pct := int64(78 + h[0]%11) // dealer pays 78–88% of the public price
	f.DealerPriceCents = f.RRPCents * pct / 100
	f.Stock = map[string]int{"Sofia-Central": 0, "Plovdiv": 0}
	if h[1]%10 != 0 { // ~1 in 10 products is out of stock everywhere
		f.Stock["Sofia-Central"] = int(h[2] % 25)
		f.Stock["Plovdiv"] = int(h[3] % 6)
	}
}

func readSources(path string) ([]source, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var srcs []source
	sc := bufio.NewScanner(file)
	for n := 1; sc.Scan(); n++ {
		line, _, _ := strings.Cut(sc.Text(), "#")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 || !strings.HasPrefix(fields[1], "https://www.vali.bg/") {
			return nil, fmt.Errorf("%s:%d: want `<category> <vali.bg url>`", path, n)
		}
		srcs = append(srcs, source{category: fields[0], url: fields[1]})
	}
	return srcs, sc.Err()
}

// existingURLs maps each captured page URL to its fixture file.
func existingURLs(root string) (map[string]string, error) {
	m := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" || strings.HasSuffix(path, ".schema.json") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var f struct{ URL string }
		if err := json.Unmarshal(b, &f); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		m[f.URL] = path
		return nil
	})
	return m, err
}
