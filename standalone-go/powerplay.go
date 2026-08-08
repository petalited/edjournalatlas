package main

// powerplay.go builds a 7th page: a dedicated Powerplay status + history view -- a full page
// rather than just an expanded section on the recap page, with real history/a chart, not just
// current standing.
//
// Real data (see summary.go's own Powerplay-family comment for the periodic-snapshot-vs-delta
// distinction already established there): "Powerplay" is a periodic snapshot of CURRENT standing
// (Power/Rank/Merits/TimePledged); "PowerplayRank" fires exactly at a rank-up; "PowerplayMerits"
// fires per merit-earning action (MeritsGained); "PowerplayDeliver"/"PowerplayCollect" are the
// real commodity-reinforcement mechanic (Count/Type); "PowerplayJoin" fires once, at the real
// pledge moment (Power only, no other fields). This file keeps the per-event detail summary.go's
// own BuildRecap deliberately throws away (it only needed running totals) -- a real history/log
// and a real cumulative-merits-over-time chart need every individual event, not just a sum.

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

//go:embed powerplay_template.html
var powerplayTemplate string

type powerplayJoinEvent struct {
	Power string `json:"Power"`
}

// Type is a real raw internal commodity symbol (e.g. "$siriusindustrialequipment_name;") --
// confirmed against the real event shape that TypeLocalised is a real sibling field, same pattern
// as basically every other enum-ish field in this journal. Real bug, fixed: the first version only
// captured Type, so the raw unresolved symbol leaked straight into the UI instead of a real name.
type powerplayDeliverEvent struct {
	Power         string `json:"Power"`
	Count         int64  `json:"Count"`
	Type          string `json:"Type"`
	TypeLocalised string `json:"Type_Localised"`
}

// deliverGoodsName: real Localised sibling preferred, same fallback-to-raw-symbol pattern used
// everywhere else in this project when a real _Localised field is present but happens to be empty.
func deliverGoodsName(v powerplayDeliverEvent) string {
	if v.TypeLocalised != "" {
		return v.TypeLocalised
	}
	return v.Type
}

type powerplaySystemDelivery struct {
	System string `json:"system"`
	Count  int64  `json:"count"`
}

type powerplayHistoryEntry struct {
	Timestamp string `json:"timestamp"`
	Kind      string `json:"kind"` // "merits" | "rank" | "delivered" | "collected" | "joined"
	Label     string `json:"label"`
	Detail    string `json:"detail,omitempty"`
	// Power: the real Power this specific event belonged to, read off that event's own Power
	// field (confirmed via EDCD's journal docs + this project's own real captured data: Deliver/
	// Collect/Merits/Rank/Join all carry their own Power field, not just Join) -- deliberately NOT
	// assumed to be data.Power (the commander's CURRENT pledge), since a real PowerplayDefect means
	// older history entries can genuinely belong to a different power than the current one. Used
	// client-side to color-code each feed entry by its own real power.
	Power string `json:"power,omitempty"`
}

type powerplayChartPoint struct {
	Timestamp        string `json:"timestamp"`
	CumulativeMerits int64  `json:"cumulativeMerits"`
}

type powerplayData struct {
	GeneratedAt string `json:"generatedAt"`
	Commander   string `json:"commander"`
	Pledged     bool   `json:"pledged"`
	// Current standing -- same "latest by timestamp across every Powerplay-family event" rule
	// summary.go's own comment on powerplayEvent already establishes (Powerplay's own periodic
	// snapshot can lag a rank-up moment PowerplayRank already reported).
	Power           string  `json:"power,omitempty"`
	Rank            int     `json:"rank,omitempty"`
	Merits          int64   `json:"merits,omitempty"`
	TimePledgedDays float64 `json:"timePledgedDays,omitempty"`
	JoinedAt        string  `json:"joinedAt,omitempty"`
	TotalDelivered  int64   `json:"totalDelivered"`
	TotalCollected  int64   `json:"totalCollected"`
	// History is reverse-chronological (most recent first) -- a real activity log/feed, and this
	// page's own "table view" fallback for the chart below (see dataviz skill's accessibility
	// check: a chart needs a non-visual alternative, and a real per-event log already IS one).
	History []powerplayHistoryEntry `json:"history"`
	// Chart is chronological (oldest first) -- a real cumulative-merits-over-time series showing
	// how you're contributing to your power.
	Chart []powerplayChartPoint `json:"chart"`
	// DeliveredSystems: real PowerplayDeliver events carry no system field of their own (confirmed
	// against the real event shape, just Count/Power/Type), so this comes from RawEvent's own
	// SystemName (best-effort "whichever system you were in when this fired", already tracked by
	// this project's parser for exactly this kind of event). A real per-system total, sorted by
	// amount (biggest contribution first) -- a flat name list alone wasn't informative enough.
	DeliveredSystems []powerplaySystemDelivery `json:"deliveredSystems,omitempty"`
}

