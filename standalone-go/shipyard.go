package main

// shipyard.go builds a 6th page: the real fleet, real fitted modules, and an honest per-ship
// engineering completion percentage. See docs/ShipyardPlanner.md for the full design discussion
// (including why "fork Coriolis" doesn't work as literally asked, and how the scope settled on
// this instead) -- summary: Loadout gives full real module detail (including Engineering) for
// whichever ship is currently ACTIVE; StoredShips is a periodic full snapshot of every OTHER
// ship's type/name/value/location (never its modules -- the journal only ever reports a
// non-active ship's Loadout while it WAS active, in the past). Same "ground truth snapshot, not
// net-accumulated" pattern already used for Materials/EngineerProgress this session.
//
// Real, confirmed gotcha: ShipID is a slot number reused after a ship is sold (confirmed directly
// in this commander's own data: ShipID 8 shows up as "asp" in some Loadout events and later as
// "python_nx" in others) -- so recovering a stored ship's last-known modules by searching
// backward for its most recent Loadout must also verify the recovered Loadout's own Ship (type)
// field matches, or the ID was recycled onto a different hull since and the stored ship's build
// should be shown as "not recorded" rather than silently wrong.

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

//go:embed shipyard_template.html
var shipyardTemplate string

type shipyardLoadoutModuleEng struct {
	Engineer                    string  `json:"Engineer"`
	BlueprintName               string  `json:"BlueprintName"`
	Level                       int     `json:"Level"`
	Quality                     float64 `json:"Quality"`
	ExperimentalEffect          string  `json:"ExperimentalEffect"`
	ExperimentalEffectLocalised string  `json:"ExperimentalEffect_Localised"`
}

type shipyardLoadoutModule struct {
	Item        string                    `json:"Item"`
	Slot        string                    `json:"Slot"`
	On          bool                      `json:"On"`
	Health      float64                   `json:"Health"`
	Value       int64                     `json:"Value"`
	Engineering *shipyardLoadoutModuleEng `json:"Engineering"`
}

type shipyardLoadoutEvent struct {
	Ship         string                  `json:"Ship"`
	ShipID       int                     `json:"ShipID"`
	ShipName     string                  `json:"ShipName"`
	ShipIdent    string                  `json:"ShipIdent"`
	HullValue    int64                   `json:"HullValue"`
	ModulesValue int64                   `json:"ModulesValue"`
	HullHealth   float64                 `json:"HullHealth"`
	Modules      []shipyardLoadoutModule `json:"Modules"`
}

type storedShipRemote struct {
	ShipID      int    `json:"ShipID"`
	ShipType    string `json:"ShipType"`
	ShipTypeLoc string `json:"ShipType_Localised"`
	Name        string `json:"Name"`
	StarSystem  string `json:"StarSystem"`
	Value       int64  `json:"Value"`
	Hot         bool   `json:"Hot"`
}

type storedShipsEvent struct {
	ShipsHere   []storedShipRemote `json:"ShipsHere"`
	ShipsRemote []storedShipRemote `json:"ShipsRemote"`
	StarSystem  string             `json:"StarSystem"` // present when the ship's own docked system needs inferring for ShipsHere entries, which don't carry their own StarSystem
}

type moduleOut struct {
	Slot          string  `json:"slot"`
	Name          string  `json:"name"`
	On            bool    `json:"on"`
	Health        float64 `json:"health"`
	Engineered    bool    `json:"engineered"`
	Engineer      string  `json:"engineer,omitempty"`
	Grade         int     `json:"grade,omitempty"`
	Quality       float64 `json:"quality,omitempty"`
	Effect        string  `json:"effect,omitempty"`
	MaxGrade      int     `json:"maxGrade,omitempty"`      // 0 if this module's blueprint symbol didn't resolve -- no "push to next grade" possible
	BlueprintType string  `json:"blueprintType,omitempty"` // resolved catalog Type, only set when MaxGrade > 0 -- what the client needs to build a basket entry
	BlueprintName string  `json:"blueprintName,omitempty"`
}

