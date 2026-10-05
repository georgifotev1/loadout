package configurator

import (
	"slices"
	"strconv"
	"strings"

	"github.com/gfotev/loadout/internal/specs"
)

// Rule IDs. The web app has a message for each.
const (
	RuleCPUMBSocket      = "CPU_MB_SOCKET"
	RuleCPUMBGeneration  = "CPU_MB_GENERATION"
	RuleRAMMBType        = "RAM_MB_TYPE"
	RuleRAMSlots         = "RAM_SLOTS"
	RuleRAMCapacity      = "RAM_CAPACITY"
	RuleRAMSpeed         = "RAM_SPEED"
	RuleRAMDualChannel   = "RAM_DUAL_CHANNEL"
	RuleCaseMBForm       = "CASE_MB_FORM"
	RuleCaseGPULength    = "CASE_GPU_LENGTH"
	RuleCaseCoolerHeight = "CASE_COOLER_HEIGHT"
	RuleCaseRadiator     = "CASE_RADIATOR"
	RuleCasePSUForm      = "CASE_PSU_FORM"
	RuleCoolerSocket     = "COOLER_SOCKET"
	RuleCoolerRequired   = "COOLER_REQUIRED"
	RuleCoolerTDP        = "COOLER_TDP"
	RuleGPURequired      = "GPU_REQUIRED"
	RulePSUWattage       = "PSU_WATTAGE"
	RulePSUConnectors    = "PSU_CONNECTORS"
	RuleStorageM2Slots   = "STORAGE_M2_SLOTS"
	RulePartRequired     = "PART_REQUIRED"

	// MissingSpec is reported instead of an error-level check that can't run because a value is
	// unknown, so an unknown never reads as "compatible". Params: field, and check (the rule).
	MissingSpec = "MISSING_SPEC"
)

// Rules is every compatibility rule, in the order findings are reported.
var Rules = []Rule{
	{RuleCPUMBSocket, cpuMBSocket},
	{RuleCPUMBGeneration, cpuMBGeneration},
	{RuleRAMMBType, ramMBType},
	{RuleRAMSlots, ramSlots},
	{RuleRAMCapacity, ramCapacity},
	{RuleRAMSpeed, ramSpeed},
	{RuleRAMDualChannel, ramDualChannel},
	{RuleCaseMBForm, caseMBForm},
	{RuleCaseGPULength, caseGPULength},
	{RuleCaseCoolerHeight, caseCoolerHeight},
	{RuleCaseRadiator, caseRadiator},
	{RuleCasePSUForm, casePSUForm},
	{RuleCoolerSocket, coolerSocket},
	{RuleCoolerRequired, coolerRequired},
	{RuleCoolerTDP, coolerTDP},
	{RuleGPURequired, gpuRequired},
	{RulePSUWattage, psuWattage},
	{RulePSUConnectors, psuConnectors},
	{RuleStorageM2Slots, storageM2Slots},
	{RulePartRequired, partRequired},
}

// gpuClearanceWarnMM is the spare length under which a GPU that fits is still a tight fit.
const gpuClearanceWarnMM = 10

// connector12V2x6 is the GPU power plug of recent high-end cards (formerly 12VHPWR).
const connector12V2x6 = "12V-2x6"

// psuFormRank orders PSU form factors by size: a case takes its psu_form_factor and anything
// smaller.
var psuFormRank = map[string]int{"SFX": 1, "SFX-L": 2, "ATX": 3}

func cpuMBSocket(b Build) []Finding {
	cpu, mb := b.CPU, b.Motherboard
	if cpu == nil || mb == nil {
		return nil
	}
	if fs := unknown(RuleCPUMBSocket,
		need{SlotCPU, "socket", cpu.Socket != nil},
		need{SlotMotherboard, "socket", mb.Socket != nil},
	); fs != nil {
		return fs
	}
	if *cpu.Socket != *mb.Socket {
		return one(RuleCPUMBSocket, SeverityError, map[string]any{
			"cpu_socket":         *cpu.Socket,
			"motherboard_socket": *mb.Socket,
		}, SlotCPU, SlotMotherboard)
	}
	return nil
}

