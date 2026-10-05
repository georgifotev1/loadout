package configurator

// Power estimate constants. The estimate is deliberately simple and errs high: a PSU picked
// from it has room for boost clocks, transient GPU spikes and ageing.
const (
	// cpuBoostPct scales the CPU's TDP: under boost, CPUs draw well above their rated TDP.
	cpuBoostPct = 150
	// basePowerW covers the motherboard, RAM and fans.
	basePowerW = 50
	// drivePowerW is per storage drive.
	drivePowerW = 10
	// psuHeadroomPct is how far above the estimate the recommended PSU is.
	psuHeadroomPct = 130
	// psuStepW is what the recommended PSU is rounded up to: PSUs are sold in 50 W steps.
	psuStepW = 50
	// psuLoadWarnPct is the share of a PSU's wattage above which the build is too close to its limit.
	psuLoadWarnPct = 80
)

// EstimatePower returns the build's estimated peak draw in watts: CPU TDP × 1.5 + GPU TDP +
// 50 W for the board, RAM and fans + 10 W per drive. Parts not picked yet, or with an unknown
// TDP, count as 0. An empty build is 0.
func EstimatePower(b Build) int {
	if b.CPU == nil && b.Motherboard == nil && len(b.RAM) == 0 && b.GPU == nil && len(b.Storage) == 0 {
		return 0
	}
	w := basePowerW + drivePowerW*len(b.Storage)
	if b.CPU != nil && b.CPU.TDPW != nil {
		w += ceilDiv(*b.CPU.TDPW*cpuBoostPct, 100)
	}
	if b.GPU != nil && b.GPU.TDPW != nil {
		w += *b.GPU.TDPW
	}
	return w
}

// RecommendedPSU returns the PSU wattage for an estimated draw: 30% headroom, rounded up to the
// next 50 W. 0 for 0.
func RecommendedPSU(estimateW int) int {
	return ceilDiv(ceilDiv(estimateW*psuHeadroomPct, 100), psuStepW) * psuStepW
}

func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}
