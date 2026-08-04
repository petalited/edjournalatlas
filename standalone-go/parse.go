package main

// Journal event parsing -- ported from journal_parse.py's event handlers. Same real-journal
// field grounding as the Python version (see docs/StandaloneJournalParser.md): almost every
// mechanic here is a direct field read, not something requiring independent state-machine
// invention. Comments below only repeat the non-obvious parts (units, real field-name
// quirks); see the Python version's own comments for the fuller "why" on each one.
//
// IMPORTANT real bug found and fixed here: a single shared struct across all event types is
// unsafe -- Elite Dangerous's journal reuses field NAMES across incompatible event types with
// different value TYPES (e.g. ScanOrganic's "Body" is a number, but Location/Screenshot's
// "Body" is a string). Go's json.Unmarshal fails the ENTIRE line if any field's JSON type
// doesn't match its Go struct type, unlike Python's dict.get() which just ignores irrelevant
// keys per event -- so a single mega-struct silently dropped every Location event (and others)
// that happened to carry a colliding field, which in turn dropped whole systems whose only
// system-establishing event in this journal history was one of those lines. Confirmed directly:
// 1955 lines failed to unmarshal with exactly this error before the fix, and 142 real systems
// were missing from the output as a direct result. Fixed by giving each event type its own
// dedicated struct with only the fields that event actually has, so there's no possibility of
// a cross-event field collision.

import (
	"encoding/json"
)

type eventHead struct {
	Event     string `json:"event"`
	Timestamp string `json:"timestamp"`
}

// genericHead pulls the handful of fields common enough across many otherwise-unrelated event
// types (confirmed against real data: FSDJump/Location/Scan/ScanBaryCentre/CarrierLocation all
// carry StarSystem+BodyID directly) to give every captured raw event best-effort system/body
// context without needing a bespoke struct per event type.
type genericHead struct {
	StarSystem string `json:"StarSystem"`
	BodyID     *int   `json:"BodyID"`
}

// Deliberately no FID field -- Frontier's own persistent account identifier, not anything about
// what happened in the game. Never parsed, let alone stored: see recordRawEvent's own comment for
// why the raw capture strips it too, confirmed present on real Commander/LoadGame events.
type commanderEntry struct {
	Name      string `json:"Name"`
	Commander string `json:"Commander"`
}

type jumpEntry struct {
	StarSystem    string      `json:"StarSystem"`
	SystemAddress *int64      `json:"SystemAddress"`
	StarPos       *[3]float64 `json:"StarPos"`
	Population    *int64      `json:"Population"`
	SystemFaction *struct {
		Name string `json:"Name"`
	} `json:"SystemFaction"`
	SystemGovernmentLocalised string `json:"SystemGovernment_Localised"`
	SystemAllegiance          string `json:"SystemAllegiance"`
	SystemSecurityLocalised   string `json:"SystemSecurity_Localised"`
}

type codexEntryEvent struct {
	SystemAddress   *int64 `json:"SystemAddress"`
	RegionLocalised string `json:"Region_Localised"`
}

type fssDiscoveryEntry struct {
	SystemAddress *int64   `json:"SystemAddress"`
	SystemName    string   `json:"SystemName"`
	BodyCount     *int     `json:"BodyCount"`
	Progress      *float64 `json:"Progress"`
}

type colonisationClaimEntry struct {
	SystemAddress *int64 `json:"SystemAddress"`
	StarSystem    string `json:"StarSystem"`
}

