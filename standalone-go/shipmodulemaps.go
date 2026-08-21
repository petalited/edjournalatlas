package main

import "strings"

// shipmodulemaps.go: the real journal BlueprintName/ExperimentalEffect symbol -> catalog
// (Type, Name) cross-reference the shipyard page (shipyard.go) needs to turn a fitted module's
// actual engineering state into something the existing planner/trade-calculator (which is keyed
// by this project's own vendored blueprintCatalog/effectCatalog, see engineeringplanner.go) can
// act on. This mapping didn't exist anywhere in this project before -- neither this project's own
// vendored `vendor/blueprints.json` nor its original upstream source (msarilar/EDEngineer, checked
// directly) carries the journal's own BlueprintName symbol alongside its human Type/Name pair;
// upstream keys its own data by a "CoriolisGuid" instead, a different ID system entirely.
//
// Source: EDDiscovery/EliteDangerousCore's Recipes.cs (Apache 2.0,
// https://github.com/EDDiscovery/EliteDangerousCore/blob/master/EliteDangerous/FrontierData/
// Recipes/Recipes.cs, fetched 2026-08-05) -- the same real, independent, community-maintained
// project already trusted elsewhere in this codebase for the Elite rank prestige (Elite I-V) fix. Its
// `EngineeringRecipe` entries carry the real journal FDName/BlueprintName symbol alongside a
// human name and applicable module type(s) for essentially every real graded blueprint (59
// distinct symbols) and Experimental Effect (85 distinct symbols) in the game.
//
// Cross-referenced against this project's own vendored blueprintCatalog/effectCatalog by
// normalized name matching (lowercased, non-alphanumeric stripped, with the module-type name
// stripped as a prefix/suffix where EDDiscovery decorates its names that way, e.g. "Engine
// Focused Power Distributor" vs. this project's own "Power Distributor" | "Engine Focused") --
// not hand-typed, to rule out transcription error across ~140 entries. EDDiscovery's own
// module-type shorthand (e.g. "Frag Cannon", "FSD", "Multicannon") was translated to this
// project's own catalog Type strings via a small alias table before matching.
//
// Two real name-wording mismatches the automated match couldn't bridge on its own were resolved
// by hand, verified directly against this project's own vendored catalog rows: EDDiscovery calls
// them "Longer Range FSD Interdictor" / "Ion Disruption"; this project's own vendored data (and,
// checked independently, real community documentation) calls the same two real blueprints "Long
// Range FSD Interdictor" / "Ion Disruptor".
//
// One real, harmless, confirmed gap: `weapon_shortrange` doesn't resolve for Pulse Laser
// specifically -- this project's own vendored EDEngineer data has no "Short Range Blaster" entry
// for Pulse Laser at all (checked directly against the vendored catalog). A Pulse Laser
// engineered with this real blueprint just won't get an auto-populate button on the shipyard page
// -- same graceful degradation as any symbol not yet in this map at all (see shipyard.go).
//
// Not exhaustive by construction -- some journal symbols genuinely have no vendored blueprint at
// all (e.g. "CargoRack_IncreasedCapacity", real and confirmed via this commander's own journal,
// belongs to a pre-engineered reward Cargo Rack obtainable only via community goals, never a
// normal engineer -- correctly absent from both this map and blueprintCatalog for the same
// reason). Refresh by re-running the same cross-reference process against a newer Recipes.cs if
// the game adds new engineering blueprints.
var blueprintSymbolMap = map[string]map[string]string{
	"armour_advanced": {
		"Armour": "Lightweight",
	},
	"armour_explosive": {
		"Armour": "Blast Resistant",
	},
	"armour_heavyduty": {
		"Armour": "Heavy Duty",
	},
	"armour_kinetic": {
		"Armour": "Kinetic Resistant",
	},
	"armour_thermic": {
		"Armour": "Thermal Resistant",
	},
	"engine_dirty": {
		"Thrusters": "Dirty Drive Tuning",
	},
	"engine_reinforced": {
		"Thrusters": "Drive Strengthening",
	},
	"engine_tuned": {
		"Thrusters": "Clean Drive Tuning",
	},
	"fsd_fastboot": {
		"Frame Shift Drive": "Faster FSD Boot Sequence",
	},
	"fsd_longrange": {
		"Frame Shift Drive": "Increased FSD Range",
	},
	"fsd_shielded": {
		"Frame Shift Drive": "Shielded FSD",
	},
	"fsdinterdictor_expanded": {
		"Frame Shift Drive Interdictor": "Expanded FSD Interdictor Capture Arc",
	},
	"fsdinterdictor_longrange": {
		"Frame Shift Drive Interdictor": "Long Range FSD Interdictor",
	},
	"hullreinforcement_advanced": {
		"Hull Reinforcement Package": "Lightweight Hull Reinforcement",
	},
	"hullreinforcement_explosive": {
		"Hull Reinforcement Package": "Blast Resistant Hull Reinforcement",
	},
	"hullreinforcement_heavyduty": {
		"Hull Reinforcement Package": "Heavy Duty Hull Reinforcement",
	},
	"hullreinforcement_kinetic": {
		"Hull Reinforcement Package": "Kinetic Resistant Hull Reinforcement",
	},
	"hullreinforcement_thermic": {
		"Hull Reinforcement Package": "Thermal Resistant Hull Reinforcement",
	},
	"misc_chaffcapacity": {
		"Chaff Launcher": "Ammo Capacity",
	},
	"misc_heatsinkcapacity": {
		"Heat Sink Launcher": "Ammo Capacity",
	},
	"misc_lightweight": {
		"Chaff Launcher":                  "Lightweight",
		"Collector Limpet Controller":     "Lightweight",
		"Electronic Countermeasure":       "Lightweight",
		"Fuel Transfer Limpet Controller": "Lightweight",
		"Hatch Breaker Limpet Controller": "Lightweight",
		"Heat Sink Launcher":              "Lightweight",
		"Kill Warrant Scanner":            "Lightweight",
		"Life Support":                    "Lightweight",
		"Manifest Scanner":                "Lightweight",
		"Point Defence":                   "Lightweight",
		"Prospector Limpet Controller":    "Lightweight",
		"Wake Scanner":                    "Lightweight",
	},
	"misc_pointdefensecapacity": {
		"Point Defence": "Ammo Capacity",
	},
	"misc_reinforced": {
		"Chaff Launcher":                  "Reinforced",
		"Collector Limpet Controller":     "Reinforced",
		"Electronic Countermeasure":       "Reinforced",
		"Fuel Transfer Limpet Controller": "Reinforced",
		"Hatch Breaker Limpet Controller": "Reinforced",
		"Heat Sink Launcher":              "Reinforced",
		"Kill Warrant Scanner":            "Reinforced",
		"Life Support":                    "Reinforced",
		"Manifest Scanner":                "Reinforced",
		"Point Defence":                   "Reinforced",
		"Prospector Limpet Controller":    "Reinforced",
		"Wake Scanner":                    "Reinforced",
	},
	"misc_shielded": {
		"Auto Field-Maintenance Unit":     "Shielded",
		"Chaff Launcher":                  "Shielded",
		"Collector Limpet Controller":     "Shielded",
		"Electronic Countermeasure":       "Shielded",
		"Fuel Scoop":                      "Shielded",
		"Fuel Transfer Limpet Controller": "Shielded",
		"Hatch Breaker Limpet Controller": "Shielded",
		"Heat Sink Launcher":              "Shielded",
		"Kill Warrant Scanner":            "Shielded",
		"Life Support":                    "Shielded",
		"Manifest Scanner":                "Shielded",
		"Point Defence":                   "Shielded",
		"Prospector Limpet Controller":    "Shielded",
		"Refinery":                        "Shielded",
		"Wake Scanner":                    "Shielded",
	},
	"powerdistributor_highcapacity": {
		"Power Distributor": "High Charge Capacity",
	},
	"powerdistributor_highfrequency": {
		"Power Distributor": "Charge Enhanced",
	},
	"powerdistributor_priorityengines": {
		"Power Distributor": "Engine Focused",
	},
	"powerdistributor_prioritysystems": {
		"Power Distributor": "System Focused",
	},
	"powerdistributor_priorityweapons": {
		"Power Distributor": "Weapon Focused",
	},
	"powerdistributor_shielded": {
		"Power Distributor": "Shielded",
	},
	"powerplant_armoured": {
		"Power Plant": "Armoured",
	},
	"powerplant_boosted": {
		"Power Plant": "Overcharged",
	},
	"powerplant_stealth": {
		"Power Plant": "Low Emissions",
	},
	"sensor_expanded": {
		"Surface Scanner": "Expanded Probe Scanning Radius",
	},
	"sensor_fastscan": {
		"Kill Warrant Scanner": "Fast Scanner",
		"Manifest Scanner":     "Fast Scanner",
		"Wake Scanner":         "Fast Scanner",
	},
	"sensor_lightweight": {
		"Sensors": "Light Weight Scanner",
	},
	"sensor_longrange": {
		"Kill Warrant Scanner": "Long Range Scanner",
		"Manifest Scanner":     "Long Range Scanner",
		"Sensors":              "Long Range Scanner",
		"Wake Scanner":         "Long Range Scanner",
	},
	"sensor_wideangle": {
		"Kill Warrant Scanner": "Wide Angle Scanner",
		"Manifest Scanner":     "Wide Angle Scanner",
		"Sensors":              "Wide Angle Scanner",
		"Wake Scanner":         "Wide Angle Scanner",
	},
	"shieldbooster_explosive": {
		"Shield Booster": "Blast Resistant",
	},
	"shieldbooster_heavyduty": {
		"Shield Booster": "Heavy Duty",
	},
	"shieldbooster_kinetic": {
		"Shield Booster": "Kinetic Resistant",
	},
	"shieldbooster_resistive": {
		"Shield Booster": "Resistance Augmented",
	},
	"shieldbooster_thermic": {
		"Shield Booster": "Thermal Resistant",
	},
	"shieldcellbank_rapid": {
		"Shield Cell Bank": "Rapid Charge",
	},
	"shieldcellbank_specialised": {
		"Shield Cell Bank": "Specialised",
	},
	"shieldgenerator_kinetic": {
		"Shield Generator": "Kinetic Resistant Shields",
	},
	"shieldgenerator_optimised": {
		"Shield Generator": "Enhanced, Low Power Shields",
	},
	"shieldgenerator_reinforced": {
		"Shield Generator": "Reinforced Shields",
	},
	"shieldgenerator_thermic": {
		"Shield Generator": "Thermal Resistant Shields",
	},
	"weapon_doubleshot": {
		"Fragment Cannon": "Double Shot",
	},
	"weapon_efficient": {
		"Beam Laser":         "Efficient Weapon",
		"Burst Laser":        "Efficient Weapon",
		"Cannon":             "Efficient Weapon",
		"Fragment Cannon":    "Efficient Weapon",
		"Multi-cannon":       "Efficient Weapon",
		"Plasma Accelerator": "Efficient Weapon",
		"Pulse Laser":        "Efficient Weapon",
	},
	"weapon_focused": {
		"Burst Laser":        "Focused Weapon",
		"Plasma Accelerator": "Focused Weapon",
		"Pulse Laser":        "Focused Weapon",
	},
	"weapon_highcapacity": {
		"Cannon":          "High Capacity Magazine",
		"Fragment Cannon": "High Capacity Magazine",
		"Mine Launcher":   "High Capacity Magazine",
		"Missile Rack":    "High Capacity Magazine",
		"Multi-cannon":    "High Capacity Magazine",
		"Rail Gun":        "High Capacity Magazine",
	},
	"weapon_lightweight": {
		"Beam Laser":         "Lightweight Mount",
		"Burst Laser":        "Lightweight Mount",
		"Cannon":             "Lightweight Mount",
		"Fragment Cannon":    "Lightweight Mount",
		"Mine Launcher":      "Lightweight Mount",
		"Missile Rack":       "Lightweight Mount",
		"Multi-cannon":       "Lightweight Mount",
		"Plasma Accelerator": "Lightweight Mount",
		"Pulse Laser":        "Lightweight Mount",
		"Rail Gun":           "Lightweight Mount",
		"Torpedo Pylon":      "Lightweight Mount",
	},
	"weapon_longrange": {
		"Beam Laser":         "Long Range Weapon",
		"Burst Laser":        "Long Range Weapon",
		"Cannon":             "Long Range Weapon",
		"Multi-cannon":       "Long Range Weapon",
		"Plasma Accelerator": "Long Range Weapon",
		"Pulse Laser":        "Long Range Weapon",
		"Rail Gun":           "Long Range Weapon",
	},
	"weapon_overcharged": {
		"Beam Laser":         "Overcharged Weapon",
		"Burst Laser":        "Overcharged Weapon",
		"Cannon":             "Overcharged Weapon",
		"Fragment Cannon":    "Overcharged Weapon",
		"Multi-cannon":       "Overcharged Weapon",
		"Plasma Accelerator": "Overcharged Weapon",
		"Pulse Laser":        "Overcharged Weapon",
	},
	"weapon_rapidfire": {
		"Burst Laser":        "Rapid Fire Modification",
		"Cannon":             "Rapid Fire Modification",
		"Fragment Cannon":    "Rapid Fire Modification",
		"Mine Launcher":      "Rapid Fire Modification",
		"Missile Rack":       "Rapid Fire Modification",
		"Multi-cannon":       "Rapid Fire Modification",
		"Plasma Accelerator": "Rapid Fire Modification",
		"Pulse Laser":        "Rapid Fire Modification",
	},
	"weapon_shortrange": {
		"Beam Laser":         "Short Range Blaster",
		"Burst Laser":        "Short Range Blaster",
		"Cannon":             "Short Range Blaster",
		"Multi-cannon":       "Short Range Blaster",
		"Plasma Accelerator": "Short Range Blaster",
		"Rail Gun":           "Short Range Blaster",
	},
	"weapon_sturdy": {
		"Beam Laser":         "Sturdy Mount",
		"Burst Laser":        "Sturdy Mount",
		"Cannon":             "Sturdy Mount",
		"Fragment Cannon":    "Sturdy Mount",
		"Mine Launcher":      "Sturdy Mount",
		"Missile Rack":       "Sturdy Mount",
		"Multi-cannon":       "Sturdy Mount",
		"Plasma Accelerator": "Sturdy Mount",
		"Pulse Laser":        "Sturdy Mount",
		"Rail Gun":           "Sturdy Mount",
		"Torpedo Pylon":      "Sturdy Mount",
	},
}

