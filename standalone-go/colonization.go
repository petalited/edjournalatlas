package main

// colonization.go builds a page: real per-commander Colonisation activity (systems claimed,
// construction-depot progress with real resource needs, contributions) plus a body-by-body
// economy-bonus/population-effect breakdown for any scanned system, so a body's real colonization
// potential is visible before committing to a build there.
//
// The economy/population reference tables below are manually ported from a researched knowledge
// base built from the community "Elite Dangerous Colonization Mega Guide" v2.3.0 (not code --
// that source has none to import). Real, exact source values, not approximations.

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

//go:embed colonization_template.html
var colonizationTemplate string

// ---- Real per-commander data (journal-derived) ----

type colonisationDepotResourceEvent struct {
	MarketID             int64   `json:"MarketID"`
	ConstructionProgress float64 `json:"ConstructionProgress"`
	ConstructionComplete bool    `json:"ConstructionComplete"`
	ConstructionFailed   bool    `json:"ConstructionFailed"`
	// ResourcesRequired: confirmed real per-commodity breakdown (Name/Name_Localised/
	// RequiredAmount/ProvidedAmount/Payment) -- the previous version of this project only ever
	// captured the top-level Progress/Complete flags and discarded this entirely.
	ResourcesRequired []struct {
		Name           string `json:"Name"`
		NameLocalised  string `json:"Name_Localised"`
		RequiredAmount int64  `json:"RequiredAmount"`
		ProvidedAmount int64  `json:"ProvidedAmount"`
	} `json:"ResourcesRequired"`
}

type colonisationContribResourceEvent struct {
	MarketID      int64 `json:"MarketID"`
	Contributions []struct {
		Name          string `json:"Name"`
		NameLocalised string `json:"Name_Localised"`
		Amount        int64  `json:"Amount"`
	} `json:"Contributions"`
}

type colonyResourceNeed struct {
	Name      string `json:"name"`
	Required  int64  `json:"required"`
	Provided  int64  `json:"provided"`
	Remaining int64  `json:"remaining"`
}

type colonyDepot struct {
	System    string               `json:"system"`
	Station   string               `json:"station"`
	Progress  float64              `json:"progress"`
	Complete  bool                 `json:"complete"`
	Failed    bool                 `json:"failed"`
	Resources []colonyResourceNeed `json:"resources,omitempty"`
	UpdatedAt string               `json:"updatedAt"`
}

type colonyContribution struct {
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
}

// ---- Body-bonus computation (game-mechanics reference, not journal data) ----

// bodyBaseEconomies mirrors edcolony's "Base Inheritable Economy (by local body type)" table
// exactly, keyed by the real PlanetClass strings this project's own Scan parsing already
// confirmed against this commander's actual journal (types.go/parse.go's scanEntry). ELW/WW/AW
// are locked -- ringLocked callers must skip modifiers for these three.
var bodyBaseEconomies = map[string][]string{
	"Earthlike body":                    {"Agriculture", "HighTech", "Military", "Tourism"},
	"Water world":                       {"Agriculture", "Tourism"},
	"Ammonia world":                     {"HighTech", "Tourism"},
	"High metal content body":           {"Extraction"},
	"Metal rich body":                   {"Extraction"},
	"Rocky ice body":                    {"Industrial", "Refinery"},
	"Rocky body":                        {"Refinery"},
	"Icy body":                          {"Industrial"},
	"Sudarsky class I gas giant":        {"HighTech", "Industrial"},
	"Sudarsky class II gas giant":       {"HighTech", "Industrial"},
	"Sudarsky class III gas giant":      {"HighTech", "Industrial"},
	"Sudarsky class IV gas giant":       {"HighTech", "Industrial"},
	"Sudarsky class V gas giant":        {"HighTech", "Industrial"},
	"Water giant":                       {"HighTech", "Industrial"},
	"Gas giant with water based life":   {"HighTech", "Industrial"},
	"Gas giant with ammonia based life": {"HighTech", "Industrial"},
}

