package main

// materialtrader.go builds real Material Trader location data from the commander's own journal
// history, plus the trade-ratio math for a shortfall calculator surfaced on both the materials
// page (materials.go) and the engineering planner (engineeringplanner.go). See
// docs/MaterialTraderMechanics.md for the full researched mechanic this implements -- summary:
// three trader types (raw/manufactured/encoded), each restricted to its own category; within a
// category, materials are grouped into named families (materialFamilyCatalog, materialgrades.go);
// trading within the same family costs a pure grade-difference ratio (6^n up, 1:3^n down);
// crossing families (same category still) costs a flat extra x6 on top. Guardian/Thargoid
// materials aren't tradeable at all, which materialFamilyCatalog already reflects by omission.
//
// Owner: "add a trade calulator, so if youre short you can click on what youre short on and see
// what you can trade at said material trader (maybe system name of nearest trader of that type
// from your current pos?". Two real data gaps discussed and resolved with the owner before
// building: (1) the journal's Docked.StationServices only flags THAT a station has a material
// trader, not WHICH of the three types -- the only way to know for certain is a real
// MaterialTrade event there, which carries an explicit TraderType. Chose (owner's call) to only
// ever surface stations the commander has personally traded at before, rather than guessing from
// station economy or adding a live galaxy-wide lookup -- this project has never made a live
// network call anywhere, and stays that way here too.

import (
	"encoding/json"
	"math"
	"sort"
)

type materialTradeEvent struct {
	MarketID   int64  `json:"MarketID"`
	TraderType string `json:"TraderType"` // "raw" / "manufactured" / "encoded"
}

// traderDockedEvent is deliberately its own small struct rather than reusing summary.go's
// dockedEvent -- same "MarketID -> real system/station name" resolution need as
// ColonisationConstructionDepot already solved there, but kept independent so this file doesn't
// have to touch already-working code to add the one extra field (none needed here, as it turns
// out) or risk a future change to dockedEvent silently affecting trader resolution too.
type traderDockedEvent struct {
	MarketID    int64  `json:"MarketID"`
	StarSystem  string `json:"StarSystem"`
	StationName string `json:"StationName"`
}

type knownTraderOut struct {
	Type       string   `json:"type"` // "raw" / "manufactured" / "encoded"
	System     string   `json:"system"`
	Station    string   `json:"station"`
	LastUsedAt string   `json:"lastUsedAt"`
	Distance   *float64 `json:"distanceLy,omitempty"` // from current position, if both are known
}

type materialCatalogEntry struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Grade  int    `json:"grade"`
	Family string `json:"family"`
	Type   string `json:"type"` // "raw" / "manufactured" / "encoded"
}

// materialCatalogForTrading lists every material that actually belongs to a named family --
// exactly the set materialFamilyCatalog already covers, which (by construction, see
// materialgrades.go's own header) is exactly the set of materials real Material Traders deal in.
// Guardian/Thargoid materials are naturally excluded, since they were never given a family entry
// there in the first place -- not a special case here, just falls out of reusing that table.
func materialCatalogForTrading() []materialCatalogEntry {
	out := make([]materialCatalogEntry, 0, 137)
	for _, fam := range materialFamilyCatalog {
		for i, slot := range fam.Slots {
			out = append(out, materialCatalogEntry{
				Key: slot.Key, Name: materialDisplayName(slot.Key), Grade: i + 1,
				Family: fam.Name, Type: fam.Type,
			})
		}
	}
	return out
}

