package api

const fixedOverheadSteps = 2

// ADTExportOptions mirrors the subset used for step counting.
type ADTExportOptions struct {
	MapsIncludeM2            bool
	MapsIncludeWMO           bool
	MapsIncludeGameObjects   bool
	MapsIncludeLiquid        bool
	MapsIncludeFoliage       bool
}

// ComputeStepsPerTile returns the fixed phase budget per tile.
func ComputeStepsPerTile(quality int, options ADTExportOptions) int {
	steps := fixedOverheadSteps
	if quality != 0 {
		steps++
	}
	if options.MapsIncludeM2 || options.MapsIncludeWMO || options.MapsIncludeGameObjects {
		steps++
	}
	if options.MapsIncludeLiquid {
		steps++
	}
	if options.MapsIncludeFoliage {
		steps++
	}
	return steps
}

func buildADTExportOptions(includeM2, includeWMO, includeWMOSets, includeGameObjects, includeLiquid, includeFoliage, includeHoles bool) ADTExportOptions {
	_ = includeWMOSets
	_ = includeHoles
	return ADTExportOptions{
		MapsIncludeM2:          includeM2,
		MapsIncludeWMO:         includeWMO,
		MapsIncludeGameObjects: includeGameObjects,
		MapsIncludeLiquid:      includeLiquid,
		MapsIncludeFoliage:     includeFoliage,
	}
}