// lockedEconomyBody: ELW/WW/AW can't have geological/biological/ring overrides -- their base
// economy is fixed regardless (edcolony's economies-and-links.md, confirmed explicitly).
func lockedEconomyBody(planetClass string) bool {
	return planetClass == "Earthlike body" || planetClass == "Water world" || planetClass == "Ammonia world"
}

// starBaseEconomies mirrors edcolony's star-type row of the same table. Real StarType codes
// confirmed against this commander's own journal (A/B/DA/DAV/DC/F/G/H/K/K_OrangeGiant/L/M/
// M_RedGiant/N/T/TTS/Y): white dwarfs are every code starting with "D", "H"/
// "SupermassiveBlackHole" is a black hole, "N" is a neutron star -- everything else (main
// sequence, brown dwarfs, giants) falls into edcolony's "Brown Dwarfs and all other star types"
// bucket.
func starBaseEconomies(starType string) []string {
	switch {
	case starType == "H" || starType == "SupermassiveBlackHole":
		return []string{"HighTech", "Tourism"}
	case starType == "N":
		return []string{"HighTech", "Tourism"}
	case strings.HasPrefix(starType, "D"):
		return []string{"HighTech", "Tourism"}
	default:
		return []string{"Military"}
	}
}

type economyBonus struct {
	Economy string `json:"economy"`
	Why     string `json:"why"`
}

// planetEconomyBonuses computes which economies a colony-type port would inherit from this real
// scanned body, and why -- base type plus real modifiers (rings/biological/geological signals),
// skipped entirely for the three locked body types per edcolony's table.
func planetEconomyBonuses(pl *Planet) []economyBonus {
	base, ok := bodyBaseEconomies[pl.Type]
	if !ok {
		return nil
	}
	out := make([]economyBonus, 0, len(base)+3)
	for _, e := range base {
		out = append(out, economyBonus{Economy: e, Why: pl.Type})
	}
	if lockedEconomyBody(pl.Type) {
		if len(pl.RingClasses) > 0 {
			out = append(out, economyBonus{Economy: "Extraction", Why: "ringed"})
		}
		return out
	}
	if len(pl.RingClasses) > 0 {
		out = append(out, economyBonus{Economy: "Extraction", Why: "ringed"})
	}
	if pl.BioSignalCount > 0 {
		out = append(out, economyBonus{Economy: "Agriculture", Why: "biological signals"}, economyBonus{Economy: "Terraforming", Why: "biological signals"})
	}
	if pl.GeoSignalCount > 0 {
		out = append(out, economyBonus{Economy: "Extraction", Why: "geological signals"}, economyBonus{Economy: "Industrial", Why: "geological signals"})
	}
	return out
}

// populationBenchmark: real known values from edcolony's own "partial -- many bodies still TBD"
// table (community-sourced, credit CMDR Betel01). Deliberately NOT extrapolated to body types
// this table doesn't cover -- an absent benchmark means "not yet known," not zero.
type populationBenchmark struct {
	Facility  string `json:"facility"`
	RapidPop  string `json:"rapidPop"`  // by tick 15 ("C+15")
	Inflect   string `json:"inflect"`   // inflection point, roughly tick 100-120
	Asymptote string `json:"asymptote"` // theoretical max (K)
}

var populationBenchmarks = map[string]populationBenchmark{
	"Earthlike body-T3":          {Facility: "T3 Orbital", RapidPop: "610m", Inflect: "1,525m", Asymptote: "3,100m"},
	"Earthlike body-T2":          {Facility: "T2 Orbital", RapidPop: "20m", Inflect: "50m", Asymptote: "100m"},
	"Water world-T3":             {Facility: "T3 Orbital", RapidPop: "365m", Inflect: "912.5m", Asymptote: "1,825m"},
	"High metal content body-T1": {Facility: "T1 Planetary", RapidPop: "9.6m", Inflect: "24m", Asymptote: "48m"},
	"Icy body-T1":                {Facility: "T1 Orbital (Commercial)", RapidPop: "5.43k", Inflect: "76.02k", Asymptote: "380.01k"},
}

