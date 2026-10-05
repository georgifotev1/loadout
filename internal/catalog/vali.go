package catalog

import (
	"strconv"
	"strings"

	"github.com/gfotev/loadout/internal/vali"
)

// valiCategories maps the second level of Vali's category tree to our canonical Categories. The
// third level is just "<second level> <brand>", so it adds nothing.
// ASSUMPTION: these are the names vali.bg shows today; the dealer API may use ids instead.
var valiCategories = map[string]string{
	"Процесори":                       "cpu",
	"Дънни платки":                    "motherboard",
	"Памети":                          "ram",
	"Видео карти":                     "gpu",
	"Захранвания":                     "psu",
	"Кутии за компютри":               "case",
	"Охладители за процесори":         "cooler",
	"Solid State Drive (SSD) дискове": "storage",
	"Геймърски столове":               "peripheral.chair",
	"Геймърски слушалки":              "peripheral.headset",
	"Геймърски клавиатури":            "peripheral.keyboard",
	"Монитори и публични дисплеи":     "peripheral.monitor",
	"Геймърски мишки":                 "peripheral.mouse",
	"Геймърски падове":                "peripheral.mousepad",
}

// valiCategory returns the canonical category for a Vali category path. ok is false for
// categories we don't sell (yet); the sync skips those products and counts them as errors, so a
// new Vali category shows up in the sync run instead of being silently dropped.
func valiCategory(raw vali.RawProduct) (category string, ok bool) {
	if len(raw.CategoryPath) < 2 {
		return "", false
	}
	category, ok = valiCategories[raw.CategoryPath[1]]
	return category, ok
}

// valiWarrantyMonths parses Vali's warranty text: "36 месеца" → 36. Anything else ("Доживотна",
// i.e. lifetime, or an empty value) is nil, and the text is kept as-is for display.
func valiWarrantyMonths(text string) *int {
	num, unit, ok := strings.Cut(strings.TrimSpace(text), " ")
	if !ok {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "месеца", "месец":
	default:
		return nil
	}
	n, err := strconv.Atoi(num)
	if err != nil || n <= 0 {
		return nil
	}
	return &n
}
