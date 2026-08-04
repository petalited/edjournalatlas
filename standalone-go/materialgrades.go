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

// Most materials aren't just "a pile of grade-N stuff" -- they're organized into named families
// (Capacitors, Crystals, Encoded Firmware, ...), one material per grade 1-5 per family, and
// that's a real in-game/community grouping, not an invented one: it's the same `category` column
// already sitting right there in the vendored EDCD material.csv (see this file's header), and
// it's confirmed to be exactly the grouping the game itself uses for Material Trader "trade
// lines" (trading within the same family/category is what a Material Trader actually does) --
// cross-checked against a second independent source (the Elite Dangerous wiki's Raw Materials
// trade-line writeup), not just taken on the CSV's say-so.
//
// Raw materials are the one wrinkle: the CSV's `category` column for Raw is a bare number (1-7),
// not a text name like Manufactured/Encoded get -- there IS no further official name for these
// beyond "Category N" (confirmed against the same wiki source: "only referred to numerically"),
// so "Category N" here is the real, complete name, not a placeholder standing in for one this
// project couldn't find. Raw families hold 4 grades each (1-4), not 5 -- Raw materials simply
// don't go to grade 5 in this game.
//
// The Guardian/Thargoid/Unknown-* materials (material.csv's `category` is literally "None" for
// these) don't belong to any family at all -- they're excluded from this table entirely and
// stay grade-only (materialGrades above already covers them), same as everything else that
// isn't part of the grade-1-through-N ladder these families represent.
type materialFamilySlot struct {
	Key     string // normalized material key
	Display string // vendored official name -- fallback only; the real per-commander Name_Localised is always preferred when the material is actually held
}

type materialFamilyDef struct {
	Name  string // "Capacitors", "Encoded Firmware", "Category 1", etc -- real in-game/CSV names, not invented
	Type  string // "raw", "manufactured", or "encoded" -- which inventory category this family belongs to
	Slots []materialFamilySlot
}