func populationBenchmarksFor(planetClass string) []populationBenchmark {
	var out []populationBenchmark
	for _, tier := range []string{"T3", "T2", "T1"} {
		if b, ok := populationBenchmarks[planetClass+"-"+tier]; ok {
			out = append(out, b)
		}
	}
	return out
}

// cpCostForNthPort mirrors edcolony's exact CP-cost escalation table (system-construction.md).
// tier is "T2" or "T3"; n is 1-indexed (1st T2/T3 port anywhere in the system, 2nd, ...).
// Primary port, T1, installations, settlements, and hubs are exempt entirely -- not modeled here
// since this project has no way to know which port is the Primary Port from journal data alone.
func cpCostForNthPort(tier string, n int) int {
	switch tier {
	case "T2":
		switch {
		case n <= 2:
			return 3
		case n == 3:
			return 5
		case n == 4:
			return 7
		case n == 5:
			return 9
		default:
			return 3 + (n-2)*2
		}
	case "T3":
		switch {
		case n <= 2:
			return 6
		case n == 3:
			return 12
		case n == 4:
			return 18
		case n == 5:
			return 24
		default:
			return (n - 1) * 6
		}
	}
	return 0
}

// ---- Page data ----

type colonyBodyOut struct {
	Name                 string                `json:"name"`
	Kind                 string                `json:"kind"` // "star" | "planet"
	Type                 string                `json:"type"`
	Bonuses              []economyBonus        `json:"bonuses,omitempty"`
	RingClasses          []string              `json:"ringClasses,omitempty"`
	ReserveLevel         string                `json:"reserveLevel,omitempty"`
	TidalLock            bool                  `json:"tidalLock,omitempty"`
	Landable             bool                  `json:"landable,omitempty"`
	PopulationBenchmarks []populationBenchmark `json:"populationBenchmarks,omitempty"`
}

type colonySystemOut struct {
	System  string          `json:"system"`
	Claimed bool            `json:"claimed"`
	Bodies  []colonyBodyOut `json:"bodies"`
}

type colonizationData struct {
	GeneratedAt      string               `json:"generatedAt"`
	Commander        string               `json:"commander"`
	ClaimedSystems   []string             `json:"claimedSystems"`
	Depots           []colonyDepot        `json:"depots"`
	TotalContributed int64                `json:"totalContributed"`
	Contributions    []colonyContribution `json:"contributions"`
	Systems          []colonySystemOut    `json:"systems"`
}

