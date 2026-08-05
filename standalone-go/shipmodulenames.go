package main

// shipmodulenames.go: turns a real journal module `Item` symbol (e.g. "int_powerplant_size8_
// class5", "hpt_pulselaserburst_turret_small") into a real player-recognizable display name, and
// separately (for resolveBlueprintSymbol's disambiguation, shipmodulemaps.go) into this project's
// own catalog Type string when the module belongs to an engineerable family.
//
// No `Item_Localised` field exists anywhere in the journal (confirmed: same as Raw materials
// never getting a Name_Localised, checked directly against this project's own real data) --
// module names have to be derived, not read off the journal directly. Rather than vendor a full
// per-module database (the same real licensing/tech-stack problem this whole page ran into with
// Coriolis, see docs/ShipyardPlanner.md), this parses the `Item` symbol's own structural
// convention directly: `int_<type>_size<N>_class<C>` for internal compartments,
// `hpt_<weapon>_<mount>_<size>` for hardpoints -- both real, stable, well-known encodings that
// haven't changed in years. The size/class -> in-game "6A"-style shorthand (class 5=A best down
// to 1=E worst) is the exact convention players themselves use.
//
// The keyword dictionary below was built from two sources: this project's own real fleet data
// (every distinct real Item symbol this commander's own ships have ever used, pulled directly
// from their `.db`) for guaranteed-correct entries, plus this author's own general knowledge of
// Elite Dangerous for a handful of additional common module types not present in this one
// commander's fleet -- flagged inline where the entry isn't independently verified against real
// data. Anything genuinely unrecognized falls back to a plain prettified version of the raw
// symbol (underscores to spaces, title case) -- same "readable but honest" fallback already used
// elsewhere in this project (materials.go's own prettifyKeyStandalone, summary.go's
// formatBlueprintName) -- never a raw, ugly `int_whatever_size3_class2` string shown verbatim.
//
// Real bug, fixed: the first version of this file guessed "slugshot" -> Rail Gun purely from
// memory, marked [known, not independently confirmed] -- wrong. A real owner report ("PE-24P has
// fragment cannons not railguns") caught it: "hpt_slugshot_*" is genuinely Fragment Cannon's real
// internal symbol, confirmed directly against EDCD/FDevIDs' own `outfitting.csv` (the same
// trusted source already used for `material.csv`) and independently cross-checked against four
// more unrelated community sources that all agree. The real Rail Gun symbol is the much more
// literal "hpt_railgun_*". A concrete lesson from this: [known]-tagged entries in this file are a
// real risk, not just a formality -- verify against an authoritative source before trusting
// memory, the same discipline already applied everywhere else in this project.

import (
	"fmt"
	"regexp"
	"strings"
)

// moduleTypeKeywords: Item symbol substring -> this project's own blueprintCatalog/effectCatalog
// Type string, for modules that ARE engineerable (used only for resolveBlueprintSymbol's
// disambiguation, not display). Checked longest-keyword-first so e.g. "multicannon" matches
// before the shorter "cannon" would. Every entry marked [real] was confirmed directly against
// this commander's own real fleet data; [known] entries are this author's own domain knowledge,
// not independently verified against real data, and may need a correction if ever proven wrong.
var moduleTypeKeywords = map[string]string{
	// Core internals [real]
	"powerplant":       "Power Plant",
	"hyperdrive":       "Frame Shift Drive", // covers both plain and "_overcharge_" variants -- same base Type, just a bigger/boosted class
	"lifesupport":      "Life Support",
	"powerdistributor": "Power Distributor",
	"sensors":          "Sensors",
	// Thrusters are handled separately (see engineItemInfix/resolveModuleType) since a bare
	// "engine" keyword would false-match cosmetic "enginecustomisation_white"-style symbols.

	// Optional internals [real]
	"shieldgenerator":             "Shield Generator",
	"shieldcellbank":              "Shield Cell Bank",
	"hullreinforcement":           "Hull Reinforcement Package",
	"refinery":                    "Refinery",
	"repairer":                    "Auto Field-Maintenance Unit",
	"fuelscoop":                   "Fuel Scoop",
	"fsdinterdictor":              "Frame Shift Drive Interdictor",
	"dronecontrol_collection":     "Collector Limpet Controller",
	"dronecontrol_prospector":     "Prospector Limpet Controller",
	"dronecontrol_fueltransfer":   "Fuel Transfer Limpet Controller",
	"dronecontrol_resourcesiphon": "Hatch Breaker Limpet Controller",

	// Hardpoints [real]
	"pulselaserburst":          "Burst Laser", // checked before "pulselaser" -- longest-match-first below handles the ordering
	"beamlaser":                "Beam Laser",
	"pulselaser":               "Pulse Laser",
	"multicannon":              "Multi-cannon",
	"slugshot":                 "Fragment Cannon", // real bug, now fixed: this was wrongly mapped to "Rail Gun" (this author's own unverified guess, see git history) -- a real owner report ("PE-24P has fragment cannons not railguns") caught it. Verified directly against EDCD/FDevIDs' own outfitting.csv (the same trusted source already used for material.csv): "Hpt_Slugshot_*" is Fragment Cannon, full stop, cross-confirmed independently against four more unrelated community sources (EDDI, EDMarketConnector, an EDDN mapper, icarus's vendored Coriolis data) that all agree.
	"advancedtorppylon":        "Torpedo Pylon",
	"basicmissilerack":         "Missile Rack",
	"dumbfiremissilerack":      "Missile Rack",
	"drunkmissilerack":         "Missile Rack",
	"chafflauncher":            "Chaff Launcher",
	"heatsinklauncher":         "Heat Sink Launcher",
	"electroniccountermeasure": "Electronic Countermeasure",
	"shieldbooster":            "Shield Booster",
	"plasmapointdefence":       "Point Defence",
	"crimescanner":             "Kill Warrant Scanner",
	"cargoscanner":             "Manifest Scanner", // real bug, now fixed: this project previously had NO entry for Manifest Scanner's actual symbol at all -- it had wrongly assumed "mrascanner" (see below) meant Manifest Scanner, which left the real symbol ("hpt_cargoscanner_*") completely unmapped. Confirmed directly against EDCD/FDevIDs' own outfitting.csv.
	"railgun":                  "Rail Gun",         // the REAL Rail Gun symbol ("Hpt_Railgun_*"), confirmed via the same EDCD/FDevIDs cross-check above -- not present in this commander's own fleet to verify against directly, but independently confirmed authoritative
	"cannon":                   "Cannon",           // must be checked after multicannon (longest-match-first)

	// Hardpoints [known, not independently confirmed against this commander's own real fleet]
	"plasmaaccelerator": "Plasma Accelerator",
}