// experimentalEffectSymbolMap: same idea as blueprintSymbolMap, for real journal
// ExperimentalEffect symbols against effectCatalog. Same source/method/provenance (see this
// file's header comment).
var experimentalEffectSymbolMap = map[string]map[string]string{
	"special_armour_chunky": {
		"Armour": "Deep Plating",
	},
	"special_armour_explosive": {
		"Armour": "Layered Plating",
	},
	"special_armour_kinetic": {
		"Armour": "Angled Plating",
	},
	"special_armour_thermic": {
		"Armour": "Reflective Plating",
	},
	"special_auto_loader": {
		"Cannon":       "Auto Loader",
		"Multi-cannon": "Auto Loader",
	},
	"special_blinding_shell": {
		"Fragment Cannon":    "Dazzle Shell",
		"Plasma Accelerator": "Dazzle Shell",
	},
	"special_choke_canister": {
		"Mine Launcher": "Ion Disruptor",
	},
	"special_concordant_sequence": {
		"Beam Laser":  "Concordant Sequence",
		"Burst Laser": "Concordant Sequence",
		"Pulse Laser": "Concordant Sequence",
	},
	"special_corrosive_shell": {
		"Fragment Cannon": "Corrosive Shell",
		"Multi-cannon":    "Corrosive Shell",
	},
	"special_deep_cut_payload": {
		"Torpedo Pylon": "Penetrator Payload",
	},
	"special_dispersal_field": {
		"Cannon":             "Dispersal Field",
		"Plasma Accelerator": "Dispersal Field",
	},
	"special_distortion_field": {
		"Burst Laser": "Inertial Impact",
	},
	"special_drag_munitions": {
		"Fragment Cannon": "Drag Munition",
		"Missile Rack":    "Drag Munition (Seeker only)",
	},
	"special_emissive_munitions": {
		"Mine Launcher": "Emissive Munitions",
		"Missile Rack":  "Emissive Munitions",
		"Multi-cannon":  "Emissive Munitions",
		"Pulse Laser":   "Emissive Munitions",
	},
	"special_engine_cooled": {
		"Thrusters": "Thermal Spread",
	},
	"special_engine_haulage": {
		"Thrusters": "Drive Distributors",
	},
	"special_engine_lightweight": {
		"Thrusters": "Stripped Down",
	},
	"special_engine_overloaded": {
		"Thrusters": "Drag Drives",
	},
	"special_engine_toughened": {
		"Thrusters": "Double Braced",
	},
	"special_feedback_cascade_cooled": {
		"Rail Gun": "Feedback Cascade",
	},
	"special_force_shell": {
		"Cannon": "Force Shell",
	},
	"special_fsd_cooled": {
		"Frame Shift Drive": "Thermal Spread",
	},
	"special_fsd_fuelcapacity": {
		"Frame Shift Drive": "Deep Charge",
	},
	"special_fsd_heavy": {
		"Frame Shift Drive": "Mass Manager",
	},
	"special_fsd_interrupt": {
		"Missile Rack": "FSD Interrupt (Dumbfire only)",
	},
	"special_fsd_lightweight": {
		"Frame Shift Drive": "Stripped Down",
	},
	"special_fsd_toughened": {
		"Frame Shift Drive": "Double Braced",
	},
	"special_high_yield_shell": {
		"Cannon": "High Yield Shell",
	},
	"special_hullreinforcement_chunky": {
		"Hull Reinforcement Package": "Deep Plating",
	},
	"special_hullreinforcement_explosive": {
		"Hull Reinforcement Package": "Layered Plating",
	},
	"special_hullreinforcement_kinetic": {
		"Hull Reinforcement Package": "Angled Plating",
	},
	"special_hullreinforcement_thermic": {
		"Hull Reinforcement Package": "Reflective Plating",
	},
	"special_incendiary_rounds": {
		"Fragment Cannon": "Incendiary Rounds",
		"Multi-cannon":    "Incendiary Rounds",
	},
	"special_lock_breaker": {
		"Plasma Accelerator": "Target Lock Breaker",
	},
	"special_mass_lock": {
		"Torpedo Pylon": "Mass Lock Munition",
	},
	"special_overload_munitions": {
		"Mine Launcher": "Overload Munitions",
		"Missile Rack":  "Overload Munitions",
	},
	"special_penetrator_munitions": {
		"Missile Rack": "Penetrator Munitions (Dumbfire only)",
	},
	"special_phasing_sequence": {
		"Burst Laser":        "Phasing Sequence",
		"Plasma Accelerator": "Phasing Sequence",
		"Pulse Laser":        "Phasing Sequence",
	},
	"special_plasma_slug": {
		"Plasma Accelerator": "Plasma Slug",
	},
	"special_plasma_slug_cooled": {
		"Rail Gun": "Plasma Slug",
	},
	"special_powerdistributor_capacity": {
		"Power Distributor": "Cluster Capacitor",
	},
	"special_powerdistributor_efficient": {
		"Power Distributor": "Flow Control",
	},
	"special_powerdistributor_fast": {
		"Power Distributor": "Super Conduits",
	},
	"special_powerdistributor_lightweight": {
		"Power Distributor": "Stripped Down",
	},
	"special_powerdistributor_toughened": {
		"Power Distributor": "Double Braced",
	},
	"special_powerplant_cooled": {
		"Power Plant": "Thermal Spread",
	},
	"special_powerplant_highcharge": {
		"Power Plant": "Monstered",
	},
	"special_powerplant_lightweight": {
		"Power Plant": "Stripped Down",
	},
	"special_powerplant_toughened": {
		"Power Plant": "Double Braced",
	},
	"special_radiant_canister": {
		"Mine Launcher": "Radiant Canister",
	},
	"special_regeneration_sequence": {
		"Beam Laser": "Regeneration Sequence",
	},
	"special_reverberating_cascade": {
		"Mine Launcher": "Reverberating Cascade",
		"Torpedo Pylon": "Reverberating Cascade",
	},
	"special_scramble_spectrum": {
		"Burst Laser": "Scramble Spectrum",
		"Pulse Laser": "Scramble Spectrum",
	},
	"special_screening_shell": {
		"Fragment Cannon": "Screening Shell",
	},
	"special_shield_efficient": {
		"Shield Generator": "Lo-draw",
	},
	"special_shield_health": {
		"Shield Generator": "Hi-cap",
	},
	"special_shield_kinetic": {
		"Shield Generator": "Force Block",
	},
	"special_shield_lightweight": {
		"Shield Generator": "Stripped Down",
	},
	"special_shield_regenerative": {
		"Shield Generator": "Fast Charge",
	},
	"special_shield_resistive": {
		"Shield Generator": "Multi-weave",
	},
	"special_shield_thermic": {
		"Shield Generator": "Thermo Block",
	},
	"special_shield_toughened": {
		"Shield Generator": "Double Braced",
	},
	"special_shieldbooster_chunky": {
		"Shield Booster": "Super Capacitor",
	},
	"special_shieldbooster_efficient": {
		"Shield Booster": "Flow Control",
	},
	"special_shieldbooster_explosive": {
		"Shield Booster": "Blast Block",
	},
	"special_shieldbooster_kinetic": {
		"Shield Booster": "Force Block",
	},
	"special_shieldbooster_thermic": {
		"Shield Booster": "Thermo Block",
	},
	"special_shieldbooster_toughened": {
		"Shield Booster": "Double Braced",
	},
	"special_shieldcell_efficient": {
		"Shield Cell Bank": "Flow Control",
	},
	"special_shieldcell_gradual": {
		"Shield Cell Bank": "Recycling Cells",
	},
	"special_shieldcell_lightweight": {
		"Shield Cell Bank": "Stripped Down",
	},
	"special_shieldcell_oversized": {
		"Shield Cell Bank": "Boss Cells",
	},
	"special_shieldcell_toughened": {
		"Shield Cell Bank": "Double Braced",
	},
	"special_shiftlock_canister": {
		"Mine Launcher": "Shift-Lock Canister",
	},
	"special_smart_rounds": {
		"Cannon":       "Smart Rounds",
		"Multi-cannon": "Smart Rounds",
	},
	"special_super_penetrator_cooled": {
		"Rail Gun": "Super Penetrator",
	},
	"special_thermal_cascade": {
		"Cannon":       "Thermal Cascade",
		"Missile Rack": "Thermal Cascade",
	},
	"special_thermal_conduit": {
		"Beam Laser":         "Thermal Conduit",
		"Plasma Accelerator": "Thermal Conduit",
	},
	"special_thermal_vent": {
		"Beam Laser": "Thermal Vent",
	},
	"special_thermalshock": {
		"Beam Laser":   "Thermal Shock",
		"Burst Laser":  "Thermal Shock",
		"Multi-cannon": "Thermal Shock",
		"Pulse Laser":  "Thermal Shock",
	},
	"special_weapon_damage": {
		"Beam Laser":         "Oversized",
		"Burst Laser":        "Oversized",
		"Cannon":             "Oversized",
		"Fragment Cannon":    "Oversized",
		"Mine Launcher":      "Oversized",
		"Missile Rack":       "Oversized",
		"Multi-cannon":       "Oversized",
		"Plasma Accelerator": "Oversized",
		"Pulse Laser":        "Oversized",
		"Rail Gun":           "Oversized",
		"Torpedo Pylon":      "Oversized",
	},
	"special_weapon_efficient": {
		"Beam Laser":         "Flow Control",
		"Burst Laser":        "Flow Control",
		"Cannon":             "Flow Control",
		"Fragment Cannon":    "Flow Control",
		"Mine Launcher":      "Flow Control",
		"Missile Rack":       "Flow Control",
		"Multi-cannon":       "Flow Control",
		"Plasma Accelerator": "Flow Control",
		"Pulse Laser":        "Flow Control",
		"Rail Gun":           "Flow Control",
		"Torpedo Pylon":      "Flow Control",
	},
	"special_weapon_lightweight": {
		"Beam Laser":         "Stripped Down",
		"Burst Laser":        "Stripped Down",
		"Cannon":             "Stripped Down",
		"Fragment Cannon":    "Stripped Down",
		"Mine Launcher":      "Stripped Down",
		"Missile Rack":       "Stripped Down",
		"Multi-cannon":       "Stripped Down",
		"Plasma Accelerator": "Stripped Down",
		"Pulse Laser":        "Stripped Down",
		"Rail Gun":           "Stripped Down",
		"Torpedo Pylon":      "Stripped Down",
	},
	"special_weapon_rateoffire": {
		"Burst Laser":        "Multi-Servos",
		"Cannon":             "Multi-Servos",
		"Fragment Cannon":    "Multi-Servos",
		"Missile Rack":       "Multi-Servos",
		"Multi-cannon":       "Multi-Servos",
		"Plasma Accelerator": "Multi-Servos",
		"Pulse Laser":        "Multi-Servos",
		"Rail Gun":           "Multi-Servos",
	},
	"special_weapon_toughened": {
		"Beam Laser":         "Double Braced",
		"Burst Laser":        "Double Braced",
		"Cannon":             "Double Braced",
		"Fragment Cannon":    "Double Braced",
		"Mine Launcher":      "Double Braced",
		"Missile Rack":       "Double Braced",
		"Multi-cannon":       "Double Braced",
		"Plasma Accelerator": "Double Braced",
		"Pulse Laser":        "Double Braced",
		"Rail Gun":           "Double Braced",
		"Torpedo Pylon":      "Double Braced",
	},
}

