package main

// materialgrades.go -- Engineering material grades (1-5), not exposed anywhere in the journal
// (confirmed: neither the "Materials" snapshot event nor "MaterialCollected" carry a Grade field
// at all, just Name/Count). Source: EDCD/FDevIDs' material.csv
// (https://github.com/EDCD/FDevIDs/blob/master/material.csv), fetched 2026-08-04 -- the same
// community-maintained reference EDMC and other Elite Dangerous tools use for this exact mapping,
// keyed by the game's own internal material symbol (its "rarity" column is this grade value; ED
// materials don't have a separately-tracked rarity distinct from grade). Static game-balance data,
// not per-commander, so vendoring it here is the same call already made for the species value
// table (vendor/) and region-lookup data (vendor/) -- reference data that doesn't change from
// commander to commander, just occasionally from game patch to game patch.
//
// Keys are normalized (lowercased, non-alphanumeric stripped) rather than matched verbatim: the
// ~82 "vanilla" material symbols are confirmed against this project's own real journal data to
// arrive as plain lowercase with no separators (e.g. "conductivecomponents"), but the rarer
// Guardian/Thargoid ones (their CSV symbols contain underscores, e.g. "Guardian_PowerCell") have
// never actually shown up in any journal this project has real data from -- normalizing on both
// sides of the lookup means a real journal's actual casing/separator choice for those, whatever
// it turns out to be, still matches correctly instead of silently missing.
var materialGrades = map[string]int{
	// Raw
	"iron": 1, "nickel": 1, "tin": 3, "zinc": 2, "carbon": 1, "sulphur": 1, "phosphorus": 1,
	"manganese": 2, "selenium": 4, "chromium": 2, "vanadium": 2, "germanium": 2, "cadmium": 3,
	"tungsten": 3, "arsenic": 2, "molybdenum": 3, "niobium": 3, "zirconium": 2, "mercury": 3,
	"yttrium": 4, "tellurium": 4, "polonium": 4, "technetium": 4, "ruthenium": 4, "antimony": 4,
	"rhenium": 1, "lead": 1, "boron": 3,

	// Manufactured
	"gridresistors": 1, "crystalshards": 1, "temperedalloys": 1, "basicconductors": 1,
	"mechanicalscrap": 1, "heatconductionwiring": 1, "wornshieldemitters": 1, "compactcomposites": 1,
	"salvagedalloys": 1, "chemicalstorageunits": 1,
	"hybridcapacitors": 2, "uncutfocuscrystals": 2, "heatresistantceramics": 2,
	"conductivecomponents": 2, "mechanicalequipment": 2, "heatdispersionplate": 2,
	"shieldemitters": 2, "filamentcomposites": 2, "galvanisingalloys": 2, "chemicalprocessors": 2,
	"electrochemicalarrays": 3, "focuscrystals": 3, "precipitatedalloys": 3, "conductiveceramics": 3,
	"mechanicalcomponents": 3, "heatexchangers": 3, "shieldingsensors": 3, "highdensitycomposites": 3,
	"phasealloys": 3, "chemicaldistillery": 3,
	"polymercapacitors": 4, "refinedfocuscrystals": 4, "thermicalloys": 4, "conductivepolymers": 4,
	"configurablecomponents": 4, "heatvanes": 4, "compoundshielding": 4,
	"fedproprietarycomposites": 4, "protolightalloys": 4, "chemicalmanipulators": 4,
	"militarysupercapacitors": 5, "exquisitefocuscrystals": 5, "militarygradealloys": 5,
	"biotechconductors": 5, "improvisedcomponents": 5, "protoheatradiators": 5,
	"imperialshielding": 5, "fedcorecomposites": 5, "protoradiolicalloys": 5,
	"pharmaceuticalisolators": 5,

	// Encoded
	"legacyfirmware": 1, "encryptedfiles": 1, "bulkscandata": 1, "disruptedwakeechoes": 1,
	"scrambledemissiondata": 1, "shieldcyclerecordings": 1,
	"consumerfirmware": 2, "encryptioncodes": 2, "scanarchives": 2, "fsdtelemetry": 2,
	"archivedemissiondata": 2, "shieldsoakanalysis": 2,
	"industrialfirmware": 3, "symmetrickeys": 3, "scandatabanks": 3, "wakesolutions": 3,
	"emissiondata": 3, "shielddensityreports": 3,
	"securityfirmware": 4, "encryptionarchives": 4, "encodedscandata": 4,
	"hyperspacetrajectories": 4, "decodedemissiondata": 4, "shieldpatternanalysis": 4,
	"embeddedfirmware": 5, "adaptiveencryptors": 5, "classifiedscandata": 5, "dataminedwake": 5,
	"compactemissionsdata": 5, "shieldfrequencydata": 5,

	// Manufactured/Encoded oddities, Thargoid, and Guardian materials -- not confirmed against
	// this project's own real journal data (never encountered by the commander(s) this project
	// has tested against), sourced from the same CSV; see this file's header comment.
	"unknownenergysource": 5, "unknownshipsignature": 3, "unknownwakedata": 4,
	"ancientlanguagedata": 4, "ancientbiologicaldata": 4, "ancientculturaldata": 4,
	"ancienthistoricaldata": 4, "ancienttechnologicaldata": 4,
	"tgcompositiondata": 3, "tgresiduedata": 4, "tgstructuraldata": 2, "unknowncarapace": 2,
	"unknownenergycell": 3, "unknownorganiccircuitry": 5, "unknowntechnologycomponents": 4,
	"tgbiomechanicalconduits": 3, "tgpropulsionelement": 5, "tgweaponparts": 4,
	"tgwreckagecomponents": 3, "tgshipflightdata": 3, "tgshipsystemsdata": 4,
	"guardianpowercell": 1, "guardianpowerconduit": 2, "guardiantechcomponent": 3,
	"guardiansentinelweaponparts": 3, "guardiansentinelwreckagecomponents": 1,
	"guardianweaponblueprint": 4, "guardianmoduleblueprint": 4, "guardianvesselblueprint": 5,
}

func normalizeMaterialKey(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c+('a'-'A'))
		case c >= '0' && c <= '9':
			out = append(out, c)
		}
	}
	return string(out)
}

// materialGrade returns 0 (not 1-5, so callers can tell "unknown" apart from a real grade) for a
// material this table has no entry for -- expected for something genuinely new to the game since
// this table was last updated, not treated as an error.
func materialGrade(name string) int {
	return materialGrades[normalizeMaterialKey(name)]
}