// nonEngineerableItemPrefixes: real module symbols that, despite superficially matching a
// moduleTypeKeywords substring (e.g. "hpt_mkiiplasmashockautocannon_..." contains "cannon"),
// are genuinely NOT part of this project's engineerable catalog -- checked BEFORE the keyword
// scan in resolveModuleType so they never get misattributed to a real Type, however close the
// substring match looks. Two real, confirmed cases (owner report, "PE-24P has fragment cannons
// not railguns" prompted a full audit of all 323 distinct real Item symbols this commander's
// fleet has ever used against EDCD/EDDI's authoritative ModuleDefinitions.cs):
//   - "hpt_mrascanner_*" is the Pulse Wave Analyser (a Guardian-ruin detection utility),
//     confirmed via EDCD/FDevIDs' outfitting.csv -- NOT Manifest Scanner, which this project had
//     wrongly assumed. Pulse Wave Analyser isn't in this project's vendored blueprint catalog at
//     all (checked directly) -- not a standard-engineerable module, so this correctly resolves
//     to no Type; it still gets a real cosmetic display name via cosmeticModuleKeywords below.
//   - "hpt_mkiiplasmashockautocannon_*" (the Mandalay's MkII Plasma Shock Accelerator) is
//     confirmed, directly researched and not engineerable at all as of this writing -- Shock
//     Cannon-family weapons have no engineering blueprints in the real game yet, so offering a
//     "push to next grade" button for one would be actively wrong, not just imprecise.
//
// The rest of the 323-symbol audit (Pack-Hound/"drunkmissilerack", Advanced Missile Rack/
// "dumbfiremissilerack_..._advanced", Pacifier Frag Cannon/"slugshot_..._large_range", Bi-Weave
// shields, Enhanced Performance/Gravity Optimised thrusters) all independently confirmed to
// share their base weapon/module type's standard engineering blueprints -- correctly already
// resolving via the ordinary keyword match, no exclusion needed.
var nonEngineerableItemPrefixes = []string{
	"hpt_mrascanner_",
	"hpt_mkiiplasmashockautocannon_",
}

// engineItemInfix is checked as a substring (not just a prefix) rather than folded into
// moduleTypeKeywords, since a bare "engine" keyword would also false-match cosmetic
// "enginecustomisation_white"-style symbols. "_engine_" (underscore-bounded) is the real,
// confirmed-safe discriminator: verified against all ~990 real hpt_/int_ symbols in EDCD/EDDI's
// own module data that every single one containing "_engine_" is a genuine Thrusters-family
// module (including real special variants like Mandalay's "int_mkiiagileboost_engine_..." --
// confirmed via research to share standard Thrusters engineering, same as Enhanced Performance/
// Gravity Optimised variants), with zero false positives from anything else. A real, live gap
// this fixed: the old strict-prefix check (`HasPrefix(item, "int_engine_")`) missed exactly this
// Mandalay module, which fell through to a raw ugly symbol string in the UI instead of "Thrusters".
const engineItemInfix = "_engine_"