var materialFamilyCatalog = []materialFamilyDef{
	{Name: "Category 1", Type: "raw", Slots: []materialFamilySlot{
		{"carbon", "Carbon"}, {"vanadium", "Vanadium"}, {"niobium", "Niobium"}, {"yttrium", "Yttrium"},
	}},
	{Name: "Category 2", Type: "raw", Slots: []materialFamilySlot{
		{"phosphorus", "Phosphorus"}, {"chromium", "Chromium"}, {"molybdenum", "Molybdenum"}, {"technetium", "Technetium"},
	}},
	{Name: "Category 3", Type: "raw", Slots: []materialFamilySlot{
		{"sulphur", "Sulphur"}, {"manganese", "Manganese"}, {"cadmium", "Cadmium"}, {"ruthenium", "Ruthenium"},
	}},
	{Name: "Category 4", Type: "raw", Slots: []materialFamilySlot{
		{"iron", "Iron"}, {"zinc", "Zinc"}, {"tin", "Tin"}, {"selenium", "Selenium"},
	}},
	{Name: "Category 5", Type: "raw", Slots: []materialFamilySlot{
		{"nickel", "Nickel"}, {"germanium", "Germanium"}, {"tungsten", "Tungsten"}, {"tellurium", "Tellurium"},
	}},
	{Name: "Category 6", Type: "raw", Slots: []materialFamilySlot{
		{"rhenium", "Rhenium"}, {"arsenic", "Arsenic"}, {"mercury", "Mercury"}, {"polonium", "Polonium"},
	}},
	{Name: "Category 7", Type: "raw", Slots: []materialFamilySlot{
		{"lead", "Lead"}, {"zirconium", "Zirconium"}, {"boron", "Boron"}, {"antimony", "Antimony"},
	}},

	{Name: "Capacitors", Type: "manufactured", Slots: []materialFamilySlot{
		{"gridresistors", "Grid Resistors"}, {"hybridcapacitors", "Hybrid Capacitors"},
		{"electrochemicalarrays", "Electrochemical Arrays"}, {"polymercapacitors", "Polymer Capacitors"},
		{"militarysupercapacitors", "Military Supercapacitors"},
	}},
	{Name: "Crystals", Type: "manufactured", Slots: []materialFamilySlot{
		{"crystalshards", "Crystal Shards"}, {"uncutfocuscrystals", "Flawed Focus Crystals"},
		{"focuscrystals", "Focus Crystals"}, {"refinedfocuscrystals", "Refined Focus Crystals"},
		{"exquisitefocuscrystals", "Exquisite Focus Crystals"},
	}},
	{Name: "Thermic", Type: "manufactured", Slots: []materialFamilySlot{
		{"temperedalloys", "Tempered Alloys"}, {"heatresistantceramics", "Heat Resistant Ceramics"},
		{"precipitatedalloys", "Precipitated Alloys"}, {"thermicalloys", "Thermic Alloys"},
		{"militarygradealloys", "Military Grade Alloys"},
	}},
	{Name: "Conductive", Type: "manufactured", Slots: []materialFamilySlot{
		{"basicconductors", "Basic Conductors"}, {"conductivecomponents", "Conductive Components"},
		{"conductiveceramics", "Conductive Ceramics"}, {"conductivepolymers", "Conductive Polymers"},
		{"biotechconductors", "Biotech Conductors"},
	}},
	{Name: "Mechanical Components", Type: "manufactured", Slots: []materialFamilySlot{
		{"mechanicalscrap", "Mechanical Scrap"}, {"mechanicalequipment", "Mechanical Equipment"},
		{"mechanicalcomponents", "Mechanical Components"}, {"configurablecomponents", "Configurable Components"},
		{"improvisedcomponents", "Improvised Components"},
	}},
	{Name: "Heat", Type: "manufactured", Slots: []materialFamilySlot{
		{"heatconductionwiring", "Heat Conduction Wiring"}, {"heatdispersionplate", "Heat Dispersion Plate"},
		{"heatexchangers", "Heat Exchangers"}, {"heatvanes", "Heat Vanes"},
		{"protoheatradiators", "Proto Heat Radiators"},
	}},
	{Name: "Shielding", Type: "manufactured", Slots: []materialFamilySlot{
		{"wornshieldemitters", "Worn Shield Emitters"}, {"shieldemitters", "Shield Emitters"},
		{"shieldingsensors", "Shielding Sensors"}, {"compoundshielding", "Compound Shielding"},
		{"imperialshielding", "Imperial Shielding"},
	}},
	{Name: "Composite", Type: "manufactured", Slots: []materialFamilySlot{
		{"compactcomposites", "Compact Composites"}, {"filamentcomposites", "Filament Composites"},
		{"highdensitycomposites", "High Density Composites"}, {"fedproprietarycomposites", "Proprietary Composites"},
		{"fedcorecomposites", "Core Dynamics Composites"},
	}},
	{Name: "Alloys", Type: "manufactured", Slots: []materialFamilySlot{
		{"salvagedalloys", "Salvaged Alloys"}, {"galvanisingalloys", "Galvanising Alloys"},
		{"phasealloys", "Phase Alloys"}, {"protolightalloys", "Proto Light Alloys"},
		{"protoradiolicalloys", "Proto Radiolic Alloys"},
	}},
	{Name: "Chemical", Type: "manufactured", Slots: []materialFamilySlot{
		{"chemicalstorageunits", "Chemical Storage Units"}, {"chemicalprocessors", "Chemical Processors"},
		{"chemicaldistillery", "Chemical Distillery"}, {"chemicalmanipulators", "Chemical Manipulators"},
		{"pharmaceuticalisolators", "Pharmaceutical Isolators"},
	}},

	{Name: "Encoded Firmware", Type: "encoded", Slots: []materialFamilySlot{
		{"legacyfirmware", "Specialised Legacy Firmware"}, {"consumerfirmware", "Modified Consumer Firmware"},
		{"industrialfirmware", "Cracked Industrial Firmware"}, {"securityfirmware", "Security Firmware Patch"},
		{"embeddedfirmware", "Modified Embedded Firmware"},
	}},
	{Name: "Encryption Files", Type: "encoded", Slots: []materialFamilySlot{
		{"encryptedfiles", "Unusual Encrypted Files"}, {"encryptioncodes", "Tagged Encryption Codes"},
		{"symmetrickeys", "Open Symmetric Keys"}, {"encryptionarchives", "Atypical Encryption Archives"},
		{"adaptiveencryptors", "Adaptive Encryptors Capture"},
	}},
	{Name: "Data Archives", Type: "encoded", Slots: []materialFamilySlot{
		{"bulkscandata", "Anomalous Bulk Scan Data"}, {"scanarchives", "Unidentified Scan Archives"},
		{"scandatabanks", "Classified Scan Databanks"}, {"encodedscandata", "Divergent Scan Data"},
		{"classifiedscandata", "Classified Scan Fragment"},
	}},
	{Name: "Wake Scans", Type: "encoded", Slots: []materialFamilySlot{
		{"disruptedwakeechoes", "Atypical Disrupted Wake Echoes"}, {"fsdtelemetry", "Anomalous FSD Telemetry"},
		{"wakesolutions", "Strange Wake Solutions"}, {"hyperspacetrajectories", "Eccentric Hyperspace Trajectories"},
		{"dataminedwake", "Datamined Wake Exceptions"},
	}},
	{Name: "Emission Data", Type: "encoded", Slots: []materialFamilySlot{
		{"scrambledemissiondata", "Exceptional Scrambled Emission Data"}, {"archivedemissiondata", "Irregular Emission Data"},
		{"emissiondata", "Unexpected Emission Data"}, {"decodedemissiondata", "Decoded Emission Data"},
		{"compactemissionsdata", "Abnormal Compact Emissions Data"},
	}},
	{Name: "Shield Data", Type: "encoded", Slots: []materialFamilySlot{
		{"shieldcyclerecordings", "Distorted Shield Cycle Recordings"}, {"shieldsoakanalysis", "Inconsistent Shield Soak Analysis"},
		{"shielddensityreports", "Untypical Shield Scans"}, {"shieldpatternanalysis", "Aberrant Shield Pattern Analysis"},
		{"shieldfrequencydata", "Peculiar Shield Frequency Data"},
	}},
}