// knownMaterialTraders scans every real MaterialTrade event this commander has ever logged,
// resolves each one's bare MarketID to a real system+station by cross-referencing this
// commander's own Docked history (same pattern already proven for
// ColonisationConstructionDepot in summary.go), and dedupes to one entry per (type, system,
// station) triple, keeping whichever trade was most recent.
func knownMaterialTraders(store *Store) []knownTraderOut {
	stations := map[int64]struct{ System, Station string }{}
	for _, e := range store.RawEvents {
		if e.Event != "Docked" {
			continue
		}
		var v traderDockedEvent
		if json.Unmarshal([]byte(e.Raw), &v) != nil || v.MarketID == 0 {
			continue
		}
		stations[v.MarketID] = struct{ System, Station string }{v.StarSystem, cleanStationName(v.StationName)}
	}

	type tradeKey struct{ Type, System, Station string }
	latestByKey := map[tradeKey]string{}

	for _, e := range store.RawEvents {
		if e.Event != "MaterialTrade" {
			continue
		}
		var v materialTradeEvent
		if json.Unmarshal([]byte(e.Raw), &v) != nil || v.TraderType == "" || v.MarketID == 0 {
			continue
		}
		st, ok := stations[v.MarketID]
		if !ok || st.System == "" {
			continue
		}
		k := tradeKey{v.TraderType, st.System, st.Station}
		if existing, seen := latestByKey[k]; !seen || e.Timestamp > existing {
			latestByKey[k] = e.Timestamp
		}
	}

	out := make([]knownTraderOut, 0, len(latestByKey))
	for k, ts := range latestByKey {
		out = append(out, knownTraderOut{Type: k.Type, System: k.System, Station: k.Station, LastUsedAt: ts})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastUsedAt > out[j].LastUsedAt })
	return out
}

// currentPosition resolves "where the commander is right now" as the system named on the
// chronologically LAST raw journal event -- RawEvent.SystemName is already a best-effort "system
// the commander was last known to be in" on every event, not just location-specific ones (see
// its own comment in types.go), so this doesn't need to specifically hunt for the last
// FSDJump/Location/Docked event.
func currentPosition(store *Store) (systemName string, x, y, z float64, ok bool) {
	if len(store.RawEvents) == 0 {
		return "", 0, 0, 0, false
	}
	latest := store.RawEvents[0]
	for _, e := range store.RawEvents {
		if e.Timestamp >= latest.Timestamp {
			latest = e
		}
	}
	if latest.SystemName == "" {
		return "", 0, 0, 0, false
	}
	for _, sys := range store.Systems {
		if sys.Name == latest.SystemName {
			return sys.Name, sys.X, sys.Y, sys.Z, true
		}
	}
	// Named but coordinates unknown (e.g. system never independently visited/scanned with its
	// own FSDJump/Location StarPos on record) -- still return the name, just no distance sort.
	return latest.SystemName, 0, 0, 0, false
}

// BuildTraderData is shared by both materials.go and engineeringplanner.go's own data builders --
// known traders (sorted nearest-first once distance is known, most-recently-used first
// otherwise), the current system name (for an honest "distances from X" label in the UI), and the
// trading-eligible material catalog the client-side ratio calculator needs.
type traderData struct {
	Traders         []knownTraderOut       `json:"traders"`
	CurrentSystem   string                 `json:"currentSystem,omitempty"`
	MaterialCatalog []materialCatalogEntry `json:"materialCatalog"`
}

func BuildTraderData(store *Store) traderData {
	traders := knownMaterialTraders(store)
	currentName, cx, cy, cz, haveCurrent := currentPosition(store)

	if haveCurrent {
		for i := range traders {
			for _, sys := range store.Systems {
				if sys.Name != traders[i].System {
					continue
				}
				d := math.Sqrt(math.Pow(sys.X-cx, 2) + math.Pow(sys.Y-cy, 2) + math.Pow(sys.Z-cz, 2))
				traders[i].Distance = &d
				break
			}
		}
		sort.SliceStable(traders, func(i, j int) bool {
			if (traders[i].Distance == nil) != (traders[j].Distance == nil) {
				return traders[j].Distance == nil // known-distance entries sort first
			}
			if traders[i].Distance != nil {
				return *traders[i].Distance < *traders[j].Distance
			}
			return false // stable: keep existing most-recent-first order among unknown-distance entries
		})
	}

	return traderData{
		Traders:         traders,
		CurrentSystem:   currentName,
		MaterialCatalog: materialCatalogForTrading(),
	}
}
