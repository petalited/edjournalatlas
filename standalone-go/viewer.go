package main

// Builds the same self-contained local-EDSM-style HTML viewer as edexotracker's standalone/
// viewer.py + viewer_template.py -- identical JSON data shape, so the exact same embedded
// JS/CSS (viewer_template.html, a verbatim copy) renders it without any changes on that side.

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

//go:embed viewer_template.html
var viewerTemplate string

// Same notable-body-type mapping as the Python version's NOTABLE_BODY_TYPES.
var notableBodyTypes = map[string]string{
	"Earthlike body": "Earthlike world",
	"Water world":    "Water world",
	"Ammonia world":  "Ammonia world",
	"Water giant":    "Water giant",
}

func classifyRareStarType(typeCode string) string {
	switch {
	case typeCode == "SupermassiveBlackHole":
		return "Supermassive black hole"
	case typeCode == "H":
		return "Black hole"
	case typeCode == "N":
		return "Neutron star"
	case strings.HasPrefix(typeCode, "D"):
		return "White dwarf"
	case strings.HasPrefix(typeCode, "W"):
		return "Wolf-Rayet star"
	case typeCode == "AeBe":
		return "Herbig Ae/Be star"
	case strings.HasPrefix(typeCode, "C"):
		return "Carbon star"
	case typeCode == "MS" || typeCode == "S":
		return typeCode + "-type star"
	}
	return ""
}

type viewerFlora struct {
	Genus            string `json:"genus"`
	Species          string `json:"species"`
	Variant          string `json:"variant,omitempty"`
	Count            int    `json:"count"`
	Value            *int64 `json:"value"`
	BaseValue        int64  `json:"baseValue"`
	Sold             bool   `json:"sold"`
	Lost             bool   `json:"lost"`
	FootfallBonus    bool   `json:"footfallBonus"`
	FirstLoggedBonus bool   `json:"firstLoggedBonus"`
	WasLogged        *bool  `json:"wasLogged"`
	ScannedAt        string `json:"scannedAt,omitempty"`
}

type viewerBody struct {
	Name           string        `json:"name"`
	Type           string        `json:"type"`
	Landable       bool          `json:"landable"`
	Mass           float64       `json:"mass"`
	Distance       float64       `json:"distance"`
	ParentStars    []string      `json:"parentStars"`
	IsMoon         bool          `json:"isMoon"`                 // orbits another planet directly (see Planet.OrbitsPlanetBodyID, parse.go), not the primary
	OrbitsBodyID   *int          `json:"orbitsBodyId,omitempty"` // the parent planet's raw BodyID, same value as IsMoon is derived from -- exposed so the client can group sibling moons under the same parent to tell the last one apart (tree connector: "├─" vs "└─")
	Terraformable  bool          `json:"terraformable"`
	TerraformState string        `json:"terraformState,omitempty"`
	Atmosphere     string        `json:"atmosphere,omitempty"`
	Gravity        float64       `json:"gravity"`
	Temp           float64       `json:"temp"`
	Pressure       float64       `json:"pressure"`
	Discovered     bool          `json:"discovered"`
	WasDiscovered  *bool         `json:"wasDiscovered"`
	Mapped         bool          `json:"mapped"`
	WasMapped      *bool         `json:"wasMapped"`
	Efficient      bool          `json:"efficient"`
	WasFootfalled  *bool         `json:"wasFootfalled"`
	BioSignalCount int           `json:"bioSignalCount"`
	NotableLabel   string        `json:"notableLabel,omitempty"`
	Flora          []viewerFlora `json:"flora"`
}

type viewerStar struct {
	Name          string  `json:"name"`
	Distance      float64 `json:"distance"`
	Type          string  `json:"type"`
	Subclass      int     `json:"subclass"`
	Luminosity    string  `json:"luminosity"`
	Mass          float64 `json:"mass"`
	WasDiscovered *bool   `json:"wasDiscovered"`
	WasFootfalled *bool   `json:"wasFootfalled"`
	NotableLabel  string  `json:"notableLabel,omitempty"`
}

type viewerSystem struct {
	Name                string         `json:"name"`
	X                   float64        `json:"x"`
	Y                   float64        `json:"y"`
	Z                   float64        `json:"z"`
	Region              string         `json:"region"`
	Population          int64          `json:"population"`
	Faction             string         `json:"faction"`
	BodyCountTotal      int            `json:"bodyCountTotal"`
	RecordedBodyCount   int            `json:"recordedBodyCount"`
	ClaimedByCommander  bool           `json:"claimedByCommander"`
	NotableCounts       map[string]int `json:"notableCounts"`
	FirstDiscoveryCount int            `json:"firstDiscoveryCount"`
	BioValue            int64          `json:"bioValue"`
	Stars               []viewerStar   `json:"stars"`
	Bodies              []viewerBody   `json:"bodies"`
}

type viewerData struct {
	GeneratedAt string         `json:"generatedAt"`
	Systems     []viewerSystem `json:"systems"`
}