// materialFamilyByKey maps a normalized material key straight to its family name -- built once
// from materialFamilyCatalog rather than hand-duplicated, so the two can never drift apart.
var materialFamilyByKey = func() map[string]string {
	m := map[string]string{}
	for _, fam := range materialFamilyCatalog {
		for _, slot := range fam.Slots {
			m[slot.Key] = fam.Name
		}
	}
	return m
}()

// materialFamily returns "" for anything not part of a named family (Guardian/Thargoid/Unknown
// materials, or anything new enough to the game that this table hasn't been updated for yet) --
// distinct from a real family name, same "0/empty means unknown" convention as materialGrade.
func materialFamily(name string) string {
	return materialFamilyByKey[normalizeMaterialKey(name)]
}

// materialDisplayNames covers every key in materialGrades (checked at generation time, not just
// the 108 that belong to a named family) -- the canonical name from the same EDCD material.csv
// "name" column, needed so the engineering planner (engineeringplanner.go) can show a real
// material name for an ingredient the commander doesn't currently hold at all, where there's no
// real per-commander Name_Localised available to fall back on.
var materialDisplayNames = map[string]string{
	"adaptiveencryptors": "Adaptive Encryptors Capture", "ancientbiologicaldata": "Pattern Alpha Obelisk Data", "ancientculturaldata": "Pattern Beta Obelisk Data",
	"ancienthistoricaldata": "Pattern Gamma Obelisk Data", "ancientlanguagedata": "Pattern Delta Obelisk Data", "ancienttechnologicaldata": "Pattern Epsilon Obelisk Data",
	"antimony": "Antimony", "archivedemissiondata": "Irregular Emission Data", "arsenic": "Arsenic",
	"basicconductors": "Basic Conductors", "biotechconductors": "Biotech Conductors", "boron": "Boron",
	"bulkscandata": "Anomalous Bulk Scan Data", "cadmium": "Cadmium", "carbon": "Carbon",
	"chemicaldistillery": "Chemical Distillery", "chemicalmanipulators": "Chemical Manipulators", "chemicalprocessors": "Chemical Processors",
	"chemicalstorageunits": "Chemical Storage Units", "chromium": "Chromium", "classifiedscandata": "Classified Scan Fragment",
	"compactcomposites": "Compact Composites", "compactemissionsdata": "Abnormal Compact Emissions Data", "compoundshielding": "Compound Shielding",
	"conductiveceramics": "Conductive Ceramics", "conductivecomponents": "Conductive Components", "conductivepolymers": "Conductive Polymers",
	"configurablecomponents": "Configurable Components", "consumerfirmware": "Modified Consumer Firmware", "crystalshards": "Crystal Shards",
	"dataminedwake": "Datamined Wake Exceptions", "decodedemissiondata": "Decoded Emission Data", "disruptedwakeechoes": "Atypical Disrupted Wake Echoes",
	"electrochemicalarrays": "Electrochemical Arrays", "embeddedfirmware": "Modified Embedded Firmware", "emissiondata": "Unexpected Emission Data",
	"encodedscandata": "Divergent Scan Data", "encryptedfiles": "Unusual Encrypted Files", "encryptionarchives": "Atypical Encryption Archives",
	"encryptioncodes": "Tagged Encryption Codes", "exquisitefocuscrystals": "Exquisite Focus Crystals", "fedcorecomposites": "Core Dynamics Composites",
	"fedproprietarycomposites": "Proprietary Composites", "filamentcomposites": "Filament Composites", "focuscrystals": "Focus Crystals",
	"fsdtelemetry": "Anomalous FSD Telemetry", "galvanisingalloys": "Galvanising Alloys", "germanium": "Germanium",
	"gridresistors": "Grid Resistors", "guardianmoduleblueprint": "Guardian Module Blueprint Segment", "guardianpowercell": "Guardian Power Cell",
	"guardianpowerconduit": "Guardian Power Conduit", "guardiansentinelweaponparts": "Guardian Sentinel Weapon Parts", "guardiansentinelwreckagecomponents": "Guardian Sentinel Wreckage Components",
	"guardiantechcomponent": "Guardian Technology Component", "guardianvesselblueprint": "Guardian Vessel Blueprint Segment", "guardianweaponblueprint": "Guardian Weapon Blueprint Segment",
	"heatconductionwiring": "Heat Conduction Wiring", "heatdispersionplate": "Heat Dispersion Plate", "heatexchangers": "Heat Exchangers",
	"heatresistantceramics": "Heat Resistant Ceramics", "heatvanes": "Heat Vanes", "highdensitycomposites": "High Density Composites",
	"hybridcapacitors": "Hybrid Capacitors", "hyperspacetrajectories": "Eccentric Hyperspace Trajectories", "imperialshielding": "Imperial Shielding",
	"improvisedcomponents": "Improvised Components", "industrialfirmware": "Cracked Industrial Firmware", "iron": "Iron",
	"lead": "Lead", "legacyfirmware": "Specialised Legacy Firmware", "manganese": "Manganese",
	"mechanicalcomponents": "Mechanical Components", "mechanicalequipment": "Mechanical Equipment", "mechanicalscrap": "Mechanical Scrap",
	"mercury": "Mercury", "militarygradealloys": "Military Grade Alloys", "militarysupercapacitors": "Military Supercapacitors",
	"molybdenum": "Molybdenum", "nickel": "Nickel", "niobium": "Niobium",
	"pharmaceuticalisolators": "Pharmaceutical Isolators", "phasealloys": "Phase Alloys", "phosphorus": "Phosphorus",
	"polonium": "Polonium", "polymercapacitors": "Polymer Capacitors", "precipitatedalloys": "Precipitated Alloys",
	"protoheatradiators": "Proto Heat Radiators", "protolightalloys": "Proto Light Alloys", "protoradiolicalloys": "Proto Radiolic Alloys",
	"refinedfocuscrystals": "Refined Focus Crystals", "rhenium": "Rhenium", "ruthenium": "Ruthenium",
	"salvagedalloys": "Salvaged Alloys", "scanarchives": "Unidentified Scan Archives", "scandatabanks": "Classified Scan Databanks",
	"scrambledemissiondata": "Exceptional Scrambled Emission Data", "securityfirmware": "Security Firmware Patch", "selenium": "Selenium",
	"shieldcyclerecordings": "Distorted Shield Cycle Recordings", "shielddensityreports": "Untypical Shield Scans", "shieldemitters": "Shield Emitters",
	"shieldfrequencydata": "Peculiar Shield Frequency Data", "shieldingsensors": "Shielding Sensors", "shieldpatternanalysis": "Aberrant Shield Pattern Analysis",
	"shieldsoakanalysis": "Inconsistent Shield Soak Analysis", "sulphur": "Sulphur", "symmetrickeys": "Open Symmetric Keys",
	"technetium": "Technetium", "tellurium": "Tellurium", "temperedalloys": "Tempered Alloys",
	"tgbiomechanicalconduits": "Bio-Mechanical Conduits", "tgcompositiondata": "Thargoid Material Composition Data", "tgpropulsionelement": "Propulsion Elements",
	"tgresiduedata": "Thargoid Residue Data", "tgshipflightdata": "Ship Flight Data", "tgshipsystemsdata": "Ship Systems Data",
	"tgstructuraldata": "Thargoid Structural Data", "tgweaponparts": "Weapon Parts", "tgwreckagecomponents": "Wreckage Components",
	"thermicalloys": "Thermic Alloys", "tin": "Tin", "tungsten": "Tungsten",
	"uncutfocuscrystals": "Flawed Focus Crystals", "unknowncarapace": "Thargoid Carapace", "unknownenergycell": "Thargoid Energy Cell",
	"unknownenergysource": "Sensor Fragment", "unknownorganiccircuitry": "Thargoid Organic Circuitry", "unknownshipsignature": "Thargoid Ship Signature",
	"unknowntechnologycomponents": "Thargoid Technological Components", "unknownwakedata": "Thargoid Wake Data", "vanadium": "Vanadium",
	"wakesolutions": "Strange Wake Solutions", "wornshieldemitters": "Worn Shield Emitters", "yttrium": "Yttrium",
	"zinc": "Zinc", "zirconium": "Zirconium",
}

// materialDisplayName falls back to the raw key itself (title-cased) for anything genuinely
// missing from the table -- shouldn't happen for the 137 keys this covers, but stays safe rather
// than panicking or showing an empty string if the game ever adds something new.
func materialDisplayName(key string) string {
	if name, ok := materialDisplayNames[key]; ok {
		return name
	}
	return prettifyKeyStandalone(key)
}