type scanEntry struct {
	SystemAddress         *int64           `json:"SystemAddress"`
	BodyID                *int             `json:"BodyID"`
	BodyName              string           `json:"BodyName"`
	StarType              string           `json:"StarType"`
	Subclass              *int             `json:"Subclass"`
	StellarMass           *float64         `json:"StellarMass"`
	Luminosity            string           `json:"Luminosity"`
	DistanceFromArrivalLS *float64         `json:"DistanceFromArrivalLS"`
	PlanetClass           string           `json:"PlanetClass"`
	Landable              *bool            `json:"Landable"`
	MassEM                *float64         `json:"MassEM"`
	TerraformState        string           `json:"TerraformState"`
	AtmosphereType        string           `json:"AtmosphereType"`
	SurfaceGravity        *float64         `json:"SurfaceGravity"`
	SurfaceTemperature    *float64         `json:"SurfaceTemperature"`
	SurfacePressure       *float64         `json:"SurfacePressure"`
	WasDiscovered         *bool            `json:"WasDiscovered"`
	WasMapped             *bool            `json:"WasMapped"`
	WasFootfalled         *bool            `json:"WasFootfalled"`
	Parents               []map[string]int `json:"Parents"`
}

type saaScanCompleteEntry struct {
	SystemAddress    *int64 `json:"SystemAddress"`
	BodyID           *int   `json:"BodyID"`
	ProbesUsed       *int   `json:"ProbesUsed"`
	EfficiencyTarget *int   `json:"EfficiencyTarget"`
	WasMapped        *bool  `json:"WasMapped"`
}

type signalsEntry struct {
	SystemAddress *int64 `json:"SystemAddress"`
	BodyID        *int   `json:"BodyID"`
	Signals       []struct {
		Type  string `json:"Type"`
		Count int    `json:"Count"`
	} `json:"Signals"`
}

type scanOrganicEntry struct {
	SystemAddress *int64 `json:"SystemAddress"`
	Body          *int   `json:"Body"` // NOT BodyID -- real FDev schema quirk, confirmed
	Genus         string `json:"Genus"`
	Species       string `json:"Species"`
	Variant       string `json:"Variant"`
	ScanType      string `json:"ScanType"`
	WasLogged     *bool  `json:"WasLogged"`
}

type multiSellEntry struct {
	Discovered []struct {
		SystemName string `json:"SystemName"`
	} `json:"Discovered"`
	BaseValue     *int64 `json:"BaseValue"`
	Bonus         *int64 `json:"Bonus"`
	TotalEarnings *int64 `json:"TotalEarnings"`
}

type resurrectEntry struct {
	Option string `json:"Option"`
}

type Parser struct {
	store                *Store
	currentSystemAddress int64
	currentSystemName    string
}

func NewParser(store *Store) *Parser {
	return &Parser{store: store}
}

func (p *Parser) ProcessLine(line []byte) {
	var head eventHead
	if err := json.Unmarshal(line, &head); err != nil {
		return
	}
	p.recordRawEvent(head, line)
	switch head.Event {
	case "Commander", "LoadGame":
		var e commanderEntry
		if json.Unmarshal(line, &e) == nil {
			p.onCommander(&e)
		}
	case "FSDJump", "Location", "CarrierJump":
		var e jumpEntry
		if json.Unmarshal(line, &e) == nil {
			p.onJumpOrLocation(&e, head.Timestamp)
		}
	case "FSSDiscoveryScan":
		var e fssDiscoveryEntry
		if json.Unmarshal(line, &e) == nil {
			p.onFSSDiscoveryScan(&e, head.Timestamp)
		}
	case "ColonisationSystemClaim":
		var e colonisationClaimEntry
		if json.Unmarshal(line, &e) == nil {
			p.onColonisationClaim(&e, head.Timestamp)
		}
	case "CodexEntry":
		var e codexEntryEvent
		if json.Unmarshal(line, &e) == nil {
			p.onCodexEntry(&e)
		}
	case "Scan":
		var e scanEntry
		if json.Unmarshal(line, &e) == nil {
			p.onScan(&e, head.Timestamp)
		}
	case "SAAScanComplete":
		var e saaScanCompleteEntry
		if json.Unmarshal(line, &e) == nil {
			p.onSAAScanComplete(&e, head.Timestamp)
		}
	case "FSSBodySignals", "SAASignalsFound":
		var e signalsEntry
		if json.Unmarshal(line, &e) == nil {
			p.onSignals(&e, head.Timestamp)
		}
	case "ScanOrganic":
		var e scanOrganicEntry
		if json.Unmarshal(line, &e) == nil {
			p.onScanOrganic(&e, head.Timestamp)
		}
	case "SellOrganicData":
		p.onSellOrganicData(head.Timestamp)
	case "MultiSellExplorationData":
		var e multiSellEntry
		if json.Unmarshal(line, &e) == nil {
			p.onMultiSellExplorationData(&e, head.Timestamp)
		}
	case "Died":
		p.onDied(head.Timestamp)
	case "Resurrect":
		var e resurrectEntry
		if json.Unmarshal(line, &e) == nil {
			p.onResurrect(&e, head.Timestamp)
		}
	}
}