// cpuMBGeneration warns when the board's socket fits but its BIOS may predate the CPU. Unknown
// generations are skipped: a board that lists none doesn't mean it supports none.
func cpuMBGeneration(b Build) []Finding {
	cpu, mb := b.CPU, b.Motherboard
	if cpu == nil || mb == nil || cpu.Generation == nil || mb.SupportedCPUGenerations == nil {
		return nil
	}
	if cpu.Socket == nil || mb.Socket == nil || *cpu.Socket != *mb.Socket {
		return nil // CPU_MB_SOCKET covers it
	}
	if !slices.Contains(mb.SupportedCPUGenerations, *cpu.Generation) {
		return one(RuleCPUMBGeneration, SeverityWarning, map[string]any{
			"generation": *cpu.Generation,
		}, SlotCPU, SlotMotherboard)
	}
	return nil
}

func ramMBType(b Build) []Finding {
	mb := b.Motherboard
	if mb == nil || len(b.RAM) == 0 {
		return nil
	}
	needs := []need{{SlotMotherboard, "memory_type", mb.MemoryType != nil}}
	for _, r := range b.RAM {
		needs = append(needs, need{SlotRAM, "type", r.Type != nil})
	}
	if fs := unknown(RuleRAMMBType, needs...); fs != nil {
		return fs
	}
	var wrong []string
	for _, r := range b.RAM {
		if *r.Type != *mb.MemoryType && !slices.Contains(wrong, *r.Type) {
			wrong = append(wrong, *r.Type)
		}
	}
	if wrong != nil {
		return one(RuleRAMMBType, SeverityError, map[string]any{
			"ram_types":        wrong,
			"motherboard_type": *mb.MemoryType,
		}, SlotRAM, SlotMotherboard)
	}
	return nil
}

func ramSlots(b Build) []Finding {
	mb := b.Motherboard
	if mb == nil || len(b.RAM) == 0 {
		return nil
	}
	needs := []need{{SlotMotherboard, "memory_slots", mb.MemorySlots != nil}}
	for _, r := range b.RAM {
		needs = append(needs, need{SlotRAM, "modules", r.Modules != nil})
	}
	if fs := unknown(RuleRAMSlots, needs...); fs != nil {
		return fs
	}
	if n := sumRAM(b.RAM, func(r specs.RAM) *int { return r.Modules }); n > *mb.MemorySlots {
		return one(RuleRAMSlots, SeverityError, map[string]any{
			"modules": n,
			"slots":   *mb.MemorySlots,
		}, SlotRAM, SlotMotherboard)
	}
	return nil
}

func ramCapacity(b Build) []Finding {
	mb := b.Motherboard
	if mb == nil || len(b.RAM) == 0 {
		return nil
	}
	needs := []need{{SlotMotherboard, "max_memory_gb", mb.MaxMemoryGB != nil}}
	for _, r := range b.RAM {
		needs = append(needs, need{SlotRAM, "capacity_gb_total", r.CapacityGBTotal != nil})
	}
	if fs := unknown(RuleRAMCapacity, needs...); fs != nil {
		return fs
	}
	if gb := sumRAM(b.RAM, func(r specs.RAM) *int { return r.CapacityGBTotal }); gb > *mb.MaxMemoryGB {
		return one(RuleRAMCapacity, SeverityError, map[string]any{
			"capacity_gb": gb,
			"max_gb":      *mb.MaxMemoryGB,
		}, SlotRAM, SlotMotherboard)
	}
	return nil
}

// ramSpeed notes RAM rated above what the board supports: it works, at the board's speed.
func ramSpeed(b Build) []Finding {
	mb := b.Motherboard
	if mb == nil || mb.MaxMemoryMHz == nil {
		return nil
	}
	fastest := 0
	for _, r := range b.RAM {
		if r.SpeedMHz != nil {
			fastest = max(fastest, *r.SpeedMHz)
		}
	}
	if fastest > *mb.MaxMemoryMHz {
		return one(RuleRAMSpeed, SeverityInfo, map[string]any{
			"ram_mhz": fastest,
			"max_mhz": *mb.MaxMemoryMHz,
		}, SlotRAM, SlotMotherboard)
	}
	return nil
}

