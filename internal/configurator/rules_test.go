package configurator

import (
	"reflect"
	"testing"

	"github.com/gfotev/loadout/internal/specs"
)

// valid returns a complete build with no findings: a Ryzen 5 7600 on a B650 ATX board. Each call
// returns fresh values, so a test case can change it freely.
func valid() Build {
	return Build{
		CPU: &specs.CPU{
			Socket:         new("AM5"),
			TDPW:           new(65),
			IGPU:           new(true),
			CoolerIncluded: new(true),
			MemoryTypes:    []string{"DDR5"},
			Generation:     new("ryzen-7000"),
		},
		Motherboard: &specs.Motherboard{
			Socket:                  new("AM5"),
			Chipset:                 new("B650"),
			FormFactor:              new("ATX"),
			MemoryType:              new("DDR5"),
			MemorySlots:             new(4),
			MaxMemoryGB:             new(192),
			MaxMemoryMHz:            new(6400),
			M2Slots:                 new(2),
			SupportedCPUGenerations: []string{"ryzen-7000", "ryzen-9000"},
		},
		RAM: []specs.RAM{
			{Type: new("DDR5"), CapacityGBTotal: new(32), Modules: new(2), SpeedMHz: new(6000)},
		},
		GPU: &specs.GPU{
			LengthMM:        new(300),
			TDPW:            new(250),
			PowerConnectors: []string{"12V-2x6"},
		},
		Storage: []specs.Storage{
			{Interface: new("nvme"), FormFactor: new("m2_2280"), CapacityGB: new(1000)},
		},
		PSU: &specs.PSU{
			WattageW:        new(750),
			FormFactor:      new("ATX"),
			PCIe12V2x6Count: new(1),
		},
		Case: &specs.Case{
			FormFactorsSupported: []string{"ATX", "mATX", "ITX"},
			MaxGPULengthMM:       new(400),
			MaxCoolerHeightMM:    new(170),
			PSUFormFactor:        new("ATX"),
			RadiatorSupport:      []string{"top:240", "top:360", "front:280", "front:360"},
		},
		Cooler: &specs.Cooler{
			Type:       new("air"),
			Sockets:    []string{"AM4", "AM5", "LGA1700"},
			HeightMM:   new(155),
			TDPRatingW: new(220),
		},
	}
}

// aio swaps the build's cooler for a 360 mm AIO.
func aio(b *Build) {
	b.Cooler = &specs.Cooler{Type: new("aio"), Sockets: []string{"AM5"}, RadiatorMM: new(360), TDPRatingW: new(300)}
}

type ruleCase struct {
	name string
	edit func(b *Build)
	want []Finding // nil: no findings
}