// recordRawEvent captures every valid journal line unconditionally, regardless of whether any
// case above recognizes its event type -- the foundation for "the whole journal, searchable",
// not just the exploration-relevant subset every other part of this program cares about.
// Only these event types are ever confirmed to carry a sensitive field (see scrubSensitiveFields)
// -- checked against a real, independent commander's full journal (262,840 events, every distinct
// field name across all of them), not guessed. Skipping the scrub check entirely for the other
// 99%+ of events avoids parsing/re-serializing every single captured line just to look for a
// field that's essentially never there.
var eventsNeedingScrub = map[string]bool{"Commander": true, "LoadGame": true, "Fileheader": true}

// FID (Frontier's own persistent account identifier) and language (the player's local UI/system
// language) describe the player's account or computer, not anything that happened in the game --
// out of scope for a tool whose whole point is "what did I do in Elite Dangerous," and exactly
// the kind of thing that shouldn't end up in a .db file someone might hand to someone else (this
// is a real scenario, not hypothetical: a third-party tester's real .db was shared for
// troubleshooting a different bug and both fields were sitting in it). The in-game commander NAME
// is kept -- that's the "ign" this capture is meant to hold, just not the account ID or locale
// alongside it.
var scrubbedRawFields = []string{"FID", "language"}

func scrubSensitiveFields(line []byte) []byte {
	var generic map[string]json.RawMessage
	if json.Unmarshal(line, &generic) != nil {
		return line // best-effort -- if it doesn't even parse as an object, nothing to scrub
	}
	changed := false
	for _, field := range scrubbedRawFields {
		if _, ok := generic[field]; ok {
			delete(generic, field)
			changed = true
		}
	}
	if !changed {
		return line
	}
	scrubbed, err := json.Marshal(generic)
	if err != nil {
		return line
	}
	return scrubbed
}

func (p *Parser) recordRawEvent(head eventHead, line []byte) {
	var g genericHead
	json.Unmarshal(line, &g) // best-effort -- a decode failure here just means less context, not a dropped event
	systemName := g.StarSystem
	if systemName == "" {
		systemName = p.currentSystemName
	}
	raw := line
	if eventsNeedingScrub[head.Event] {
		raw = scrubSensitiveFields(line)
	}
	p.store.RawEvents = append(p.store.RawEvents, RawEvent{
		// string(...) always copies -- safe even though the caller's scanner buffer backing
		// `line` gets reused/overwritten on the next call.
		Timestamp: head.Timestamp, Event: head.Event, SystemName: systemName, BodyID: g.BodyID,
		Raw: string(raw),
	})
}

func (p *Parser) onCommander(e *commanderEntry) {
	name := e.Name
	if name == "" {
		name = e.Commander
	}
	if name == "" {
		return
	}
	p.store.Commander = name
}

