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
	"math"
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
	MaxJumpRange float64                 `json:"MaxJumpRange"` // real field, present on every real Loadout event this project has seen
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
	MaxGrade      int     `json:"maxGrade,omitempty"`      // the CURRENTLY applied blueprint's real max grade, 0 if not currently engineered or its symbol didn't resolve
	BlueprintType string  `json:"blueprintType,omitempty"` // resolved catalog Type of the CURRENTLY applied blueprint, if any
	BlueprintName string  `json:"blueprintName,omitempty"` // resolved catalog Name of the CURRENTLY applied blueprint, if any
	// EngineerableType is set whenever this module's slot COULD take engineering at all (via
	// resolveModuleType on its real Item symbol), regardless of whether it currently has any --
	// what the "Engineer" modal (shipyard_template.html) needs to offer "start engineering this
	// stock module" as well as "push this already-engineered one further".
	EngineerableType string `json:"engineerableType,omitempty"`
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
	// Distance is nil when either the commander's current position or this ship's own system's
	// coordinates aren't independently known (e.g. a system named by StoredShips that's never
	// been visited/scanned with its own real StarPos on record) -- same "nil means honestly
	// unknown, don't fake a number" pattern BuildTraderData already uses for known traders.
	Distance *float64 `json:"distanceLy,omitempty"`
	// MaxJumpRange only exists when a real Loadout was recovered for this ship (0/omitted for a
	// stored ship whose build wasn't independently confirmed) -- owner: "list some more info about
	// ships, like jump range or something".
	MaxJumpRange float64 `json:"maxJumpRange,omitempty"`
	// ShipSymbol/RawLoadoutJSON: see buildShipFromLoadout's comment -- power the Export to
	// Coriolis/EDSY buttons and the paste-a-build-link import. Both omitted (zero value) for a
	// stored ship whose build wasn't independently recovered -- there's no real Loadout JSON to
	// export in that case.
	ShipSymbol     string `json:"shipSymbol,omitempty"`
	RawLoadoutJSON string `json:"rawLoadoutJson,omitempty"`
}

type shipyardData struct {
	GeneratedAt      string                    `json:"generatedAt"`
	Ships            []shipOut                 `json:"ships"`
	SnapshotAt       string                    `json:"snapshotAt,omitempty"` // the StoredShips event this fleet listing is based on
	HasFleetData     bool                      `json:"hasFleetData"`
	CurrentSystem    string                    `json:"currentSystem,omitempty"` // for an honest "distances from X" label, same as traderData
	EffectsByType    map[string][]string       `json:"effectsByType"`           // catalog Type -> real Experimental Effect names for that Type, for the "Engineer" modal's effect picker
	BlueprintsByType map[string][]blueprintOpt `json:"blueprintsByType"`        // catalog Type -> every real blueprint Name (+ real max grade) for that Type, for the modal's Name/Grade pickers -- lets it offer "start engineering this stock module", not just "push an already-started one further"
	// Blueprints/Effects/HeldMaterials power a self-contained "Materials Needed" panel on this
	// page itself -- real owner report: "pressing add doesnt add it to the engineering planner i
	// guess because its not on the same page" (confirmed real: this project already learned,
	// building the theme toggle earlier this session, that separate file:// pages can't reliably
	// share localStorage -- the exact same limitation applies here, and the shared-basket
	// cross-page handoff was never reliable in the first place). Rather than depend on that again,
	// the shipyard page now tracks its own plan and computes materials needed without ever
	// leaving the page -- the owner's own suggested fix ("open a side window with materials
	// needed if you cant"). Per-grade ingredient data (needed to sum a range of grades, not just
	// look one up) isn't otherwise exposed to this page, so it's embedded here the same way
	// engineeringplanner.go already does for its own page.
	Blueprints    []plannerBlueprintOut `json:"blueprints"`
	Effects       []plannerEffectOut    `json:"effects"`
	HeldMaterials map[string]int64      `json:"heldMaterials"`
	// Trading feature-matches this page's Materials Needed panel with the engineering planner's
	// own trade calculator (user: "materials needed needs to be feature matched with engineering
	// planner, have stuff for trades and all that") -- same traderData BuildTraderData already
	// gives materials.go/engineeringplanner.go, just also embedded here.
	Trading traderData `json:"trading"`
	// ModuleTypeKeywords/NonEngineerableItemPrefixes/EngineItemInfix: this project generates fully
	// static HTML with no live backend, so a pasted Coriolis/EDSY import (real Item symbols this
	// project has never necessarily seen before, e.g. a community build using different weapons)
	// needs the SAME resolveModuleType algorithm (shipmodulenames.go) available client-side too --
	// exporting the same single source-of-truth data rather than hand-maintaining a second copy in
	// JS that could drift out of sync.
	ModuleTypeKeywords          map[string]string `json:"moduleTypeKeywords"`
	NonEngineerableItemPrefixes []string          `json:"nonEngineerableItemPrefixes"`
	EngineItemInfix             string            `json:"engineItemInfix"`
	// BlueprintSymbolMap/ExperimentalEffectSymbolMap: real bug caught during import verification --
	// a pasted build's raw Engineering.BlueprintName/ExperimentalEffect are the game's own internal
	// symbols (e.g. "FSD_LongRange", "special_armour_chunky"), NOT this project's own display names
	// ("Increased FSD Range", "Deep Plating") that DATA.blueprintsByType/effectsByType use --
	// confirmed directly against this commander's own real Loadout JSON, which a naive display-name
	// string comparison would have silently failed to match on every single real module. Exporting
	// the SAME symbol->name maps buildModuleOut already resolves through server-side
	// (resolveBlueprintSymbol/resolveExperimentalEffectSymbol in shipmodulemaps.go) so the import
	// path uses the identical real resolution, not a second guess.
	BlueprintSymbolMap          map[string]map[string]string `json:"blueprintSymbolMap"`
	ExperimentalEffectSymbolMap map[string]map[string]string `json:"experimentalEffectSymbolMap"`
}

