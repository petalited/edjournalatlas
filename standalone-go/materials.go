package main

// materials.go builds a fourth page: a current-inventory viewer for Engineering materials
// (Raw/Manufactured/Encoded), the kind of thing the in-game right-panel Materials screen shows --
// requested directly by the owner ("add inventory viewer ... like an engineer area").
//
// Deliberately NOT computed as net gained-minus-spent from MaterialCollected/EngineerCraft/
// Synthesis/MaterialTrade/MaterialDiscarded/etc: that requires correctly modeling every possible
// consumption path, and silently drifts wrong if any path is missed or mismodeled -- exactly the
// kind of blind spot this project already learned to watch for (see unmodeled.go's own header).
// The journal already gives a much better ground-truth source directly: a "Materials" event is a
// periodic FULL inventory snapshot the game itself sends (confirmed real shape, this commander's
// own data: 126 of them across their history, most recent one from today) -- so this just takes
// the single most-recent one and shows it as-is. No net-math, no drift risk.
//
// Confirmed against real data exactly when "Materials" fires: immediately after "Commander" and
// immediately before "LoadGame" in every real example checked -- i.e. once per game session
// login, not continuously. The page says so explicitly rather than implying this is live.

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

//go:embed materials_template.html
var materialsTemplate string

type materialEntry struct {
	Name          string `json:"Name"`
	NameLocalised string `json:"Name_Localised"`
	Count         int64  `json:"Count"`
}

type materialsSnapshotEvent struct {
	Raw          []materialEntry `json:"Raw"`
	Manufactured []materialEntry `json:"Manufactured"`
	Encoded      []materialEntry `json:"Encoded"`
}

// engineerCraftIngredientsEvent is its own dedicated struct (not a reuse of summary.go's
// engineerCraftEvent) purely for the Ingredients field -- same per-event-type-struct discipline
// parse.go's field-collision comment established, applied here even though there's no actual
// collision risk for this particular field, just to keep each file's event structs scoped to
// exactly what that file needs.
type engineerCraftIngredientsEvent struct {
	BlueprintName string `json:"BlueprintName"`
	Level         int    `json:"Level"`
	Ingredients   []struct {
		Name          string `json:"Name"`
		NameLocalised string `json:"Name_Localised"`
		Count         int64  `json:"Count"`
	} `json:"Ingredients"`
}

type engineeringUsage struct {
	TotalUsed  int64          `json:"totalUsed"`
	Craftings  int            `json:"craftings"`
	Blueprints map[string]int `json:"blueprints"` // "BlueprintName Lv2" -> times used
}

type materialOut struct {
	Name  string            `json:"name"`
	Count int64             `json:"count"`
	Usage *engineeringUsage `json:"usage,omitempty"`
}

type materialsData struct {
	GeneratedAt  string        `json:"generatedAt"`
	HasSnapshot  bool          `json:"hasSnapshot"`
	SnapshotAt   string        `json:"snapshotAt,omitempty"`
	Raw          []materialOut `json:"raw"`
	Manufactured []materialOut `json:"manufactured"`
	Encoded      []materialOut `json:"encoded"`
}

// latestMaterialsSnapshot scans every RawEvent for "Materials" and keeps whichever has the
// latest real timestamp -- a plain linear scan across the already-merged, all-files RawEvents
// slice, not a per-file tail -1 (which would only find the latest in whichever file happened to
// be checked last, not the true latest across the whole journal history -- a real mistake this
// project caught itself making more than once before this page was built).
func latestMaterialsSnapshot(store *Store) (materialsSnapshotEvent, string, bool) {
	var best materialsSnapshotEvent
	var bestTS string
	found := false
	for _, e := range store.RawEvents {
		if e.Event != "Materials" {
			continue
		}
		if found && e.Timestamp <= bestTS {
			continue
		}
		var v materialsSnapshotEvent
		if json.Unmarshal([]byte(e.Raw), &v) != nil {
			continue
		}
		best = v
		bestTS = e.Timestamp
		found = true
	}
	return best, bestTS, found
}