// ramDualChannel warns about a single stick, which halves memory bandwidth.
func ramDualChannel(b Build) []Finding {
	if len(b.RAM) == 0 || slices.ContainsFunc(b.RAM, func(r specs.RAM) bool { return r.Modules == nil }) {
		return nil
	}
	if sumRAM(b.RAM, func(r specs.RAM) *int { return r.Modules }) == 1 {
		return one(RuleRAMDualChannel, SeverityWarning, nil, SlotRAM)
	}
	return nil
}

func caseMBForm(b Build) []Finding {
	c, mb := b.Case, b.Motherboard
	if c == nil || mb == nil {
		return nil
	}
	if fs := unknown(RuleCaseMBForm,
		need{SlotMotherboard, "form_factor", mb.FormFactor != nil},
		need{SlotCase, "form_factors_supported", c.FormFactorsSupported != nil},
	); fs != nil {
		return fs
	}
	if !slices.Contains(c.FormFactorsSupported, *mb.FormFactor) {
		return one(RuleCaseMBForm, SeverityError, map[string]any{
			"form_factor": *mb.FormFactor,
			"supported":   c.FormFactorsSupported,
		}, SlotCase, SlotMotherboard)
	}
	return nil
}

func caseGPULength(b Build) []Finding {
	c, gpu := b.Case, b.GPU
	if c == nil || gpu == nil {
		return nil
	}
	if fs := unknown(RuleCaseGPULength,
		need{SlotGPU, "length_mm", gpu.LengthMM != nil},
		need{SlotCase, "max_gpu_length_mm", c.MaxGPULengthMM != nil},
	); fs != nil {
		return fs
	}
	params := map[string]any{"gpu_mm": *gpu.LengthMM, "max_mm": *c.MaxGPULengthMM}
	switch spare := *c.MaxGPULengthMM - *gpu.LengthMM; {
	case spare < 0:
		return one(RuleCaseGPULength, SeverityError, params, SlotCase, SlotGPU)
	case spare < gpuClearanceWarnMM:
		return one(RuleCaseGPULength, SeverityWarning, params, SlotCase, SlotGPU)
	}
	return nil
}

func caseCoolerHeight(b Build) []Finding {
	c, cooler := b.Case, b.Cooler
	if c == nil || cooler == nil {
		return nil
	}
	if fs := unknown(RuleCaseCoolerHeight, need{SlotCooler, "type", cooler.Type != nil}); fs != nil {
		return fs
	}
	if *cooler.Type != specs.CoolerAir {
		return nil
	}
	if fs := unknown(RuleCaseCoolerHeight,
		need{SlotCooler, "height_mm", cooler.HeightMM != nil},
		need{SlotCase, "max_cooler_height_mm", c.MaxCoolerHeightMM != nil},
	); fs != nil {
		return fs
	}
	if *cooler.HeightMM > *c.MaxCoolerHeightMM {
		return one(RuleCaseCoolerHeight, SeverityError, map[string]any{
			"cooler_mm": *cooler.HeightMM,
			"max_mm":    *c.MaxCoolerHeightMM,
		}, SlotCase, SlotCooler)
	}
	return nil
}

// caseRadiator checks an AIO's radiator against the case's mounts. A mount is listed per size it
// takes ("top:360", "top:240"), so the match is exact.
func caseRadiator(b Build) []Finding {
	c, cooler := b.Case, b.Cooler
	if c == nil || cooler == nil {
		return nil
	}
	if fs := unknown(RuleCaseRadiator, need{SlotCooler, "type", cooler.Type != nil}); fs != nil {
		return fs
	}
	if *cooler.Type != specs.CoolerAIO {
		return nil
	}
	if fs := unknown(RuleCaseRadiator,
		need{SlotCooler, "radiator_mm", cooler.RadiatorMM != nil},
		need{SlotCase, "radiator_support", c.RadiatorSupport != nil},
	); fs != nil {
		return fs
	}
	for _, mount := range c.RadiatorSupport {
		_, size, _ := strings.Cut(mount, ":")
		if mm, err := strconv.Atoi(size); err == nil && mm == *cooler.RadiatorMM {
			return nil
		}
	}
	return one(RuleCaseRadiator, SeverityError, map[string]any{
		"radiator_mm": *cooler.RadiatorMM,
		"supported":   c.RadiatorSupport,
	}, SlotCase, SlotCooler)
}