func BuildColonizationData(store *Store) colonizationData {
	data := colonizationData{Commander: store.Commander}

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

	claimed := map[string]bool{}
	depots := map[int64]*colonyDepot{}
	contribByCommodity := map[string]int64{}
	var totalContributed int64

	for _, e := range store.RawEvents {
		switch e.Event {
		case "ColonisationSystemClaim":
			var v colonisationClaimEntry
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.StarSystem != "" {
				claimed[v.StarSystem] = true
			}
		case "ColonisationConstructionDepot":
			var v colonisationDepotResourceEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.MarketID != 0 {
				d, ok := depots[v.MarketID]
				if !ok {
					d = &colonyDepot{}
					if loc, ok := stations[v.MarketID]; ok {
						d.System, d.Station = loc.System, loc.Station
					}
					depots[v.MarketID] = d
				}
				if e.Timestamp >= d.UpdatedAt {
					d.Progress = v.ConstructionProgress
					d.Complete = v.ConstructionComplete
					d.Failed = v.ConstructionFailed
					d.UpdatedAt = e.Timestamp
					d.Resources = d.Resources[:0]
					for _, r := range v.ResourcesRequired {
						if r.RequiredAmount <= r.ProvidedAmount {
							continue // fully supplied -- not worth listing as a "need"
						}
						name := r.NameLocalised
						if name == "" {
							name = r.Name
						}
						d.Resources = append(d.Resources, colonyResourceNeed{
							Name: name, Required: r.RequiredAmount, Provided: r.ProvidedAmount,
							Remaining: r.RequiredAmount - r.ProvidedAmount,
						})
					}
					sort.Slice(d.Resources, func(i, j int) bool { return d.Resources[i].Remaining > d.Resources[j].Remaining })
				}
			}
		case "ColonisationContribution":
			var v colonisationContribResourceEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				for _, c := range v.Contributions {
					name := c.NameLocalised
					if name == "" {
						name = c.Name
					}
					contribByCommodity[name] += c.Amount
					totalContributed += c.Amount
				}
			}
		}
	}

	for sys := range claimed {
		data.ClaimedSystems = append(data.ClaimedSystems, sys)
	}
	sort.Strings(data.ClaimedSystems)

	for _, d := range depots {
		data.Depots = append(data.Depots, *d)
	}
	sort.Slice(data.Depots, func(i, j int) bool { return data.Depots[i].Progress < data.Depots[j].Progress })

	data.TotalContributed = totalContributed
	for name, amount := range contribByCommodity {
		data.Contributions = append(data.Contributions, colonyContribution{Name: name, Amount: amount})
	}
	sort.Slice(data.Contributions, func(i, j int) bool { return data.Contributions[i].Amount > data.Contributions[j].Amount })

	// Every system this commander has ever scanned a body in -- the button in the systems viewer
	// works for any system, not just ones with real Colonisation activity, so the body-bonus
	// browser needs every candidate, not just claimed/depot ones.
	for _, sys := range store.Systems {
		if len(sys.Stars) == 0 && len(sys.Planets) == 0 {
			continue
		}
		out := colonySystemOut{System: sys.Name, Claimed: sys.ClaimedByCmdr}
		for _, st := range sys.Stars {
			out.Bodies = append(out.Bodies, colonyBodyOut{
				Name: st.Name, Kind: "star", Type: st.Type,
				Bonuses:      starEconomyBonuses(st),
				RingClasses:  st.RingClasses,
				ReserveLevel: st.ReserveLevel,
			})
		}
		for _, pl := range sys.Planets {
			if pl.Type == "" {
				continue
			}
			out.Bodies = append(out.Bodies, colonyBodyOut{
				Name: pl.Name, Kind: "planet", Type: pl.Type,
				Bonuses:              planetEconomyBonuses(pl),
				RingClasses:          pl.RingClasses,
				ReserveLevel:         pl.ReserveLevel,
				TidalLock:            pl.TidalLock,
				Landable:             pl.Landable,
				PopulationBenchmarks: populationBenchmarksFor(pl.Type),
			})
		}
		sort.Slice(out.Bodies, func(i, j int) bool { return out.Bodies[i].Name < out.Bodies[j].Name })
		data.Systems = append(data.Systems, out)
	}
	sort.Slice(data.Systems, func(i, j int) bool { return data.Systems[i].System < data.Systems[j].System })

	data.GeneratedAt = time.Now().UTC().Format("2006-01-02 15:04 MST")
	return data
}

func starEconomyBonuses(st *Star) []economyBonus {
	base := starBaseEconomies(st.Type)
	out := make([]economyBonus, 0, len(base)+1)
	for _, e := range base {
		out = append(out, economyBonus{Economy: e, Why: "star type"})
	}
	if len(st.RingClasses) > 0 {
		out = append(out, economyBonus{Economy: "Extraction", Why: "ringed"})
	}
	return out
}

func RenderColonization(store *Store) (string, error) {
	data := BuildColonizationData(store)
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	escaped := strings.ReplaceAll(string(dataJSON), "</", "<\\/")
	return strings.Replace(colonizationTemplate, "__DATA_JSON__", escaped, 1), nil
}
