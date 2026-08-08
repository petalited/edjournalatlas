package main

import (
	"encoding/json"
)

// shipnames.go: real ship TYPE display names (e.g. "smallcombat01_nx" -> "Kestrel Mk II") --
// distinct from ship-modules.go's per-module naming. Without this, ships would show their raw
// internal type symbol instead of a real name.
//
// Two sources, real data preferred when available: this commander's own real
// ShipyardBuy/ShipyardSwap/ShipyardNew events carry a real ShipType_Localised field for
// whichever ships they've actually bought/swapped to (same real per-commander mechanism
// summary.go's own BuildRecap already uses for its "current ship" stat) -- but confirmed
// directly against this commander's own data that coverage is real but incomplete (12 of their
// 17 real fleet ship types were covered this way; the starter ship and anything bought before
// this commander's captured journal history began have no such event on record at all). Falls
// back to a small static table for the rest, sourced from EDCD/FDevIDs' own `shipyard.csv`
// (https://github.com/EDCD/FDevIDs/blob/master/shipyard.csv, fetched 2026-08-06) -- the same
// trusted org/source already used for `material.csv` and (this session) the module-symbol
// cross-reference -- covering every real ship symbol in the game, so this always has an answer
// even for a stored ship whose purchase predates this commander's own journal history.
var shipTypeDisplayNames = map[string]string{
	"sidewinder": "Sidewinder", "eagle": "Eagle", "hauler": "Hauler", "adder": "Adder",
	"viper": "Viper MkIII", "cobramkiii": "Cobra MkIII", "type6": "Type-6 Transporter",
	"dolphin": "Dolphin", "type7": "Type-7 Transporter", "asp": "Asp Explorer",
	"vulture": "Vulture", "empire_trader": "Imperial Clipper",
	"federation_dropship": "Federal Dropship", "orca": "Orca", "type9": "Type-9 Heavy",
	"python": "Python", "belugaliner": "Beluga Liner", "ferdelance": "Fer-de-Lance",
	"anaconda": "Anaconda", "federation_corvette": "Federal Corvette", "cutter": "Imperial Cutter",
	"diamondback": "Diamondback Scout", "empire_courier": "Imperial Courier",
	"diamondbackxl": "Diamondback Explorer", "empire_eagle": "Imperial Eagle",
	"federation_dropship_mkii": "Federal Assault Ship", "federation_gunship": "Federal Gunship",
	"viper_mkiv": "Viper MkIV", "cobramkiv": "Cobra MkIV", "independant_trader": "Keelback",
	"asp_scout": "Asp Scout", "type9_military": "Type-10 Defender", "krait_mkii": "Krait MkII",
	"typex": "Alliance Chieftain", "typex_2": "Alliance Crusader", "typex_3": "Alliance Challenger",
	"krait_light": "Krait Phantom", "mamba": "Mamba", "python_nx": "Python MkII",
	"type8": "Type-8 Transporter", "mandalay": "Mandalay", "cobramkv": "Cobra MkV",
	"corsair": "Corsair", "panthermkii": "Panther Clipper MkII", "lakonminer": "Type-11 Prospector",
	"explorer_nx": "Caspian Explorer", "smallcombat01_nx": "Kestrel Mk II",
	"mediumtransport01": "Lynx Highliner",
}

// shipTypeName resolves a real journal ship-type symbol to a real display name -- real
// per-commander localised names (captured from ShipyardBuy/Swap/New, passed in via
// localCaptured) take priority when available, falling back to the static table above, and
// finally to a prettified raw symbol if truly unrecognized (a brand-new ship this table hasn't
// been updated for yet -- same honest-fallback discipline as every other unmapped symbol in this
// project). Both the static table and localCaptured are keyed by normalizeMaterialKey (strip
// non-alphanumeric, lowercase) rather than a plain case fold -- a real, confirmed casing quirk:
// Loadout's own "Ship" field is always plain lowercase ("python_nx"), but StoredShips' "ShipType"
// field uses TitleCase ("Empire_Trader") for the very same real ship -- a normalized key is the
// only way both sources reliably hit the same table entry.
func shipTypeName(shipType string, localCaptured map[string]string) string {
	key := normalizeMaterialKey(shipType)
	if name, ok := localCaptured[key]; ok && name != "" {
		return name
	}
	if name, ok := shipTypeDisplayNamesNormalized[key]; ok {
		return name
	}
	return prettifyKeyStandalone(shipType)
}

// shipTypeDisplayNamesNormalized: shipTypeDisplayNames re-keyed by normalizeMaterialKey, built
// once -- so the underscore-containing keys in the source table above (kept underscored for
// readability, matching the real FDevIDs symbol spelling) still match a TitleCase journal value.
var shipTypeDisplayNamesNormalized = func() map[string]string {
	m := make(map[string]string, len(shipTypeDisplayNames))
	for k, v := range shipTypeDisplayNames {
		m[normalizeMaterialKey(k)] = v
	}
	return m
}()

// captureRealShipTypeNames scans every real ShipyardBuy/ShipyardSwap/ShipyardNew event for a
// real ShipType/ShipType_Localised pair -- same mechanism/reasoning as summary.go's own
// BuildRecap uses for its "current ship" stat, kept independent here (not shared code) so this
// file doesn't have to reach into summary.go's private closures for an unrelated page.
func captureRealShipTypeNames(store *Store) map[string]string {
	names := map[string]string{}
	for _, e := range store.RawEvents {
		if e.Event != "ShipyardBuy" && e.Event != "ShipyardSwap" && e.Event != "ShipyardNew" {
			continue
		}
		var v shipyardTypeEvent
		if json.Unmarshal([]byte(e.Raw), &v) != nil {
			continue
		}
		if v.ShipType != "" && v.ShipTypeLocalised != "" {
			names[normalizeMaterialKey(v.ShipType)] = v.ShipTypeLocalised
		}
	}
	return names
}