type blueprintOpt struct {
	Name     string `json:"name"`
	MaxGrade int    `json:"maxGrade"`
}

// effectsByType groups the existing vendored effectCatalog (engineeringplanner.go) by Type,
// exactly the same real data the engineering planner's own picker already offers -- just
// reshaped for the shipyard page's per-ship "Engineer" modal, which needs to scope the effect
// choices to whichever module Type is being engineered.
func effectsByType() map[string][]string {
	out := map[string][]string{}
	for _, ef := range effectCatalog {
		out[ef.Type] = append(out[ef.Type], ef.Name)
	}
	return out
}

// blueprintsByType groups the existing vendored blueprintCatalog by Type, collapsing each
// (Type, Name) group's real grade rows down to just that blueprint's real max grade (155 of 160
// real blueprint groups cap at 5, a handful of utility upgrades cap lower -- see
// docs/ShipyardPlanner.md, already relied on by maxGradeFor).
func blueprintsByType() map[string][]blueprintOpt {
	seen := map[string]map[string]int{} // Type -> Name -> max grade seen so far
	for _, b := range blueprintCatalog {
		if seen[b.Type] == nil {
			seen[b.Type] = map[string]int{}
		}
		if b.Grade > seen[b.Type][b.Name] {
			seen[b.Type][b.Name] = b.Grade
		}
	}
	out := map[string][]blueprintOpt{}
	for t, names := range seen {
		for n, maxGrade := range names {
			out[t] = append(out[t], blueprintOpt{Name: n, MaxGrade: maxGrade})
		}
	}
	return out
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
	slotType := resolveModuleType(m.Item)
	out.EngineerableType = slotType // set regardless of current engineering state -- lets the "Engineer" modal offer stock modules too, not just ones already started
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

// trimmedShipName: a real, confirmed journal quirk -- an unnamed ship's ShipName sometimes
// arrives as a single space character rather than a genuinely empty string (confirmed against
// this commander's own real data, ShipIdent "PE-24P"). Left un-trimmed, that space is truthy in
// the client's own JS ("s.name ? ... : ..."), so the ship card showed a stray leading blank
// before the ident badge instead of falling back to the ship type -- a real owner-reported bug
// ("name rendering in the shipyard is weird"). Trimmed once here, at the one place ShipName ever
// enters shipOut, rather than in the template -- keeps every consumer honest by construction.
func trimmedShipName(raw string) string {
	return strings.TrimSpace(raw)
}

func buildShipFromLoadout(v shipyardLoadoutEvent, active bool, system string, localCaptured map[string]string, rawJSON string) shipOut {
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
		ShipID: v.ShipID, Type: shipTypeName(v.Ship, localCaptured), Name: trimmedShipName(v.ShipName), Ident: v.ShipIdent,
		Active: active, System: system, Value: v.HullValue + v.ModulesValue,
		HullHealth: v.HullHealth, Modules: modules, HasModules: true,
		EngineeredCount: engineeredCount, MaxJumpRange: v.MaxJumpRange,
		// ShipSymbol/RawLoadoutJSON power the export/import buttons (user: "have export ship to
		// edsy / coriolis buttons ... paste a cori/edsy link and press confirm and itll update
		// goal ship to that"). The raw journal Loadout JSON is exactly what Coriolis's own real
		// `shipFromLoadoutJSON` importer and EDSY's own real "Journal JSONL...Loadout" importer
		// each already know how to parse natively -- verified directly against both projects' own
		// real open-source code (EDCD/coriolis, taleden/EDSY), not guessed. ShipSymbol (the raw
		// game symbol, e.g. "python_nx") is kept separately from the resolved display Type so a
		// pasted import can be validated against the SAME real symbol, not a display name that
		// could coincidentally collide/differ.
		ShipSymbol: v.Ship, RawLoadoutJSON: rawJSON,
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
func latestLoadoutFor(store *Store, shipID int, atOrBefore string) (shipyardLoadoutEvent, string, string, bool) {
	var best shipyardLoadoutEvent
	var bestTS, bestRaw string
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
		best, bestTS, bestRaw, found = v, e.Timestamp, e.Raw, true
	}
	return best, bestTS, bestRaw, found
}

func BuildShipyardData(store *Store) shipyardData {
	// Per-grade ingredient data for the self-contained materials-needed panel -- same shape and
	// resolution as engineeringplanner.go's own BuildPlannerData, engineer status omitted (not
	// needed here, this page already shows real per-module engineer/grade state directly from
	// the journal on each ship card).
	blueprints := make([]plannerBlueprintOut, 0, len(blueprintCatalog))
	for _, bp := range blueprintCatalog {
		blueprints = append(blueprints, plannerBlueprintOut{
			Type: bp.Type, Name: bp.Name, Grade: bp.Grade,
			Ingredients: resolveIngredients(bp.Ingredients),
		})
	}
	effects := make([]plannerEffectOut, 0, len(effectCatalog))
	for _, ef := range effectCatalog {
		effects = append(effects, plannerEffectOut{
			Type: ef.Type, Name: ef.Name,
			Ingredients: resolveIngredients(ef.Ingredients),
		})
	}
	held := map[string]int64{}
	if snapshot, _, ok := latestMaterialsSnapshot(store); ok {
		for _, cat := range [][]materialEntry{snapshot.Raw, snapshot.Manufactured, snapshot.Encoded} {
			for _, m := range cat {
				held[normalizeMaterialKey(m.Name)] += m.Count
			}
		}
	}

	data := shipyardData{
		GeneratedAt:                 time.Now().UTC().Format("2006-01-02 15:04 MST"),
		EffectsByType:               effectsByType(),
		BlueprintsByType:            blueprintsByType(),
		Blueprints:                  blueprints,
		Effects:                     effects,
		HeldMaterials:               held,
		Trading:                     BuildTraderData(store),
		ModuleTypeKeywords:          moduleTypeKeywords,
		NonEngineerableItemPrefixes: nonEngineerableItemPrefixes,
		EngineItemInfix:             engineItemInfix,
		BlueprintSymbolMap:          blueprintSymbolMap,
		ExperimentalEffectSymbolMap: experimentalEffectSymbolMap,
	}

	// Active ship: the true latest real Loadout, full detail, no staleness concern -- this is
	// always exactly current, since the game only sends Loadout for the ship you're actually in.
	var trueLatestLoadout shipyardLoadoutEvent
	var trueLatestTS, trueLatestRaw string
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
		trueLatestLoadout, trueLatestTS, trueLatestRaw, haveAnyLoadout = v, e.Timestamp, e.Raw, true
	}
	if !haveAnyLoadout {
		return data
	}

	localCaptured := captureRealShipTypeNames(store)
	activeSystem, curX, curY, curZ, haveCurrentPos := currentPosition(store)
	data.CurrentSystem = activeSystem
	ships := []shipOut{buildShipFromLoadout(trueLatestLoadout, true, activeSystem, localCaptured, trueLatestRaw)}

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
				// StoredShips carries its own real ShipType_Localised directly -- preferred over
				// the general shipTypeName resolver when present, since it's the most current
				// real name for exactly this stored ship (real bug fixed here: this field was
				// previously computed and then silently never used at all).
				name := s.ShipTypeLoc
				if name == "" {
					name = shipTypeName(s.ShipType, localCaptured)
				}
				out := shipOut{ShipID: s.ShipID, Type: name, Name: trimmedShipName(s.Name), System: sys, Value: s.Value}
				// Recover this stored ship's last-known build -- only trust it if the ShipType
				// the recovered Loadout itself reports still matches what StoredShips says is
				// there NOW, guarding against the real, confirmed ShipID-reuse gotcha (see this
				// file's header).
				if lo, ts, raw, ok := latestLoadoutFor(store, s.ShipID, latestStoredTS); ok && strings.EqualFold(lo.Ship, s.ShipType) {
					built := buildShipFromLoadout(lo, false, sys, localCaptured, raw)
					built.Type = name     // StoredShips' own real name (or the resolver) wins over Loadout's raw symbol
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

	// Distance from your current position -- owner: "distance from you" per ship. Same
	// coordinate-lookup + math.Sqrt approach BuildTraderData already uses for known material
	// traders, just applied per-ship instead of per-trader; the active ship is always exactly 0
	// (you're standing in it). Left nil (not 0) when a stored ship's system was never
	// independently visited/scanned, so the client can honestly show "distance unknown" instead
	// of a fabricated zero.
	if haveCurrentPos {
		for i := range ships {
			if ships[i].System == "" {
				continue
			}
			if ships[i].System == activeSystem {
				d := 0.0
				ships[i].Distance = &d
				continue
			}
			for _, sys := range store.Systems {
				if sys.Name != ships[i].System {
					continue
				}
				d := math.Sqrt(math.Pow(sys.X-curX, 2) + math.Pow(sys.Y-curY, 2) + math.Pow(sys.Z-curZ, 2))
				ships[i].Distance = &d
				break
			}
		}
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