func Render(store *Store) (string, int, error) {
	floraValues := ComputeFloraValues(store)
	floraByBody := make(map[int64]map[int][]FloraValue)
	for _, fv := range floraValues {
		if floraByBody[fv.SystemAddress] == nil {
			floraByBody[fv.SystemAddress] = make(map[int][]FloraValue)
		}
		floraByBody[fv.SystemAddress][fv.BodyID] = append(floraByBody[fv.SystemAddress][fv.BodyID], fv)
	}

	systems := []viewerSystem{}
	for systemAddress, sys := range store.Systems {
		starNameByBodyID := make(map[int]string)
		// Which stars share the same immediate barycenter -- built from the SAME parent-ancestry
		// resolution used for planets (see parentAncestry in parse.go), just run against each
		// star's own Parents instead. Lets a circumbinary body whose only resolvable ancestor is
		// a barycenter (not a single star) be attributed to every star sharing that barycenter,
		// instead of falling into an unresolvable "unknown parent" bucket.
		starsByBarycenter := make(map[int][]string)
		stars := []viewerStar{}
		for bodyID, st := range sys.Stars {
			starNameByBodyID[bodyID] = st.Name
			for _, bcID := range st.BarycenterIDs {
				starsByBarycenter[bcID] = append(starsByBarycenter[bcID], st.Name)
			}
			stars = append(stars, viewerStar{
				Name: st.Name, Distance: st.Distance, Type: st.Type, Subclass: st.Subclass,
				Luminosity: st.Luminosity, Mass: st.Mass,
				WasDiscovered: st.WasDiscovered, WasFootfalled: st.WasFootfalled,
				NotableLabel: classifyRareStarType(st.Type),
			})
		}
		sort.Slice(stars, func(i, j int) bool { return stars[i].Distance < stars[j].Distance })
		for id := range starsByBarycenter {
			sort.Strings(starsByBarycenter[id])
		}

		bodies := []viewerBody{}
		for _, pl := range sys.Planets {
			flora := []viewerFlora{}
			for _, fv := range floraByBody[systemAddress][pl.BodyID] {
				vf := viewerFlora{
					Genus: fv.GenusName, Species: fv.SpeciesName, Variant: fv.Variant, Count: fv.Count,
					BaseValue: fv.BaseValue,
					Sold:      fv.Sold, Lost: fv.Lost,
					FootfallBonus: fv.FootfallBonus, FirstLoggedBonus: fv.FirstLoggedBonus,
					WasLogged: fv.WasLogged, ScannedAt: fv.ScannedAt,
				}
				if fv.HasValue {
					v := fv.Value
					vf.Value = &v
				}
				flora = append(flora, vf)
			}
			var parentStars []string
			if pl.ParentStarBodyID != nil {
				if name := starNameByBodyID[*pl.ParentStarBodyID]; name != "" {
					parentStars = []string{name}
				}
			} else if len(pl.BarycenterIDs) > 0 {
				// Union across every barycenter in this body's own chain (not just the nearest
				// one) matched against every barycenter in each star's own chain -- see
				// parentAncestry's comment in parse.go for why a single-entry match isn't enough.
				seen := make(map[string]bool)
				for _, bcID := range pl.BarycenterIDs {
					for _, name := range starsByBarycenter[bcID] {
						if !seen[name] {
							seen[name] = true
							parentStars = append(parentStars, name)
						}
					}
				}
				sort.Strings(parentStars)
			}
			bodies = append(bodies, viewerBody{
				Name: pl.Name, Type: pl.Type, Landable: pl.Landable, Mass: pl.Mass,
				Distance: pl.Distance, ParentStars: parentStars, IsMoon: pl.OrbitsPlanetBodyID != nil, OrbitsBodyID: pl.OrbitsPlanetBodyID,
				Terraformable:  pl.TerraformState == "Terraformable" || pl.TerraformState == "Terraforming" || pl.TerraformState == "Terraformed",
				TerraformState: pl.TerraformState, Atmosphere: pl.Atmosphere,
				Gravity: pl.Gravity, Temp: pl.Temp, Pressure: pl.Pressure,
				Discovered: pl.Discovered, WasDiscovered: pl.WasDiscovered,
				Mapped: pl.Mapped, WasMapped: pl.WasMapped,
				Efficient: pl.Efficient, WasFootfalled: pl.WasFootfalled,
				BioSignalCount: pl.BioSignalCount, NotableLabel: notableBodyTypes[pl.Type],
				Flora: flora,
			})
		}
		sort.Slice(bodies, func(i, j int) bool { return bodies[i].Distance < bodies[j].Distance })

		notableCounts := make(map[string]int)
		for _, b := range bodies {
			if b.NotableLabel != "" {
				notableCounts[b.NotableLabel]++
			}
		}
		for _, st := range stars {
			if st.NotableLabel != "" {
				notableCounts[st.NotableLabel]++
			}
		}

		firstDiscoveryCount := 0
		for _, b := range bodies {
			if b.Discovered && b.WasDiscovered != nil && !*b.WasDiscovered {
				firstDiscoveryCount++
			}
		}
		for _, st := range stars {
			if st.WasDiscovered != nil && !*st.WasDiscovered {
				firstDiscoveryCount++
			}
		}

		var bioValue int64
		for _, b := range bodies {
			for _, f := range b.Flora {
				if f.Value != nil && !f.Lost {
					bioValue += *f.Value
				}
			}
		}

		systems = append(systems, viewerSystem{
			Name: sys.Name, X: sys.X, Y: sys.Y, Z: sys.Z, Region: sys.Region,
			Population: sys.Population, Faction: sys.Faction,
			BodyCountTotal: sys.BodyCountTotal, RecordedBodyCount: len(bodies) + len(stars),
			ClaimedByCommander: sys.ClaimedByCmdr, NotableCounts: notableCounts,
			FirstDiscoveryCount: firstDiscoveryCount, BioValue: bioValue,
			Stars: stars, Bodies: bodies,
		})
	}
	sort.Slice(systems, func(i, j int) bool { return systems[i].Name < systems[j].Name })

	data := viewerData{
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04 MST"),
		Systems:     systems,
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", 0, err
	}
	escaped := strings.ReplaceAll(string(dataJSON), "</", "<\\/")
	html := strings.Replace(viewerTemplate, "__DATA_JSON__", escaped, 1)
	return html, len(systems), nil
}
