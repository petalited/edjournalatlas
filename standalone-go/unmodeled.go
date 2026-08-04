package main

// unmodeled.go answers a real question the owner raised: this whole project's development
// practice is "ground every stat in this commander's own real journal data before writing code"
// (see summary.go's own header comment) -- which is exactly right for correctness of what IS
// modeled, but has a structural blind spot: an event type (or a field within one this project
// DOES otherwise model, like CommitCrime's Bounty field) that never appears in the one commander's
// journal used for development simply never gets built at all. Nothing crashes for another
// commander whose journal exercises that path (every event type here already gets its own
// tolerant struct -- see parse.go's "field-type collision" comment for why that matters), but
// their data for it silently isn't shown anywhere.
//
// This is the fix: every journal event is already captured verbatim regardless of whether
// anything else in this program recognizes it (see RawEvent/eventsearch.go) -- so this file just
// cross-references that against what summary.go's BuildRecap actually has a case for, and reports
// event TYPES and their real field NAMES (never values) for anything summary.go doesn't cover yet.
// Field names only, not a raw data dump: real journal field names are just the game's fixed
// schema (e.g. "Bounty", "Victim_Localised"), not personal information, so this is safe to
// actually hand to someone else to extend the tool with -- unlike the full raw_events capture
// itself, which does contain real personal data (commander names, credit balances, systems
// visited) and was never intended to leave the commander's own machine.

import (
	"encoding/json"
	"sort"
	"time"
)

// Kept in sync by hand with every top-level `case "X":` in BuildRecap's switch (summary.go) --
// no reflection trick derives this automatically from the switch itself, so this list needs a
// matching update whenever a case is added or removed there. Deliberately conservative: an event
// type going stale on this list just means the coverage report includes something that's
// actually already modeled (a false positive, harmless -- shows up as "0 fields new" at worst),
// never the other way around (a real gap silently omitted from the report).
var recognizedEventTypes = map[string]bool{
	"Loadout": true, "ShipyardBuy": true, "ShipyardSwap": true, "ShipyardNew": true,
	"Bounty": true, "FactionKillBond": true, "Died": true, "Interdicted": true,
	"MarketSell": true, "MarketBuy": true, "MiningRefined": true,
	"MissionCompleted": true, "MissionAccepted": true, "CommitCrime": true,
	"FSDJump": true, "Scan": true, "SAAScanComplete": true, "MultiSellExplorationData": true,
	"WingAdd": true, "EngineerCraft": true, "MaterialCollected": true,
	"Powerplay": true, "PowerplayRank": true, "PowerplayMerits": true,
	"NpcCrewPaidWage": true, "CarrierJump": true, "CarrierStats": true, "Docked": true,
	"ColonisationConstructionDepot": true, "ColonisationContribution": true, "ColonisationSystemClaim": true,
	"Rank": true, "Promotion": true,
}

type unmodeledEventType struct {
	Event  string   `json:"event"`
	Count  int      `json:"count"`
	Fields []string `json:"fields"`
}

type unmodeledReport struct {
	GeneratedAt string               `json:"generatedAt"`
	EventTypes  []unmodeledEventType `json:"eventTypes"`
}

func BuildUnmodeledReport(store *Store) unmodeledReport {
	counts := map[string]int{}
	fieldSets := map[string]map[string]bool{}

	for _, e := range store.RawEvents {
		if recognizedEventTypes[e.Event] {
			continue
		}
		counts[e.Event]++
		var generic map[string]json.RawMessage
		if json.Unmarshal([]byte(e.Raw), &generic) != nil {
			continue
		}
		set := fieldSets[e.Event]
		if set == nil {
			set = map[string]bool{}
			fieldSets[e.Event] = set
		}
		for k := range generic {
			set[k] = true
		}
	}

	types := make([]unmodeledEventType, 0, len(counts))
	for event, count := range counts {
		fields := make([]string, 0, len(fieldSets[event]))
		for f := range fieldSets[event] {
			if f == "event" || f == "timestamp" {
				continue // present on literally everything, not useful signal
			}
			fields = append(fields, f)
		}
		sort.Strings(fields)
		types = append(types, unmodeledEventType{Event: event, Count: count, Fields: fields})
	}
	sort.Slice(types, func(i, j int) bool { return types[i].Count > types[j].Count })

	return unmodeledReport{
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04 MST"),
		EventTypes:  types,
	}
}

func RenderUnmodeledReport(store *Store) (string, int, error) {
	report := BuildUnmodeledReport(store)
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", 0, err
	}
	return string(out) + "\n", len(report.EventTypes), nil
}