// resolveBlueprintSymbol returns the (Type, Name) blueprintCatalog is keyed by for a real
// journal BlueprintName symbol, given which module Slot it was found on to disambiguate symbols
// that apply to multiple module types (e.g. "misc_shielded" applies to over a dozen different
// module types -- the Slot tells us which one this particular module actually is). slotType is
// this file's own moduleTypeForSlot output. Returns ok=false for any symbol not yet in the map
// (see this file's header for why that's a real, expected, gracefully-handled case, not a bug).
func resolveBlueprintSymbol(symbol string, slotType string) (bpType, bpName string, ok bool) {
	candidates, found := blueprintSymbolMap[strings.ToLower(symbol)]
	if !found {
		return "", "", false
	}
	if name, ok := candidates[slotType]; ok {
		return slotType, name, true
	}
	// slotType didn't resolve (module-type prettifier gap) or didn't match any candidate exactly
	// -- if there's only one possible candidate for this symbol regardless of type, it's still
	// unambiguous.
	if len(candidates) == 1 {
		for t, name := range candidates {
			return t, name, true
		}
	}
	return "", "", false
}

func resolveExperimentalEffectSymbol(symbol string, slotType string) (name string, ok bool) {
	candidates, found := experimentalEffectSymbolMap[strings.ToLower(symbol)]
	if !found {
		return "", false
	}
	if name, ok := candidates[slotType]; ok {
		return name, true
	}
	if len(candidates) == 1 {
		for _, name := range candidates {
			return name, true
		}
	}
	return "", false
}
