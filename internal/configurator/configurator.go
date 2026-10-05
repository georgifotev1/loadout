// Package configurator is the PC builder's compatibility engine: it checks a build's parts
// against each other and estimates its power draw.
//
// The rules are pure functions over canonical specs (internal/specs), so they never touch the
// database and every rule has table-driven tests. A check that can't run because a value is
// unknown never passes silently: an error-level check reports MISSING_SPEC instead.
package configurator

import (
	"slices"

	"github.com/gfotev/loadout/internal/specs"
)

// Slot is a part of a build. The values are the catalog categories.
type Slot string

const (
	SlotCPU         Slot = "cpu"
	SlotMotherboard Slot = "motherboard"
	SlotRAM         Slot = "ram"
	SlotGPU         Slot = "gpu"
	SlotStorage     Slot = "storage"
	SlotPSU         Slot = "psu"
	SlotCase        Slot = "case"
	SlotCooler      Slot = "cooler"
)

// Slots lists the slots in the order the builder asks for them.
var Slots = []Slot{SlotCPU, SlotMotherboard, SlotRAM, SlotGPU, SlotStorage, SlotPSU, SlotCase, SlotCooler}

// Severity says what a finding means for the build.
type Severity string

const (
	SeverityError   Severity = "error"   // the parts don't work together; the build can't be ordered
	SeverityWarning Severity = "warning" // it works, but the shopper should know (BIOS update, tight fit)
	SeverityInfo    Severity = "info"    // worth a note, nothing to fix
)

// Build is the parts picked so far, with their canonical specs. A nil part isn't picked yet.
// RAM and Storage can hold several parts; a repeated part is bought twice.
type Build struct {
	CPU         *specs.CPU
	Motherboard *specs.Motherboard
	RAM         []specs.RAM
	GPU         *specs.GPU
	Storage     []specs.Storage
	PSU         *specs.PSU
	Case        *specs.Case
	Cooler      *specs.Cooler
}

// Finding is one problem or note about a build. The web app turns Rule and Params into text, so
// the engine stays language-neutral.
type Finding struct {
	Rule     string
	Severity Severity
	Slots    []Slot         // the parts involved, so the UI can mark them
	Params   map[string]any // the values the message needs, e.g. {"gpu_mm": 340, "max_mm": 320}
}

// Rule is one compatibility check. A rule returns nothing when its parts aren't picked yet.
// Severity is on each finding rather than the rule, because some rules can return either (a GPU
// that doesn't fit is an error, one that fits with under 10 mm to spare is a warning).
type Rule struct {
	ID    string
	Check func(Build) []Finding
}

// Result is the outcome of checking a build.
type Result struct {
	Errors          []Finding
	Warnings        []Finding
	Info            []Finding
	EstimatedPowerW int
	RecommendedPSUW int
}

// OK reports whether the build has no errors.
func (r Result) OK() bool {
	return len(r.Errors) == 0
}

// Check runs every rule over b. Repeated MISSING_SPEC findings (two rules needing the same
// unknown value) are reported once.
func Check(b Build) Result {
	est := EstimatePower(b)
	res := Result{
		Errors:          []Finding{},
		Warnings:        []Finding{},
		Info:            []Finding{},
		EstimatedPowerW: est,
		RecommendedPSUW: RecommendedPSU(est),
	}
	for _, r := range Rules {
		for _, f := range r.Check(b) {
			switch f.Severity {
			case SeverityError:
				res.Errors = append(res.Errors, f)
			case SeverityWarning:
				if f.Rule == MissingSpec && slices.ContainsFunc(res.Warnings, f.sameMissing) {
					continue
				}
				res.Warnings = append(res.Warnings, f)
			case SeverityInfo:
				res.Info = append(res.Info, f)
			}
		}
	}
	return res
}

func (f Finding) sameMissing(other Finding) bool {
	return other.Rule == MissingSpec && slices.Equal(f.Slots, other.Slots) && f.Params["field"] == other.Params["field"]
}