func (p *Parser) onJumpOrLocation(e *jumpEntry, timestamp string) {
	if e.SystemAddress == nil || e.StarSystem == "" {
		return
	}
	p.currentSystemAddress = *e.SystemAddress
	p.currentSystemName = e.StarSystem

	sys := p.store.getOrCreateSystem(*e.SystemAddress)
	sys.Name = e.StarSystem
	if e.StarPos != nil {
		sys.X, sys.Y, sys.Z = e.StarPos[0], e.StarPos[1], e.StarPos[2]
		// Prefer a real CodexEntry's own Region_Localised when we've seen one (ground-truth,
		// see region.go); only fall back to the coordinate lookup if we haven't yet.
		if sys.Region == "" {
			sys.Region = findRegion(e.StarPos[0], e.StarPos[1], e.StarPos[2])
		}
	}
	if e.Population != nil {
		sys.Population = *e.Population
	}
	if e.SystemFaction != nil && e.SystemFaction.Name != "" {
		sys.Faction = e.SystemFaction.Name
	}
	if e.SystemGovernmentLocalised != "" {
		sys.Government = e.SystemGovernmentLocalised
	}
	if e.SystemAllegiance != "" {
		sys.Allegiance = e.SystemAllegiance
	}
	if e.SystemSecurityLocalised != "" {
		sys.Security = e.SystemSecurityLocalised
	}
	sys.UpdatedAt = timestamp
}

// CodexEntry directly reports Region_Localised -- a more direct ground-truth than the
// coordinate lookup (confirmed matching exactly for the same real coordinate in the Python
// version, see docs/StandaloneJournalParser.md), used opportunistically whenever a CodexEntry
// happens to fire, overwriting any earlier coordinate-based guess.
func (p *Parser) onCodexEntry(e *codexEntryEvent) {
	if e.SystemAddress == nil || e.RegionLocalised == "" {
		return
	}
	sys := p.store.getOrCreateSystem(*e.SystemAddress)
	sys.Region = e.RegionLocalised
}

func (p *Parser) onFSSDiscoveryScan(e *fssDiscoveryEntry, timestamp string) {
	if e.SystemAddress == nil {
		return
	}
	sys := p.store.getOrCreateSystem(*e.SystemAddress)
	if sys.Name == "" {
		sys.Name = e.SystemName
	}
	if e.BodyCount != nil {
		sys.BodyCountTotal = *e.BodyCount
	}
	sys.Honked = true
	if e.Progress != nil && *e.Progress >= 1.0 {
		sys.FullyScanned = true
	}
	sys.UpdatedAt = timestamp
}

func (p *Parser) onColonisationClaim(e *colonisationClaimEntry, timestamp string) {
	if e.SystemAddress == nil {
		return
	}
	sys := p.store.getOrCreateSystem(*e.SystemAddress)
	if sys.Name == "" {
		sys.Name = e.StarSystem
	}
	sys.ClaimedByCmdr = true
	sys.UpdatedAt = timestamp
}

func (p *Parser) onScan(e *scanEntry, timestamp string) {
	systemAddress := int64(0)
	if e.SystemAddress != nil {
		systemAddress = *e.SystemAddress
	} else {
		systemAddress = p.currentSystemAddress
	}
	if systemAddress == 0 || e.BodyID == nil {
		return
	}
	sys := p.store.getOrCreateSystem(systemAddress)

	if e.StarType != "" {
		st, ok := sys.Stars[*e.BodyID]
		if !ok {
			st = &Star{BodyID: *e.BodyID}
			sys.Stars[*e.BodyID] = st
		}
		st.Name = e.BodyName
		if e.DistanceFromArrivalLS != nil {
			st.Distance = *e.DistanceFromArrivalLS
		}
		st.Type = e.StarType
		if e.Subclass != nil {
			st.Subclass = *e.Subclass
		}
		st.Luminosity = e.Luminosity
		if e.StellarMass != nil {
			st.Mass = *e.StellarMass
		}
		st.WasDiscovered = e.WasDiscovered
		st.WasFootfalled = e.WasFootfalled
		_, st.BarycenterIDs = parentAncestry(e.Parents)
		st.UpdatedAt = timestamp
	} else if e.PlanetClass != "" {
		pl, ok := sys.Planets[*e.BodyID]
		if !ok {
			pl = &Planet{BodyID: *e.BodyID, Flora: make(map[string]*FloraScan)}
			sys.Planets[*e.BodyID] = pl
		}
		pl.Name = e.BodyName
		pl.Type = e.PlanetClass
		if e.Landable != nil {
			pl.Landable = *e.Landable
		}
		if e.MassEM != nil {
			pl.Mass = *e.MassEM
		}
		if e.DistanceFromArrivalLS != nil {
			pl.Distance = *e.DistanceFromArrivalLS
		}
		pl.ParentStarBodyID, pl.BarycenterIDs = parentAncestry(e.Parents)
		pl.TerraformState = e.TerraformState
		pl.Atmosphere = e.AtmosphereType
		if e.SurfaceGravity != nil {
			pl.Gravity = *e.SurfaceGravity
		}
		if e.SurfaceTemperature != nil {
			pl.Temp = *e.SurfaceTemperature
		}
		if e.SurfacePressure != nil {
			pl.Pressure = *e.SurfacePressure
		}
		pl.Discovered = true
		pl.WasDiscovered = e.WasDiscovered
		pl.WasMapped = e.WasMapped
		pl.WasFootfalled = e.WasFootfalled
		pl.UpdatedAt = timestamp
	}
}