type shipOut struct {
	ShipID          int         `json:"shipId"`
	Type            string      `json:"type"`
	Name            string      `json:"name,omitempty"`
	Ident           string      `json:"ident,omitempty"`
	Active          bool        `json:"active"`
	System          string      `json:"system,omitempty"`
	Value           int64       `json:"value"`
	HullHealth      float64     `json:"hullHealth,omitempty"`
	Modules         []moduleOut `json:"modules,omitempty"` // empty if this stored ship's build was never recovered
	HasModules      bool        `json:"hasModules"`
	CompletionPct   float64     `json:"completionPct,omitempty"` // -1 (omitted, i.e. zero value) when HasModules is false or no engineered modules exist to measure
	EngineeredCount int         `json:"engineeredCount"`
	LoadoutAt       string      `json:"loadoutAt,omitempty"` // real timestamp of the Loadout this build was recovered from -- honesty about how stale it might be
}

type shipyardData struct {
	GeneratedAt  string    `json:"generatedAt"`
	Ships        []shipOut `json:"ships"`
	SnapshotAt   string    `json:"snapshotAt,omitempty"` // the StoredShips event this fleet listing is based on
	HasFleetData bool      `json:"hasFleetData"`
}

// engineeredModuleOut/effectiveGrade helpers: resolve a fitted module's real engineering state
// (already on the journal, no mapping needed) plus, when its blueprint symbol resolves via
// shipmodulemaps.go, how far from max grade it is.
func buildModuleOut(m shipyardLoadoutModule) moduleOut {
	out := moduleOut{
		Slot:   m.Slot,
		Name:   prettifyModuleItem(m.Item),
		On:     m.On,
		Health: m.Health,
	}
	if m.Engineering == nil {
		return out
	}
	out.Engineered = true
	out.Engineer = m.Engineering.Engineer
	out.Grade = m.Engineering.Level
	out.Quality = m.Engineering.Quality
	if m.Engineering.ExperimentalEffectLocalised != "" {
		out.Effect = m.Engineering.ExperimentalEffectLocalised
	} else if m.Engineering.ExperimentalEffect != "" {
		out.Effect = prettifyKeyStandalone(strings.ReplaceAll(m.Engineering.ExperimentalEffect, "_", " "))
	}

	slotType := resolveModuleType(m.Item)
	bpType, bpName, ok := resolveBlueprintSymbol(m.Engineering.BlueprintName, slotType)
	if !ok {
		return out
	}
	out.BlueprintType = bpType
	out.BlueprintName = bpName
	out.MaxGrade = maxGradeFor(bpType, bpName)
	return out
}

// maxGradeFor looks up the real max grade a (Type, Name) blueprint goes to in this project's own
// vendored catalog -- almost always 5, but a handful of utility upgrades cap lower (checked
// directly against the vendored data, see docs/ShipyardPlanner.md), so this doesn't just assume.
func maxGradeFor(bpType, bpName string) int {
	max := 0
	for _, b := range blueprintCatalog {
		if b.Type == bpType && b.Name == bpName && b.Grade > max {
			max = b.Grade
		}
	}
	if max == 0 {
		return 5 // symbol resolved but somehow no grade rows matched -- shouldn't happen given resolveBlueprintSymbol's own verification, but never claim "no max" over a safe default
	}
	return max
}

func buildShipFromLoadout(v shipyardLoadoutEvent, active bool, system string) shipOut {
	modules := make([]moduleOut, 0, len(v.Modules))
	engineeredCount := 0
	gradeSum, maxSum := 0, 0
	for _, m := range v.Modules {
		mo := buildModuleOut(m)
		modules = append(modules, mo)
		if mo.Engineered {
			engineeredCount++
			if mo.MaxGrade > 0 {
				gradeSum += mo.Grade
				maxSum += mo.MaxGrade
			}
		}
	}
	out := shipOut{
		ShipID: v.ShipID, Type: v.Ship, Name: v.ShipName, Ident: v.ShipIdent,
		Active: active, System: system, Value: v.HullValue + v.ModulesValue,
		HullHealth: v.HullHealth, Modules: modules, HasModules: true,
		EngineeredCount: engineeredCount,
	}
	if maxSum > 0 {
		out.CompletionPct = float64(gradeSum) / float64(maxSum) * 100
	}
	return out
}

// latestLoadoutByShipID indexes every real Loadout event this commander has ever generated by
// ShipID, keeping the most recent one seen up to (and including) atOrBefore -- used both for the
// active ship (atOrBefore = "" meaning no cutoff, i.e. the true latest) and for recovering a
// stored ship's last-known build as of a specific StoredShips snapshot (see this file's header
// comment on why the cutoff and the type-match guard both matter).
func latestLoadoutFor(store *Store, shipID int, atOrBefore string) (shipyardLoadoutEvent, string, bool) {
	var best shipyardLoadoutEvent
	var bestTS string
	found := false
	for _, e := range store.RawEvents {
		if e.Event != "Loadout" {
			continue
		}
		if atOrBefore != "" && e.Timestamp > atOrBefore {
			continue
		}
		var v shipyardLoadoutEvent
		if json.Unmarshal([]byte(e.Raw), &v) != nil || v.ShipID != shipID {
			continue
		}
		if found && e.Timestamp <= bestTS {
			continue
		}
		best, bestTS, found = v, e.Timestamp, true
	}
	return best, bestTS, found
}

