package main

// stats.go builds a 9th page: personal statistics that are real RATES, not just totals -- e.g.
// what % of a given star type has produced a notable body, weighted honestly rather than as a
// raw count. Two real methodology choices worth being explicit about, since a naive version of
// this stat would be misleading:
//
//  1. Denominator honesty: a system that's only PARTIALLY scanned can't fairly count as "no
//     notable body" -- you just haven't looked at everything yet. The notable-body RATE only
//     ever divides by FullyScanned systems (a real per-system flag this project already tracks),
//     not every system you've merely visited. Systems still being scanned are counted and shown
//     separately, never silently dropped or silently counted as a miss.
//  2. Sample-size honesty: a star type with only 2 systems scanned can produce a 50%/100%/0% rate
//     that means almost nothing. Every rate ships with its real N so the frontend can visibly
//     de-emphasize small samples instead of presenting every percentage with equal confidence.
//
// Star types are grouped the same way viewer.go's own classifyRareStarType already groups rare
// ones (white dwarf/neutron star/black hole/etc, so this page never disagrees with the recap
// about what counts as which), with common main-sequence/brown-dwarf codes and giant-suffix
// variants (e.g. "K_OrangeGiant") added on top so every real star type in this commander's data
// gets a real group, not just the rare ones.

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

//go:embed stats_template.html
var statsTemplate string

// starGroupLabel extends classifyRareStarType (viewer.go) to cover every real star type, not
// just the rare ones that function was built for -- reusing its labels where they already exist
// so this page can never disagree with the recap/viewer about what a "White dwarf" etc. is.
func starGroupLabel(typeCode string) string {
	if label := classifyRareStarType(typeCode); label != "" {
		return label
	}
	if typeCode == "TTS" {
		return "T Tauri star"
	}
	if idx := strings.IndexByte(typeCode, '_'); idx > 0 {
		// Real confirmed suffix pattern, e.g. "K_OrangeGiant", "M_RedGiant" -- a giant is a
		// physically distinct population from a main-sequence star of the same letter, so it
		// gets its own group rather than being folded into the bare letter's stats.
		base, suffix := typeCode[:idx], typeCode[idx+1:]
		return base + " (" + prettifyStarSuffix(suffix) + ")"
	}
	return typeCode
}

func prettifyStarSuffix(suffix string) string {
	var out strings.Builder
	for i, r := range suffix {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
	}
	return out.String()
}

type starTypeStat struct {
	StarGroup string `json:"starGroup"`
	// SystemCount: every system where this star group is the primary/arrival star (BodyID 0),
	// regardless of scan completeness -- the honest total, shown even though it's not the rate's
	// own denominator.
	SystemCount int `json:"systemCount"`
	// FullyScannedCount: the subset of SystemCount that's actually FullyScanned -- the real,
	// fair denominator for NotableRate (see this file's own header comment on why).
	FullyScannedCount  int     `json:"fullyScannedCount"`
	NotableSystemCount int     `json:"notableSystemCount"` // of the fully-scanned ones, how many have >=1 notable planet
	NotableRate        float64 `json:"notableRate"`        // 0 when FullyScannedCount is 0
	// Per-body hit rate: a second, independent reading of the same underlying data -- "of every
	// planet you've actually scanned around this star type, what fraction are notable" -- which
	// answers a subtly different question than the per-system rate above (a system with 40 bodies
	// and 1 notable one reads very differently per-body vs. per-system).
	ScannedPlanetCount int     `json:"scannedPlanetCount"`
	NotablePlanetCount int     `json:"notablePlanetCount"`
	NotablePerBodyRate float64 `json:"notablePerBodyRate"`
	// AvgBodyCount: mean of the REAL total body count the game itself reports per system
	// (System.BodyCountTotal, from FSSDiscoveryScan -- independent of how much you personally
	// scanned), averaged only over Honked systems (the ones that actually got a real BodyCountTotal
	// reported at all).
	HonkedCount  int     `json:"honkedCount"`
	AvgBodyCount float64 `json:"avgBodyCount"`
}

type statsData struct {
	GeneratedAt string         `json:"generatedAt"`
	Commander   string         `json:"commander"`
	StarTypes   []starTypeStat `json:"starTypes"`
}

func BuildStatsData(store *Store) statsData {
	data := statsData{Commander: store.Commander}

	type accum struct {
		systemCount, fullyScannedCount, notableSystemCount int
		scannedPlanetCount, notablePlanetCount             int
		honkedCount                                        int
		bodyCountSum                                       int64
	}
	byGroup := map[string]*accum{}

	for _, sys := range store.Systems {
		primary, ok := sys.Stars[0]
		if !ok || primary.Type == "" {
			continue // no confirmed arrival-star scan for this system -- can't classify it
		}
		group := starGroupLabel(primary.Type)
		a, ok := byGroup[group]
		if !ok {
			a = &accum{}
			byGroup[group] = a
		}
		a.systemCount++

		if sys.Honked && sys.BodyCountTotal > 0 {
			a.honkedCount++
			a.bodyCountSum += int64(sys.BodyCountTotal)
		}

		hasNotable := false
		for _, pl := range sys.Planets {
			if pl.Type == "" {
				continue
			}
			a.scannedPlanetCount++
			if notableBodyTypes[pl.Type] != "" {
				a.notablePlanetCount++
				hasNotable = true
			}
		}
		if sys.FullyScanned {
			a.fullyScannedCount++
			if hasNotable {
				a.notableSystemCount++
			}
		}
	}

	for group, a := range byGroup {
		st := starTypeStat{
			StarGroup:          group,
			SystemCount:        a.systemCount,
			FullyScannedCount:  a.fullyScannedCount,
			NotableSystemCount: a.notableSystemCount,
			ScannedPlanetCount: a.scannedPlanetCount,
			NotablePlanetCount: a.notablePlanetCount,
			HonkedCount:        a.honkedCount,
		}
		if a.fullyScannedCount > 0 {
			st.NotableRate = float64(a.notableSystemCount) / float64(a.fullyScannedCount)
		}
		if a.scannedPlanetCount > 0 {
			st.NotablePerBodyRate = float64(a.notablePlanetCount) / float64(a.scannedPlanetCount)
		}
		if a.honkedCount > 0 {
			st.AvgBodyCount = float64(a.bodyCountSum) / float64(a.honkedCount)
		}
		data.StarTypes = append(data.StarTypes, st)
	}
	sort.Slice(data.StarTypes, func(i, j int) bool { return data.StarTypes[i].SystemCount > data.StarTypes[j].SystemCount })

	data.GeneratedAt = time.Now().UTC().Format("2006-01-02 15:04 MST")
	return data
}

func RenderStats(store *Store) (string, error) {
	data := BuildStatsData(store)
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	escaped := strings.ReplaceAll(string(dataJSON), "</", "<\\/")
	return strings.Replace(statsTemplate, "__DATA_JSON__", escaped, 1), nil
}