// parentAncestry: `Parents` is an ordered ancestor chain (closest first) keyed by BodyID, e.g.
// [{"Star":1},{"Null":0}] -- 'Null' entries are barycenters, not real bodies. Returns the first
// real 'Star' ancestor's BodyID, unresolved -- name resolution happens at render time
// (viewer.go), not here, since a planet's own Scan can fire before its parent star's own Scan
// (confirmed against real data, see docs/StandaloneJournalParser.md), so a parse-time name
// lookup could permanently miss it.
//
// A body genuinely orbiting a binary pair's shared barycenter (not either star individually --
// real example confirmed against live data: bodies named "AB 2"/"AB 4") has no 'Star' entry
// anywhere in its chain at all, only 'Null' ones. Those Nulls' own BodyIDs are the shared
// identifiers that let viewer.go later find every star sharing any of the same barycenter
// ancestry (this same function run against each star's own Parents), so they're returned as a
// fallback rather than discarded -- discarding them (the previous behavior) is what silently
// dropped every circumbinary body into an unresolvable "unknown parent" bucket instead of
// grouping it correctly.
//
// ALL Nulls in the chain are kept, not just the closest one: confirmed against real data that
// ED sometimes inserts an extra synthetic barycenter layer even for a plain two-star pair (e.g.
// "AB 2" has chain [Null 27, Null 0] while stars A and B both sit directly on [Null 0] --
// barycenter 27 isn't a third body, just a tighter orbital grouping that itself sits under the
// same outer barycenter 0 the stars share). Matching on the full set rather than just the
// nearest entry is what makes viewer.go correctly attribute "AB 2" to both A and B instead of
// only the simpler "AB 4" (whose chain goes straight to Null 0). This can over-attribute in a
// genuine three-or-more-star hierarchy where an inner pair's own barycenter chain happens to
// pass through the same outer node as a third, unrelated star -- accepted as a rare edge case
// with no real example confirmed in this project's data, versus the near-certain false negative
// of the single-nearest-barycenter approach on any real binary.
func parentAncestry(parents []map[string]int) (starID *int, barycenterIDs []int) {
	for _, entry := range parents {
		if id, ok := entry["Star"]; ok {
			v := id
			return &v, nil
		}
	}
	for _, entry := range parents {
		if id, ok := entry["Null"]; ok {
			barycenterIDs = append(barycenterIDs, id)
		}
	}
	return nil, barycenterIDs
}

