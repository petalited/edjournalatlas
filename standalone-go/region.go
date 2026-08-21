package main

// Offline (x, y, z) -> galactic region name lookup. No network call. Ported from
// this project's Python version's region_map.py (itself reimplemented from EDMC-ExploData's
// RegionMap.py, GPLv2) -- see regiondata.json (generated from the same vendored
// RegionMapData.py table) and its own README under standalone/vendor/explodata_region_data/
// for full provenance. Matched exactly ("Inner Orion Spur") against a real CodexEntry event's
// own Region_Localised field for the same coordinate in the Python version.

import (
	_ "embed"
	"encoding/json"
)

//go:embed regiondata.json
var regionDataJSON []byte

type regionData struct {
	Regions   []string    `json:"regions"`
	RegionMap [][][2]int  `json:"regionmap"`
}

var regionTable regionData

func init() {
	if err := json.Unmarshal(regionDataJSON, &regionTable); err != nil {
		panic("failed to parse embedded region data: " + err.Error())
	}
}

// Galactic coordinate grid origin -- same constants the original findRegion() uses; the grid
// is a fixed 83-column run-length-encoded map of the galaxy starting at this offset.
const regionX0 = -49985
const regionZ0 = -24105

func findRegion(x, y, z float64) string {
	px := int((x - regionX0) * 83 / 4096)
	pz := int((z - regionZ0) * 83 / 4096)

	if px < 0 || pz < 0 || pz >= len(regionTable.RegionMap) {
		return ""
	}

	row := regionTable.RegionMap[pz]
	rx := 0
	regionID := 0
	for _, pair := range row {
		runLength, value := pair[0], pair[1]
		if px < rx+runLength {
			regionID = value
			break
		}
		rx += runLength
	}

	if regionID == 0 || regionID >= len(regionTable.Regions) {
		return ""
	}
	return regionTable.Regions[regionID]
}