func runRule(t *testing.T, check func(Build) []Finding, cases []ruleCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := valid()
			if tc.edit != nil {
				tc.edit(&b)
			}
			if got := check(b); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func finding(rule string, sev Severity, params map[string]any, slots ...Slot) []Finding {
	return one(rule, sev, params, slots...)
}

func missing(check string, slot Slot, field string) []Finding {
	return unknown(check, need{slot, field, false})
}

func TestValidBuildHasNoFindings(t *testing.T) {
	res := Check(valid())
	if !res.OK() || len(res.Warnings) > 0 || len(res.Info) > 0 {
		t.Fatalf("errors %+v\nwarnings %+v\ninfo %+v", res.Errors, res.Warnings, res.Info)
	}
}

func TestCPUMBSocket(t *testing.T) {
	runRule(t, cpuMBSocket, []ruleCase{
		{name: "same socket"},
		{name: "AM4 CPU on an AM5 board", edit: func(b *Build) { b.CPU.Socket = new("AM4") },
			want: finding(RuleCPUMBSocket, SeverityError, map[string]any{"cpu_socket": "AM4", "motherboard_socket": "AM5"}, SlotCPU, SlotMotherboard)},
		{name: "no motherboard yet", edit: func(b *Build) { b.Motherboard = nil; b.CPU.Socket = new("AM4") }},
		{name: "board socket unknown", edit: func(b *Build) { b.Motherboard.Socket = nil },
			want: missing(RuleCPUMBSocket, SlotMotherboard, "socket")},
	})
}

func TestCPUMBGeneration(t *testing.T) {
	runRule(t, cpuMBGeneration, []ruleCase{
		{name: "generation listed"},
		{name: "generation not listed: BIOS update", edit: func(b *Build) { b.Motherboard.SupportedCPUGenerations = []string{"ryzen-9000"} },
			want: finding(RuleCPUMBGeneration, SeverityWarning, map[string]any{"generation": "ryzen-7000"}, SlotCPU, SlotMotherboard)},
		{name: "board lists no generations: skipped", edit: func(b *Build) { b.Motherboard.SupportedCPUGenerations = nil }},
		{name: "socket differs: left to CPU_MB_SOCKET", edit: func(b *Build) {
			b.CPU.Socket = new("AM4")
			b.CPU.Generation = new("ryzen-5000")
		}},
	})
}

func TestRAMMBType(t *testing.T) {
	runRule(t, ramMBType, []ruleCase{
		{name: "DDR5 on a DDR5 board"},
		{name: "DDR4 on a DDR5 board", edit: func(b *Build) { b.RAM[0].Type = new("DDR4") },
			want: finding(RuleRAMMBType, SeverityError, map[string]any{"ram_types": []string{"DDR4"}, "motherboard_type": "DDR5"}, SlotRAM, SlotMotherboard)},
		{name: "two wrong kits reported once", edit: func(b *Build) {
			b.RAM[0].Type = new("DDR4")
			b.RAM = append(b.RAM, b.RAM[0])
		}, want: finding(RuleRAMMBType, SeverityError, map[string]any{"ram_types": []string{"DDR4"}, "motherboard_type": "DDR5"}, SlotRAM, SlotMotherboard)},
		{name: "no RAM yet", edit: func(b *Build) { b.RAM = nil }},
		{name: "kit type unknown", edit: func(b *Build) { b.RAM[0].Type = nil },
			want: missing(RuleRAMMBType, SlotRAM, "type")},
	})
}

func TestRAMSlots(t *testing.T) {
	runRule(t, ramSlots, []ruleCase{
		{name: "2 sticks in 4 slots"},
		{name: "two 2-stick kits fill 4 slots", edit: func(b *Build) { b.RAM = append(b.RAM, b.RAM[0]) }},
		{name: "two 2-stick kits on a 2-slot ITX board", edit: func(b *Build) {
			b.RAM = append(b.RAM, b.RAM[0])
			b.Motherboard.MemorySlots = new(2)
		}, want: finding(RuleRAMSlots, SeverityError, map[string]any{"modules": 4, "slots": 2}, SlotRAM, SlotMotherboard)},
		{name: "slot count unknown", edit: func(b *Build) { b.Motherboard.MemorySlots = nil },
			want: missing(RuleRAMSlots, SlotMotherboard, "memory_slots")},
	})
}

func TestRAMCapacity(t *testing.T) {
	runRule(t, ramCapacity, []ruleCase{
		{name: "32 GB of 192"},
		{name: "exactly the maximum", edit: func(b *Build) { b.Motherboard.MaxMemoryGB = new(32) }},
		{name: "over the maximum", edit: func(b *Build) {
			b.RAM = append(b.RAM, b.RAM[0])
			b.Motherboard.MaxMemoryGB = new(32)
		}, want: finding(RuleRAMCapacity, SeverityError, map[string]any{"capacity_gb": 64, "max_gb": 32}, SlotRAM, SlotMotherboard)},
		{name: "board maximum unknown", edit: func(b *Build) { b.Motherboard.MaxMemoryGB = nil },
			want: missing(RuleRAMCapacity, SlotMotherboard, "max_memory_gb")},
	})
}

func TestRAMSpeed(t *testing.T) {
	runRule(t, ramSpeed, []ruleCase{
		{name: "6000 on a 6400 board"},
		{name: "8000 on a 6400 board", edit: func(b *Build) { b.RAM[0].SpeedMHz = new(8000) },
			want: finding(RuleRAMSpeed, SeverityInfo, map[string]any{"ram_mhz": 8000, "max_mhz": 6400}, SlotRAM, SlotMotherboard)},
		{name: "fastest kit counts", edit: func(b *Build) { b.RAM = append(b.RAM, specs.RAM{SpeedMHz: new(7200)}) },
			want: finding(RuleRAMSpeed, SeverityInfo, map[string]any{"ram_mhz": 7200, "max_mhz": 6400}, SlotRAM, SlotMotherboard)},
		{name: "board maximum unknown: skipped", edit: func(b *Build) {
			b.RAM[0].SpeedMHz = new(8000)
			b.Motherboard.MaxMemoryMHz = nil
		}},
	})
}

func TestRAMDualChannel(t *testing.T) {
	runRule(t, ramDualChannel, []ruleCase{
		{name: "2-stick kit"},
		{name: "one stick", edit: func(b *Build) { b.RAM[0].Modules = new(1) },
			want: finding(RuleRAMDualChannel, SeverityWarning, nil, SlotRAM)},
		{name: "two single sticks", edit: func(b *Build) {
			b.RAM[0].Modules = new(1)
			b.RAM = append(b.RAM, b.RAM[0])
		}},
		{name: "no RAM yet", edit: func(b *Build) { b.RAM = nil }},
	})
}

func TestCaseMBForm(t *testing.T) {
	runRule(t, caseMBForm, []ruleCase{
		{name: "ATX board in an ATX case"},
		{name: "ATX board in an ITX case", edit: func(b *Build) { b.Case.FormFactorsSupported = []string{"ITX"} },
			want: finding(RuleCaseMBForm, SeverityError, map[string]any{"form_factor": "ATX", "supported": []string{"ITX"}}, SlotCase, SlotMotherboard)},
		{name: "mATX board in an ATX case", edit: func(b *Build) { b.Motherboard.FormFactor = new("mATX") }},
		{name: "case form factors unknown", edit: func(b *Build) { b.Case.FormFactorsSupported = nil },
			want: missing(RuleCaseMBForm, SlotCase, "form_factors_supported")},
	})
}

func TestCaseGPULength(t *testing.T) {
	params := func(gpu, maxLen int) map[string]any { return map[string]any{"gpu_mm": gpu, "max_mm": maxLen} }
	runRule(t, caseGPULength, []ruleCase{
		{name: "100 mm to spare"},
		{name: "exactly 10 mm to spare", edit: func(b *Build) { b.GPU.LengthMM = new(390) }},
		{name: "9 mm to spare", edit: func(b *Build) { b.GPU.LengthMM = new(391) },
			want: finding(RuleCaseGPULength, SeverityWarning, params(391, 400), SlotCase, SlotGPU)},
		{name: "exactly the maximum", edit: func(b *Build) { b.GPU.LengthMM = new(400) },
			want: finding(RuleCaseGPULength, SeverityWarning, params(400, 400), SlotCase, SlotGPU)},
		{name: "too long", edit: func(b *Build) { b.GPU.LengthMM = new(420) },
			want: finding(RuleCaseGPULength, SeverityError, params(420, 400), SlotCase, SlotGPU)},
		{name: "GPU length unknown", edit: func(b *Build) { b.GPU.LengthMM = nil },
			want: missing(RuleCaseGPULength, SlotGPU, "length_mm")},
	})
}

func TestCaseCoolerHeight(t *testing.T) {
	runRule(t, caseCoolerHeight, []ruleCase{
		{name: "155 mm in a 170 mm case"},
		{name: "exactly the maximum", edit: func(b *Build) { b.Cooler.HeightMM = new(170) }},
		{name: "too tall", edit: func(b *Build) { b.Cooler.HeightMM = new(175) },
			want: finding(RuleCaseCoolerHeight, SeverityError, map[string]any{"cooler_mm": 175, "max_mm": 170}, SlotCase, SlotCooler)},
		{name: "AIO: no height check", edit: func(b *Build) {
			aio(b)
			b.Case.MaxCoolerHeightMM = new(50)
		}},
		{name: "air cooler height unknown", edit: func(b *Build) { b.Cooler.HeightMM = nil },
			want: missing(RuleCaseCoolerHeight, SlotCooler, "height_mm")},
	})
}

func TestCaseRadiator(t *testing.T) {
	runRule(t, caseRadiator, []ruleCase{
		{name: "air cooler: no radiator check"},
		{name: "360 AIO, case takes 360", edit: aio},
		{name: "360 AIO, case takes 240 only", edit: func(b *Build) {
			aio(b)
			b.Case.RadiatorSupport = []string{"top:240", "front:240"}
		}, want: finding(RuleCaseRadiator, SeverityError, map[string]any{"radiator_mm": 360, "supported": []string{"top:240", "front:240"}}, SlotCase, SlotCooler)},
		{name: "case takes no radiators", edit: func(b *Build) {
			aio(b)
			b.Case.RadiatorSupport = []string{}
		}, want: finding(RuleCaseRadiator, SeverityError, map[string]any{"radiator_mm": 360, "supported": []string{}}, SlotCase, SlotCooler)},
		{name: "case radiator support unknown", edit: func(b *Build) {
			aio(b)
			b.Case.RadiatorSupport = nil
		}, want: missing(RuleCaseRadiator, SlotCase, "radiator_support")},
	})
}

func TestCasePSUForm(t *testing.T) {
	runRule(t, casePSUForm, []ruleCase{
		{name: "ATX in an ATX case"},
		{name: "SFX in an ATX case", edit: func(b *Build) { b.PSU.FormFactor = new("SFX") }},
		{name: "ATX in an SFX case", edit: func(b *Build) { b.Case.PSUFormFactor = new("SFX") },
			want: finding(RuleCasePSUForm, SeverityError, map[string]any{"psu_form_factor": "ATX", "case_form_factor": "SFX"}, SlotCase, SlotPSU)},
		{name: "SFX-L in an SFX case", edit: func(b *Build) {
			b.PSU.FormFactor = new("SFX-L")
			b.Case.PSUFormFactor = new("SFX")
		}, want: finding(RuleCasePSUForm, SeverityError, map[string]any{"psu_form_factor": "SFX-L", "case_form_factor": "SFX"}, SlotCase, SlotPSU)},
		{name: "unrecognised form factor", edit: func(b *Build) { b.PSU.FormFactor = new("TFX") },
			want: missing(RuleCasePSUForm, SlotPSU, "form_factor")},
	})
}

func TestCoolerSocket(t *testing.T) {
	runRule(t, coolerSocket, []ruleCase{
		{name: "AM5 listed"},
		{name: "AM5 not listed", edit: func(b *Build) { b.Cooler.Sockets = []string{"LGA1700", "LGA1851"} },
			want: finding(RuleCoolerSocket, SeverityError, map[string]any{"cpu_socket": "AM5", "sockets": []string{"LGA1700", "LGA1851"}}, SlotCooler, SlotCPU)},
		{name: "no CPU yet", edit: func(b *Build) { b.CPU = nil }},
		{name: "cooler sockets unknown", edit: func(b *Build) { b.Cooler.Sockets = nil },
			want: missing(RuleCoolerSocket, SlotCooler, "sockets")},
	})
}

func TestCoolerRequired(t *testing.T) {
	runRule(t, coolerRequired, []ruleCase{
		{name: "cooler picked"},
		{name: "boxed CPU with a cooler, none picked", edit: func(b *Build) { b.Cooler = nil }},
		{name: "tray CPU, none picked", edit: func(b *Build) {
			b.Cooler = nil
			b.CPU.CoolerIncluded = new(false)
		}, want: finding(RuleCoolerRequired, SeverityError, nil, SlotCooler, SlotCPU)},
		{name: "unknown whether a cooler is included", edit: func(b *Build) {
			b.Cooler = nil
			b.CPU.CoolerIncluded = nil
		}, want: missing(RuleCoolerRequired, SlotCPU, "cooler_included")},
	})
}

func TestCoolerTDP(t *testing.T) {
	runRule(t, coolerTDP, []ruleCase{
		{name: "220 W cooler on a 65 W CPU"},
		{name: "exactly the CPU's TDP", edit: func(b *Build) { b.Cooler.TDPRatingW = new(65) }},
		{name: "under the CPU's TDP", edit: func(b *Build) {
			b.CPU.TDPW = new(125)
			b.Cooler.TDPRatingW = new(95)
		}, want: finding(RuleCoolerTDP, SeverityWarning, map[string]any{"cooler_w": 95, "cpu_w": 125}, SlotCooler, SlotCPU)},
		{name: "cooler rating unknown: skipped", edit: func(b *Build) { b.Cooler.TDPRatingW = nil }},
	})
}

func TestGPURequired(t *testing.T) {
	ryzen3_4100 := func(b *Build) {
		b.CPU = &specs.CPU{Socket: new("AM4"), TDPW: new(65), IGPU: new(false), CoolerIncluded: new(true)}
	}
	runRule(t, gpuRequired, []ruleCase{
		{name: "GPU picked"},
		{name: "CPU with iGPU, no GPU", edit: func(b *Build) { b.GPU = nil }},
		{name: "Ryzen 3 4100 (no iGPU), no GPU", edit: func(b *Build) {
			ryzen3_4100(b)
			b.GPU = nil
		}, want: finding(RuleGPURequired, SeverityError, nil, SlotGPU, SlotCPU)},
		{name: "Ryzen 3 4100 with a GPU", edit: ryzen3_4100},
		{name: "iGPU unknown, no GPU", edit: func(b *Build) {
			b.CPU.IGPU = nil
			b.GPU = nil
		}, want: missing(RuleGPURequired, SlotCPU, "igpu")},
	})
}

func TestPSUWattage(t *testing.T) {
	// The valid build draws 408 W: 98 (65 W CPU × 1.5) + 250 + 50 + 10, recommended 550 W.
	params := func(psu int) map[string]any {
		return map[string]any{"estimate_w": 408, "psu_w": psu, "recommended_w": 550}
	}
	runRule(t, psuWattage, []ruleCase{
		{name: "750 W for 408 W"},
		{name: "exactly 80% load", edit: func(b *Build) { b.PSU.WattageW = new(510) }},
		{name: "over 80% load", edit: func(b *Build) { b.PSU.WattageW = new(500) },
			want: finding(RulePSUWattage, SeverityWarning, params(500), SlotPSU)},
		{name: "over the PSU's wattage", edit: func(b *Build) { b.PSU.WattageW = new(400) },
			want: finding(RulePSUWattage, SeverityError, params(400), SlotPSU)},
		{name: "GPU TDP unknown", edit: func(b *Build) { b.GPU.TDPW = nil },
			want: missing(RulePSUWattage, SlotGPU, "tdp_w")},
		{name: "no PSU yet", edit: func(b *Build) { b.PSU = nil }},
	})
}

func TestPSUConnectors(t *testing.T) {
	runRule(t, psuConnectors, []ruleCase{
		{name: "PSU has a 12V-2x6"},
		{name: "PSU has none", edit: func(b *Build) { b.PSU.PCIe12V2x6Count = new(0) },
			want: finding(RulePSUConnectors, SeverityWarning, map[string]any{"connector": "12V-2x6"}, SlotPSU, SlotGPU)},
		{name: "GPU uses 8-pin plugs", edit: func(b *Build) {
			b.GPU.PowerConnectors = []string{"8-pin", "8-pin"}
			b.PSU.PCIe12V2x6Count = new(0)
		}},
		{name: "PSU connectors unknown: skipped", edit: func(b *Build) { b.PSU.PCIe12V2x6Count = nil }},
	})
}

func TestStorageM2Slots(t *testing.T) {
	sata := specs.Storage{Interface: new("sata"), FormFactor: new("2.5"), CapacityGB: new(2000)}
	m2sata := specs.Storage{Interface: new("sata"), FormFactor: new("m2_2280"), CapacityGB: new(1000)}
	runRule(t, storageM2Slots, []ruleCase{
		{name: "1 NVMe, 2 slots"},
		{name: "2 NVMe, 2 slots", edit: func(b *Build) { b.Storage = append(b.Storage, b.Storage[0]) }},
		{name: "3 NVMe, 2 slots", edit: func(b *Build) { b.Storage = append(b.Storage, b.Storage[0], b.Storage[0]) },
			want: finding(RuleStorageM2Slots, SeverityError, map[string]any{"drives": 3, "slots": 2}, SlotStorage, SlotMotherboard)},
		{name: "M.2 SATA takes a slot too", edit: func(b *Build) { b.Storage = append(b.Storage, m2sata, m2sata) },
			want: finding(RuleStorageM2Slots, SeverityError, map[string]any{"drives": 3, "slots": 2}, SlotStorage, SlotMotherboard)},
		{name: "2.5\" SATA doesn't", edit: func(b *Build) { b.Storage = append(b.Storage, b.Storage[0], sata, sata) }},
	})
}

func TestPartRequired(t *testing.T) {
	required := func(slot Slot) Finding {
		return Finding{Rule: RulePartRequired, Severity: SeverityError, Slots: []Slot{slot}}
	}
	runRule(t, partRequired, []ruleCase{
		{name: "complete build"},
		{name: "no storage", edit: func(b *Build) { b.Storage = nil },
			want: []Finding{required(SlotStorage)}},
		{name: "GPU and cooler are optional", edit: func(b *Build) {
			b.GPU = nil
			b.Cooler = nil
		}},
		{name: "empty build", edit: func(b *Build) { *b = Build{} }, want: []Finding{
			required(SlotCPU),
			required(SlotMotherboard),
			required(SlotRAM),
			required(SlotStorage),
			required(SlotPSU),
			required(SlotCase),
		}},
	})
}

func TestCheckReportsMissingSpecOnce(t *testing.T) {
	b := valid()
	b.CPU.Socket = nil // needed by CPU_MB_SOCKET and COOLER_SOCKET
	res := Check(b)
	if !res.OK() {
		t.Fatalf("errors: %+v", res.Errors)
	}
	want := missing(RuleCPUMBSocket, SlotCPU, "socket")
	if !reflect.DeepEqual(res.Warnings, want) {
		t.Errorf("warnings: got %+v, want %+v", res.Warnings, want)
	}
}

func TestCheckSortsBySeverity(t *testing.T) {
	b := valid()
	b.CPU.Socket = new("AM4")          // error
	b.RAM[0].Modules = new(1)          // warning
	b.RAM[0].SpeedMHz = new(8000)      // info
	b.Cooler.Sockets = []string{"AM5"} // a second error
	res := Check(b)
	rules := func(fs []Finding) []string {
		var ids []string
		for _, f := range fs {
			ids = append(ids, f.Rule)
		}
		return ids
	}
	if got, want := rules(res.Errors), []string{RuleCPUMBSocket, RuleCoolerSocket}; !reflect.DeepEqual(got, want) {
		t.Errorf("errors: got %v, want %v", got, want)
	}
	if got, want := rules(res.Warnings), []string{RuleRAMDualChannel}; !reflect.DeepEqual(got, want) {
		t.Errorf("warnings: got %v, want %v", got, want)
	}
	if got, want := rules(res.Info), []string{RuleRAMSpeed}; !reflect.DeepEqual(got, want) {
		t.Errorf("info: got %v, want %v", got, want)
	}
	if res.EstimatedPowerW != 408 || res.RecommendedPSUW != 550 {
		t.Errorf("power: got %d W, recommended %d W", res.EstimatedPowerW, res.RecommendedPSUW)
	}
}
