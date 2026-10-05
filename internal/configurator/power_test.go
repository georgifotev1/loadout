package configurator

import (
	"testing"

	"github.com/gfotev/loadout/internal/specs"
)

func TestEstimatePower(t *testing.T) {
	tests := []struct {
		name string
		edit func(b *Build)
		want int
	}{
		{"valid build: 98 + 250 + 50 + 10", nil, 408},
		{"empty build", func(b *Build) { *b = Build{} }, 0},
		{"odd TDP rounds up: 105 × 1.5 = 157.5", func(b *Build) { b.CPU.TDPW = new(105) }, 158 + 250 + 50 + 10},
		{"no GPU", func(b *Build) { b.GPU = nil }, 98 + 50 + 10},
		{"three drives", func(b *Build) { b.Storage = append(b.Storage, specs.Storage{}, specs.Storage{}) }, 98 + 250 + 50 + 30},
		{"unknown TDPs count as 0", func(b *Build) { b.CPU.TDPW = nil; b.GPU.TDPW = nil }, 50 + 10},
		{"only a case and PSU", func(b *Build) { *b = Build{Case: b.Case, PSU: b.PSU} }, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := valid()
			if tt.edit != nil {
				tt.edit(&b)
			}
			if got := EstimatePower(b); got != tt.want {
				t.Errorf("got %d W, want %d W", got, tt.want)
			}
		})
	}
}

func TestRecommendedPSU(t *testing.T) {
	tests := []struct{ estimate, want int }{
		{0, 0},
		{408, 550},   // 530.4 → 550
		{500, 650},   // exactly 650
		{501, 700},   // 651.3 → 700
		{1000, 1300}, // exactly 1300
	}
	for _, tt := range tests {
		if got := RecommendedPSU(tt.estimate); got != tt.want {
			t.Errorf("RecommendedPSU(%d) = %d, want %d", tt.estimate, got, tt.want)
		}
	}
}