// casePSUForm checks the PSU isn't bigger than the case takes. A form factor we don't know the
// size of can't be compared, so it's reported as missing.
func casePSUForm(b Build) []Finding {
	c, psu := b.Case, b.PSU
	if c == nil || psu == nil {
		return nil
	}
	psuRank, psuOK := 0, psu.FormFactor != nil
	if psuOK {
		psuRank, psuOK = psuFormRank[*psu.FormFactor]
	}
	caseRank, caseOK := 0, c.PSUFormFactor != nil
	if caseOK {
		caseRank, caseOK = psuFormRank[*c.PSUFormFactor]
	}
	if fs := unknown(RuleCasePSUForm,
		need{SlotPSU, "form_factor", psuOK},
		need{SlotCase, "psu_form_factor", caseOK},
	); fs != nil {
		return fs
	}
	if psuRank > caseRank {
		return one(RuleCasePSUForm, SeverityError, map[string]any{
			"psu_form_factor":  *psu.FormFactor,
			"case_form_factor": *c.PSUFormFactor,
		}, SlotCase, SlotPSU)
	}
	return nil
}

func coolerSocket(b Build) []Finding {
	cpu, cooler := b.CPU, b.Cooler
	if cpu == nil || cooler == nil {
		return nil
	}
	if fs := unknown(RuleCoolerSocket,
		need{SlotCPU, "socket", cpu.Socket != nil},
		need{SlotCooler, "sockets", cooler.Sockets != nil},
	); fs != nil {
		return fs
	}
	if !slices.Contains(cooler.Sockets, *cpu.Socket) {
		return one(RuleCoolerSocket, SeverityError, map[string]any{
			"cpu_socket": *cpu.Socket,
			"sockets":    cooler.Sockets,
		}, SlotCooler, SlotCPU)
	}
	return nil
}

func coolerRequired(b Build) []Finding {
	cpu := b.CPU
	if cpu == nil || b.Cooler != nil {
		return nil
	}
	if fs := unknown(RuleCoolerRequired, need{SlotCPU, "cooler_included", cpu.CoolerIncluded != nil}); fs != nil {
		return fs
	}
	if !*cpu.CoolerIncluded {
		return one(RuleCoolerRequired, SeverityError, nil, SlotCooler, SlotCPU)
	}
	return nil
}

func coolerTDP(b Build) []Finding {
	cpu, cooler := b.CPU, b.Cooler
	if cpu == nil || cooler == nil || cpu.TDPW == nil || cooler.TDPRatingW == nil {
		return nil
	}
	if *cooler.TDPRatingW < *cpu.TDPW {
		return one(RuleCoolerTDP, SeverityWarning, map[string]any{
			"cooler_w": *cooler.TDPRatingW,
			"cpu_w":    *cpu.TDPW,
		}, SlotCooler, SlotCPU)
	}
	return nil
}

// gpuRequired catches CPUs without integrated graphics (Ryzen 3 4100, Core i5-12400F) in a
// build without a graphics card: it would have no display output.
func gpuRequired(b Build) []Finding {
	cpu := b.CPU
	if cpu == nil || b.GPU != nil {
		return nil
	}
	if fs := unknown(RuleGPURequired, need{SlotCPU, "igpu", cpu.IGPU != nil}); fs != nil {
		return fs
	}
	if !*cpu.IGPU {
		return one(RuleGPURequired, SeverityError, nil, SlotGPU, SlotCPU)
	}
	return nil
}