// buildEngineeringUsage aggregates every real EngineerCraft.Ingredients entry ever seen into
// "how much of this material has actually gone into engineering, and for which upgrades" --
// a plain historical sum, not a recipe/cost lookup.
//
// Deliberately NOT used to answer "can I currently afford to redo blueprint X": checked first,
// and confirmed against this commander's real data that the same (BlueprintName, Level) can have
// genuinely DIFFERENT real ingredient costs across repeat applications (8 of 37 real distinct
// blueprint+level combos here have 2-3 different observed ingredient sets -- almost certainly
// because an Experimental Effect was applied on some attempts and not others, which the journal
// doesn't separately flag). Treating any one of those as "the" recipe would be confidently wrong
// roughly a fifth of the time on this commander's own data -- exactly the kind of silent
// incorrectness this project's "ground every claim" standard exists to catch, so that specific
// feature isn't built. What IS safe and still real: total historical consumption per material,
// which needs no recipe-matching at all.
func buildEngineeringUsage(store *Store) map[string]*engineeringUsage {
	usage := map[string]*engineeringUsage{}
	for _, e := range store.RawEvents {
		if e.Event != "EngineerCraft" {
			continue
		}
		var v engineerCraftIngredientsEvent
		if json.Unmarshal([]byte(e.Raw), &v) != nil {
			continue
		}
		if len(v.Ingredients) == 0 {
			continue
		}
		bpLabel := formatBlueprintName(v.BlueprintName)
		if v.Level > 0 {
			bpLabel = bpLabel + " Lv" + fmtInt(int64(v.Level))
		}
		for _, ing := range v.Ingredients {
			name := ing.NameLocalised
			if name == "" {
				name = prettifyKeyStandalone(ing.Name)
			}
			u := usage[name]
			if u == nil {
				u = &engineeringUsage{Blueprints: map[string]int{}}
				usage[name] = u
			}
			u.TotalUsed += ing.Count
			u.Craftings++
			u.Blueprints[bpLabel]++
		}
	}
	return usage
}

// prettifyKeyStandalone mirrors summary.go's BuildRecap-local prettifyKey closure -- duplicated
// (not shared) since that one is a closure scoped inside BuildRecap, not a package-level function,
// and this file's need for it (Ingredients entries can lack Name_Localised the same way
// MaterialCollected/Materials.Raw entries do) is small enough that reaching into summary.go's
// internals isn't worth the coupling.
func prettifyKeyStandalone(key string) string {
	if key == "" {
		return "Unknown"
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

func toMaterialOut(entries []materialEntry, usage map[string]*engineeringUsage) []materialOut {
	out := make([]materialOut, 0, len(entries))
	for _, m := range entries {
		name := m.NameLocalised
		if name == "" {
			name = prettifyKeyStandalone(m.Name) // confirmed real: Raw entries never carry Name_Localised
		}
		mo := materialOut{Name: name, Count: m.Count}
		if u, ok := usage[name]; ok {
			mo.Usage = u
		}
		out = append(out, mo)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

func BuildMaterialsData(store *Store) materialsData {
	snapshot, snapshotTS, found := latestMaterialsSnapshot(store)
	usage := buildEngineeringUsage(store)

	data := materialsData{
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04 MST"),
		HasSnapshot: found,
	}
	if found {
		data.SnapshotAt = snapshotTS
		data.Raw = toMaterialOut(snapshot.Raw, usage)
		data.Manufactured = toMaterialOut(snapshot.Manufactured, usage)
		data.Encoded = toMaterialOut(snapshot.Encoded, usage)
	}
	return data
}

func RenderMaterials(store *Store) (string, int, error) {
	data := BuildMaterialsData(store)
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", 0, err
	}
	escaped := strings.ReplaceAll(string(dataJSON), "</", "<\\/")
	html := strings.Replace(materialsTemplate, "__DATA_JSON__", escaped, 1)
	total := len(data.Raw) + len(data.Manufactured) + len(data.Encoded)
	return html, total, nil
}