func BuildPowerplayData(store *Store) powerplayData {
	data := powerplayData{Commander: store.Commander}

	var latestTS string
	var history []powerplayHistoryEntry
	var cumulativeMerits int64
	var chart []powerplayChartPoint
	deliveredSystems := map[string]int64{}

	for _, e := range store.RawEvents {
		switch e.Event {
		case "Powerplay":
			var v powerplayEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Power != "" && e.Timestamp >= latestTS {
				latestTS = e.Timestamp
				data.Pledged = true
				data.Power, data.Rank, data.Merits = v.Power, v.Rank, v.Merits
				data.TimePledgedDays = float64(v.TimePledged) / 86400
			}
		case "PowerplayRank":
			var v powerplayRankEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Power != "" {
				if e.Timestamp >= latestTS {
					latestTS = e.Timestamp
					data.Pledged = true
					data.Power, data.Rank = v.Power, v.Rank
				}
				history = append(history, powerplayHistoryEntry{
					Timestamp: e.Timestamp, Kind: "rank", Power: v.Power,
					Label: "Reached Powerplay rank " + fmtInt(int64(v.Rank)),
				})
			}
		case "PowerplayJoin":
			var v powerplayJoinEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Power != "" {
				data.JoinedAt = e.Timestamp
				history = append(history, powerplayHistoryEntry{
					Timestamp: e.Timestamp, Kind: "joined", Power: v.Power,
					Label: "Pledged to " + v.Power,
				})
			}
		case "PowerplayMerits":
			var v struct {
				Power        string `json:"Power"`
				MeritsGained int64  `json:"MeritsGained"`
			}
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.MeritsGained != 0 {
				cumulativeMerits += v.MeritsGained
				chart = append(chart, powerplayChartPoint{Timestamp: e.Timestamp, CumulativeMerits: cumulativeMerits})
				label := "+" + fmtInt(v.MeritsGained) + " merits"
				if v.Power != "" {
					label += " for " + v.Power
				}
				if e.SystemName != "" {
					label += " in " + e.SystemName
				}
				history = append(history, powerplayHistoryEntry{
					Timestamp: e.Timestamp, Kind: "merits", Power: v.Power,
					Label: label,
				})
			}
		case "PowerplayDeliver":
			var v powerplayDeliverEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Count > 0 {
				data.TotalDelivered += v.Count
				label := "Delivered " + fmtInt(v.Count) + " " + deliverGoodsName(v)
				if e.SystemName != "" {
					deliveredSystems[e.SystemName] += v.Count
					label += " in " + e.SystemName
				}
				history = append(history, powerplayHistoryEntry{
					Timestamp: e.Timestamp, Kind: "delivered", Power: v.Power,
					Label: label,
				})
			}
		case "PowerplayCollect":
			var v powerplayDeliverEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Count > 0 {
				data.TotalCollected += v.Count
				history = append(history, powerplayHistoryEntry{
					Timestamp: e.Timestamp, Kind: "collected", Power: v.Power,
					Label: "Collected " + fmtInt(v.Count) + " " + deliverGoodsName(v),
				})
			}
		}
	}

	sort.SliceStable(history, func(i, j int) bool { return history[i].Timestamp > history[j].Timestamp })
	data.History = history
	data.Chart = chart
	for sys, count := range deliveredSystems {
		data.DeliveredSystems = append(data.DeliveredSystems, powerplaySystemDelivery{System: sys, Count: count})
	}
	sort.Slice(data.DeliveredSystems, func(i, j int) bool { return data.DeliveredSystems[i].Count > data.DeliveredSystems[j].Count })
	data.GeneratedAt = time.Now().UTC().Format("2006-01-02 15:04 MST")
	return data
}

func RenderPowerplay(store *Store) (string, bool, error) {
	data := BuildPowerplayData(store)
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", false, err
	}
	escaped := strings.ReplaceAll(string(dataJSON), "</", "<\\/")
	html := strings.Replace(powerplayTemplate, "__DATA_JSON__", escaped, 1)
	return html, data.Pledged, nil
}
