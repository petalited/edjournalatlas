package main

// Loads the vendored BioScan-derived species value table and ExploData's genus display names.
// Same data as edexotracker's standalone/value_table.py, embedded directly into the binary via
// go:embed instead of read from disk at runtime -- see standalone/vendor/bio_value_data/README.md
// for full provenance/refresh instructions.

import (
	_ "embed"
	"encoding/json"
)

//go:embed vendor/value_table.json
var valueTableJSON []byte

//go:embed vendor/genus_names.json
var genusNamesJSON []byte

type speciesInfo struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

var valueTable map[string]map[string]speciesInfo
var genusNames map[string]string

func init() {
	if err := json.Unmarshal(valueTableJSON, &valueTable); err != nil {
		panic("failed to parse embedded value table: " + err.Error())
	}
	if err := json.Unmarshal(genusNamesJSON, &genusNames); err != nil {
		panic("failed to parse embedded genus names: " + err.Error())
	}
}

func lookupSpecies(genus, species string) (speciesInfo, bool) {
	if genus == "" || species == "" {
		return speciesInfo{}, false
	}
	sp, ok := valueTable[genus]
	if !ok {
		return speciesInfo{}, false
	}
	info, ok := sp[species]
	return info, ok
}

func genusDisplayName(genus string) string {
	if name, ok := genusNames[genus]; ok {
		return name
	}
	return genus
}