func BuildShipyardData(store *Store) shipyardData {
	data := shipyardData{GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04 MST")}

	// Active ship: the true latest real Loadout, full detail, no staleness concern -- this is
	// always exactly current, since the game only sends Loadout for the ship you're actually in.
	var trueLatestLoadout shipyardLoadoutEvent
	var trueLatestTS string
	haveAnyLoadout := false
	for _, e := range store.RawEvents {
		if e.Event != "Loadout" {
			continue
		}
		if haveAnyLoadout && e.Timestamp <= trueLatestTS {
			continue
		}
		var v shipyardLoadoutEvent
		if json.Unmarshal([]byte(e.Raw), &v) != nil {
			continue
		}
		trueLatestLoadout, trueLatestTS, haveAnyLoadout = v, e.Timestamp, true
	}
	if !haveAnyLoadout {
		return data
	}

	activeSystem, _, _, _, _ := currentPosition(store)
	ships := []shipOut{buildShipFromLoadout(trueLatestLoadout, true, activeSystem)}

	// Stored fleet: the most recent StoredShips snapshot -- ShipsHere (docked at whatever station
	// that snapshot was taken at) + ShipsRemote (everywhere else, each with its own StarSystem).
	var latestStored storedShipsEvent
	var latestStoredTS string
	var latestStoredSystem string
	haveStored := false
	for _, e := range store.RawEvents {
		if e.Event != "StoredShips" {
			continue
		}
		if haveStored && e.Timestamp <= latestStoredTS {
			continue
		}
		var v storedShipsEvent
		if json.Unmarshal([]byte(e.Raw), &v) != nil {
			continue
		}
		latestStored, latestStoredTS, haveStored = v, e.Timestamp, true
		latestStoredSystem = e.SystemName // RawEvent's own best-effort system context, for ShipsHere entries
	}

	if haveStored {
		data.SnapshotAt = latestStoredTS
		addStored := func(entries []storedShipRemote, fallbackSystem string) {
			for _, s := range entries {
				if s.ShipID == trueLatestLoadout.ShipID {
					continue // this is the active ship -- StoredShips never lists it, but guard anyway
				}
				sys := s.StarSystem
				if sys == "" {
					sys = fallbackSystem
				}
				name := s.ShipTypeLoc
				if name == "" {
					name = prettifyKeyStandalone(s.ShipType)
				}
				out := shipOut{ShipID: s.ShipID, Type: s.ShipType, Name: s.Name, System: sys, Value: s.Value}
				// Recover this stored ship's last-known build -- only trust it if the ShipType
				// the recovered Loadout itself reports still matches what StoredShips says is
				// there NOW, guarding against the real, confirmed ShipID-reuse gotcha (see this
				// file's header).
				if lo, ts, ok := latestLoadoutFor(store, s.ShipID, latestStoredTS); ok && strings.EqualFold(lo.Ship, s.ShipType) {
					built := buildShipFromLoadout(lo, false, sys)
					built.Value = s.Value // StoredShips' own Value is more current than a possibly-stale Loadout's
					built.LoadoutAt = ts
					ships = append(ships, built)
				} else {
					out.HasModules = false
					ships = append(ships, out)
				}
			}
		}
		addStored(latestStored.ShipsHere, latestStoredSystem)
		addStored(latestStored.ShipsRemote, "")
	}

	sort.SliceStable(ships, func(i, j int) bool {
		if ships[i].Active != ships[j].Active {
			return ships[i].Active
		}
		return ships[i].Type < ships[j].Type
	})

	data.Ships = ships
	data.HasFleetData = true
	return data
}

func RenderShipyard(store *Store) (string, int, error) {
	data := BuildShipyardData(store)
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", 0, err
	}
	escaped := strings.ReplaceAll(string(dataJSON), "</", "<\\/")
	html := strings.Replace(shipyardTemplate, "__DATA_JSON__", escaped, 1)
	return html, len(data.Ships), nil
}