// moduleTypeSortedKeywords: moduleTypeKeywords' keys, longest first, computed once -- so a
// symbol containing both "cannon" and "multicannon" resolves to the more specific match.
var moduleTypeSortedKeywords = func() []string {
	keys := make([]string, 0, len(moduleTypeKeywords))
	for k := range moduleTypeKeywords {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && len(keys[j]) > len(keys[j-1]); j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}()

// resolveModuleType returns this project's own catalog Type string for a real module Item
// symbol, or "" if it's not a recognized engineerable module type (cosmetic items, fuel tanks,
// cargo racks, and any module type not yet in moduleTypeKeywords all correctly return "").
func resolveModuleType(item string) string {
	for _, prefix := range nonEngineerableItemPrefixes {
		if strings.HasPrefix(item, prefix) {
			return ""
		}
	}
	if strings.Contains(item, engineItemInfix) {
		return "Thrusters"
	}
	for _, kw := range moduleTypeSortedKeywords {
		if strings.Contains(item, kw) {
			return moduleTypeKeywords[kw]
		}
	}
	return ""
}

var classLetters = map[int]string{5: "A", 4: "B", 3: "C", 2: "D", 1: "E"}

// sizeClassRe matches the real, stable "_size<N>_class<C>" internal-compartment convention.
// Requires a nonzero size so "_size0_class2"-style utility hardpoints (shield boosters etc.,
// which use a real but different "size 0" convention) fall through to utilitySizeRe instead --
// there's no real in-game "size 0" internal compartment, so a literal "0A Shield Booster"-style
// label would be nonsensical.
var sizeClassRe = regexp.MustCompile(`_size([1-9]\d*)_class(\d+)`)

// hardpointSizeRe matches the real "_fixed_<size>" / "_gimbal_<size>" / "_turret_<size>"
// hardpoint-mount convention.
var hardpointSizeRe = regexp.MustCompile(`_(fixed|gimbal|turret)_(tiny|small|medium|large|huge)`)

// utilitySizeRe matches the simpler tiny-utility-mount convention with no fixed/gimbal/turret
// choice (shield boosters, chaff launchers, etc: "hpt_shieldbooster_size0_class2",
// "hpt_chafflauncher_tiny").
var utilitySizeRe = regexp.MustCompile(`_size0_class(\d+)|_tiny$`)

func classLetter(class int) string {
	if l, ok := classLetters[class]; ok {
		return l
	}
	return fmt.Sprintf("%d", class)
}

// prettifyModuleItem produces a real player-recognizable module name from its raw Item symbol --
// used for every fitted module shown on the shipyard page, not just engineerable ones (fuel
// tanks, cargo racks, cockpits, paint jobs all go through here too).
func prettifyModuleItem(item string) string {
	base := resolveModuleType(item)
	if base == "" {
		// Not an engineerable Type -- try the broader cosmetic/utility keyword set before
		// falling all the way back to a raw prettified symbol.
		base = cosmeticModuleName(item)
	}
	if base == "" {
		return prettifyKeyStandalone(strings.ReplaceAll(item, "_", " "))
	}

	if m := sizeClassRe.FindStringSubmatch(item); m != nil {
		size, class := m[1], atoiSafe(m[2])
		return fmt.Sprintf("%s%s %s", size, classLetter(class), base)
	}
	if m := hardpointSizeRe.FindStringSubmatch(item); m != nil {
		mount, size := strings.Title(m[1]), strings.Title(m[2])
		return fmt.Sprintf("%s %s %s", size, mount, base)
	}
	if m := utilitySizeRe.FindStringSubmatch(item); m != nil && m[1] != "" {
		return fmt.Sprintf("Class %s %s", classLetter(atoiSafe(m[1])), base)
	}
	return base
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// cosmeticModuleName covers common non-engineerable module families this commander's own real
// fleet actually uses (fuel tanks, cargo racks, life-support-adjacent utility, cockpits) so the
// shipyard page doesn't show a raw symbol for the majority of a ship's non-combat loadout either.
// Not exhaustive -- anything missing here still gets the honest raw-symbol fallback.
var cosmeticModuleKeywords = map[string]string{
	"largecargorack":            "Cargo Rack",
	"cargorack":                 "Cargo Rack",
	"fueltank":                  "Fuel Tank",
	"cockpit":                   "Cockpit",
	"armour_grade":              "Armour",
	"dockingcomputer_advanced":  "Advanced Docking Computer",
	"detailedsurfacescanner":    "Detailed Surface Scanner",
	"planetapproachsuite":       "Planetary Approach Suite",
	"guardianfsdbooster":        "Guardian FSD Booster",
	"supercruiseassist":         "Supercruise Assist",
	"modulereinforcement":       "Module Reinforcement Package",
	"passengercabin":            "Passenger Cabin",
	"buggybay":                  "Planetary Vehicle Hangar",
	"fighterbay":                "Fighter Hangar",
	"mrascanner":                "Pulse Wave Analyser",           // real, not engineerable -- see nonEngineerableItemPrefixes above
	"mkiiplasmashockautocannon": "MkII Plasma Shock Accelerator", // real, not engineerable -- see nonEngineerableItemPrefixes above
}

var cosmeticModuleSortedKeywords = func() []string {
	keys := make([]string, 0, len(cosmeticModuleKeywords))
	for k := range cosmeticModuleKeywords {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && len(keys[j]) > len(keys[j-1]); j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}()

func cosmeticModuleName(item string) string {
	for _, kw := range cosmeticModuleSortedKeywords {
		if strings.Contains(item, kw) {
			return cosmeticModuleKeywords[kw]
		}
	}
	return ""
}