func (p *Parser) onSAAScanComplete(e *saaScanCompleteEntry, timestamp string) {
	systemAddress := int64(0)
	if e.SystemAddress != nil {
		systemAddress = *e.SystemAddress
	} else {
		systemAddress = p.currentSystemAddress
	}
	if systemAddress == 0 || e.BodyID == nil {
		return
	}
	sys := p.store.getOrCreateSystem(systemAddress)
	pl, ok := sys.Planets[*e.BodyID]
	if !ok {
		pl = &Planet{BodyID: *e.BodyID, Flora: make(map[string]*FloraScan)}
		sys.Planets[*e.BodyID] = pl
	}
	pl.Mapped = true
	pl.WasMapped = e.WasMapped
	if e.ProbesUsed != nil && e.EfficiencyTarget != nil {
		pl.Efficient = *e.EfficiencyTarget >= *e.ProbesUsed
	}
	pl.UpdatedAt = timestamp
}

func (p *Parser) onSignals(e *signalsEntry, timestamp string) {
	systemAddress := int64(0)
	if e.SystemAddress != nil {
		systemAddress = *e.SystemAddress
	} else {
		systemAddress = p.currentSystemAddress
	}
	if systemAddress == 0 || e.BodyID == nil {
		return
	}
	bioCount := 0
	for _, sig := range e.Signals {
		if sig.Type == "$SAA_SignalType_Biological;" {
			bioCount = sig.Count
		}
	}
	if bioCount == 0 {
		return
	}
	sys := p.store.getOrCreateSystem(systemAddress)
	pl, ok := sys.Planets[*e.BodyID]
	if !ok {
		pl = &Planet{BodyID: *e.BodyID, Flora: make(map[string]*FloraScan)}
		sys.Planets[*e.BodyID] = pl
	}
	pl.BioSignalCount = bioCount
	pl.UpdatedAt = timestamp
}

func (p *Parser) onScanOrganic(e *scanOrganicEntry, timestamp string) {
	systemAddress := int64(0)
	if e.SystemAddress != nil {
		systemAddress = *e.SystemAddress
	} else {
		systemAddress = p.currentSystemAddress
	}
	if systemAddress == 0 || e.Body == nil || e.Genus == "" || e.Species == "" {
		return
	}
	sys := p.store.getOrCreateSystem(systemAddress)
	pl, ok := sys.Planets[*e.Body]
	if !ok {
		pl = &Planet{BodyID: *e.Body, Flora: make(map[string]*FloraScan)}
		sys.Planets[*e.Body] = pl
	}
	key := e.Genus + "|" + e.Species
	fs, ok := pl.Flora[key]
	if !ok {
		fs = &FloraScan{Genus: e.Genus, Species: e.Species}
		pl.Flora[key] = fs
	}
	fs.Variant = e.Variant
	count := 0
	switch e.ScanType {
	case "Log":
		count = 1
	case "Sample":
		count = 2
	case "Analyse":
		count = 3
	}
	if count > fs.Count {
		fs.Count = count
	}
	if e.ScanType == "Analyse" {
		fs.WasLogged = e.WasLogged
	}
	fs.ScannedAt = timestamp
}

func (p *Parser) onSellOrganicData(timestamp string) {
	p.store.SaleTimes = append(p.store.SaleTimes, timestamp)
}

func (p *Parser) onMultiSellExplorationData(e *multiSellEntry, timestamp string) {
	names := make([]string, 0, len(e.Discovered))
	for _, d := range e.Discovered {
		names = append(names, d.SystemName)
	}
	sale := SystemSale{SoldAt: timestamp, SystemNames: names}
	if e.BaseValue != nil {
		sale.BaseValue = *e.BaseValue
	}
	if e.Bonus != nil {
		sale.Bonus = *e.Bonus
	}
	if e.TotalEarnings != nil {
		sale.TotalEarnings = *e.TotalEarnings
	}
	p.store.SystemSales = append(p.store.SystemSales, sale)
}

func (p *Parser) onDied(timestamp string) {
	p.store.DeathTimes = append(p.store.DeathTimes, timestamp)
}

func (p *Parser) onResurrect(e *resurrectEntry, timestamp string) {
	p.store.Resurrections = append(p.store.Resurrections, Resurrection{
		ResurrectedAt: timestamp, Option: e.Option,
	})
}
