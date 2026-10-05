package money

import (
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		in   Cents
		want string
	}{
		{0, "0,00 €"},
		{5, "0,05 €"},
		{6990, "69,90 €"},
		{123456, "1 234,56 €"},
		{100000000, "1 000 000,00 €"},
		{-123456, "-1 234,56 €"},
	}
	for _, tt := range tests {
		// The test table uses plain spaces for readability; Format uses non-breaking ones.
		want := strings.ReplaceAll(tt.want, " ", nbsp)
		if got := Format(tt.in); got != want {
			t.Errorf("Format(%d) = %q, want %q", tt.in, got, want)
		}
	}
}