// psuWattage checks the estimate against the PSU. An unknown TDP would make the estimate too
// low, so it's reported rather than ignored.
func psuWattage(b Build) []Finding {
	psu := b.PSU
	est := EstimatePower(b)
	if psu == nil || est == 0 {
		return nil
	}
	needs := []need{{SlotPSU, "wattage_w", psu.WattageW != nil}}
	if b.CPU != nil {
		needs = append(needs, need{SlotCPU, "tdp_w", b.CPU.TDPW != nil})
	}
	if b.GPU != nil {
		needs = append(needs, need{SlotGPU, "tdp_w", b.GPU.TDPW != nil})
	}
	if fs := unknown(RulePSUWattage, needs...); fs != nil {
		return fs
	}
	params := map[string]any{
		"estimate_w":    est,
		"psu_w":         *psu.WattageW,
		"recommended_w": RecommendedPSU(est),
	}
	switch {
	case est > *psu.WattageW:
		return one(RulePSUWattage, SeverityError, params, SlotPSU)
	case est*100 > *psu.WattageW*psuLoadWarnPct:
		return one(RulePSUWattage, SeverityWarning, params, SlotPSU)
	}
	return nil
}

// psuConnectors warns when the GPU needs a 12V-2x6 plug the PSU doesn't have. Most cards ship
// an adapter, so it's a warning to check, not an error.
func psuConnectors(b Build) []Finding {
	gpu, psu := b.GPU, b.PSU
	if gpu == nil || psu == nil || psu.PCIe12V2x6Count == nil {
		return nil
	}
	if slices.Contains(gpu.PowerConnectors, connector12V2x6) && *psu.PCIe12V2x6Count == 0 {
		return one(RulePSUConnectors, SeverityWarning, map[string]any{"connector": connector12V2x6}, SlotPSU, SlotGPU)
	}
	return nil
}

// storageM2Slots counts M.2 drives, NVMe or SATA: both take an M.2 slot.
func storageM2Slots(b Build) []Finding {
	mb := b.Motherboard
	if mb == nil || len(b.Storage) == 0 {
		return nil
	}
	needs := []need{{SlotMotherboard, "m2_slots", mb.M2Slots != nil}}
	for _, s := range b.Storage {
		needs = append(needs, need{SlotStorage, "form_factor", s.FormFactor != nil})
	}
	if fs := unknown(RuleStorageM2Slots, needs...); fs != nil {
		return fs
	}
	n := 0
	for _, s := range b.Storage {
		if strings.HasPrefix(*s.FormFactor, "m2_") {
			n++
		}
	}
	if n > *mb.M2Slots {
		return one(RuleStorageM2Slots, SeverityError, map[string]any{
			"drives": n,
			"slots":  *mb.M2Slots,
		}, SlotStorage, SlotMotherboard)
	}
	return nil
}

// partRequired lists the parts every build needs. A GPU and a cooler are only needed sometimes:
// GPU_REQUIRED and COOLER_REQUIRED cover them.
func partRequired(b Build) []Finding {
	picked := []struct {
		slot Slot
		ok   bool
	}{
		{SlotCPU, b.CPU != nil},
		{SlotMotherboard, b.Motherboard != nil},
		{SlotRAM, len(b.RAM) > 0},
		{SlotStorage, len(b.Storage) > 0},
		{SlotPSU, b.PSU != nil},
		{SlotCase, b.Case != nil},
	}
	var fs []Finding
	for _, p := range picked {
		if !p.ok {
			fs = append(fs, Finding{Rule: RulePartRequired, Severity: SeverityError, Slots: []Slot{p.slot}})
		}
	}
	return fs
}

// need is a value a check needs: whether it's known, and where it comes from.
type need struct {
	slot  Slot
	field string
	known bool
}

// unknown returns a MISSING_SPEC warning for each value that isn't known, or nil if all are.
func unknown(check string, needs ...need) []Finding {
	var fs []Finding
	for _, n := range needs {
		if !n.known {
			fs = append(fs, Finding{
				Rule:     MissingSpec,
				Severity: SeverityWarning,
				Slots:    []Slot{n.slot},
				Params:   map[string]any{"field": n.field, "check": check},
			})
		}
	}
	return fs
}

func one(rule string, sev Severity, params map[string]any, slots ...Slot) []Finding {
	return []Finding{{Rule: rule, Severity: sev, Slots: slots, Params: params}}
}

// sumRAM adds up a field over the RAM kits. Callers check the field is known on every kit.
func sumRAM(kits []specs.RAM, field func(specs.RAM) *int) int {
	n := 0
	for _, r := range kits {
		n += *field(r)
	}
	return n
}
