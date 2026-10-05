// Package money holds the money type. Loadout sells in euros only: every amount is integer
// euro cents with VAT included, and there is no currency conversion anywhere.
package money

import (
	"strconv"
	"strings"
)

// Currency is the ISO 4217 code of the only currency Loadout uses.
const Currency = "EUR"

// VATStandardBP is Bulgaria's standard VAT rate in basis points (2000 = 20%). Price records
// store their own vat_bp, so a rate change is data, not a code change.
const VATStandardBP = 2000

// Cents is an amount in euro cents. Never use floats for money.
type Cents int64

// nbsp keeps "1 234,56 €" on one line wherever it's shown.
const nbsp = " "

// Format renders c the bg-BG way: "1 234,56 €" (non-breaking spaces). The web app formats the
// cents it receives on its own; this is for server-rendered text such as emails.
func Format(c Cents) string {
	var b strings.Builder
	n := int64(c)
	if n < 0 {
		b.WriteByte('-')
		n = -n
	}
	euros := strconv.FormatInt(n/100, 10)
	for i, d := range euros {
		if i > 0 && (len(euros)-i)%3 == 0 {
			b.WriteString(nbsp)
		}
		b.WriteRune(d)
	}
	cents := n % 100
	b.WriteByte(',')
	b.WriteByte(byte('0' + cents/10))
	b.WriteByte(byte('0' + cents%10))
	b.WriteString(nbsp + "€")
	return b.String()
}
