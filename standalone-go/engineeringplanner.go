package main

// engineeringplanner.go builds a fifth page: an engineering upgrade planner. Requested directly
// by the owner: "add an engineering planner where you can add a few engineering upgrades to a
// list and ittl tell you a summary of what materials you have for it and which engineers you
// have that can give you it" -- basket-style, multiple picks with quantities ("4 overcharged
// multicannons and 1 range fsd").
//
// The journal itself has no blueprint recipe data at all (confirmed for materials.go already --
// EngineerCraft only records what WAS spent on a specific past craft, and the same blueprint+
// grade can cost genuinely different amounts across repeat applications, almost certainly due to
// unflagged Experimental Effects -- see materials.go's buildEngineeringUsage comment, which is
// exactly why THAT feature never tries to answer "can I afford this again"). So, same "vendor
// real, cited, external reference data" pattern already used for material grades/families and
// the exobiology species table: this vendors msarilar/EDEngineer's blueprints.json (MIT
// licensed, https://github.com/msarilar/EDEngineer/blob/master/EDEngineer/Resources/Data/
// blueprints.json, fetched 2026-08-04) -- an actively maintained community reference covering
// every ship-module blueprint's real per-grade ingredient cost and which Engineer(s) offer it.
//
// Deliberately scoped to ship-module blueprints only: excludes the "Suit" and "Weapon" blueprint
// types entirely (Odyssey on-foot suit/weapon engineering -- a completely separate resource
// system this project has no inventory data for at all; confirmed at vendoring time -- every one
// of their ingredient names, e.g. "Aerogel", "Microelectrode", "Weapon Schematic", failed to
// resolve against this project's own known-materials table, the only case among 907 graded
// blueprints where that happened) and excludes pure-synthesis recipes (Engineers ==
// ["@Synthesis"] -- ammo synthesis craftable anywhere, no engineer involved, doesn't fit "which
// engineer can give me this"). 786 real ship-module blueprints remain, and every one of their
// ingredients resolved cleanly against this project's own 137-material table.
//
// Experimental Effects (real owner catch: "you forgot experimental effects lol") are also from
// engineers and cost the same material pool -- vendored as a second, separate list, scoped the
// same way (real engineer, ship-module, not Suit/Weapon) plus two effect-specific exclusions:
// the upstream data's "@Technology"-only entries (Technology Broker unlocks, not a real
// Engineer) and its "Unlock" Type (engineer INVITE requirements -- a different concept from
// applying an effect to a module you already have). 154 real ship-module experimental effects
// remain. Unlike graded blueprints, an effect isn't tied to a specific grade -- the same effect
// applies regardless of which grade you rolled the base upgrade to, so it's a second, independent
// pick in the UI (Type -> Upgrade -> Grade -> optional Effect), not a 4th column on the same row.
//
// Fully static/offline like every other vendored table here -- no live network calls, refreshed
// by re-running the vendoring step against a newer copy of the upstream JSON.

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

//go:embed vendor/blueprints.json
var blueprintsJSON []byte

//go:embed engineering_template.html
var engineeringTemplate string

type blueprintIngredient struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

type blueprintDef struct {
	Type        string                `json:"type"`
	Name        string                `json:"name"`
	Grade       int                   `json:"grade"` // 0 for entries loaded into effectCatalog -- effects aren't graded
	Engineers   []string              `json:"engineers"`
	Ingredients []blueprintIngredient `json:"ingredients"`
}

type vendoredBlueprintData struct {
	Blueprints []blueprintDef `json:"blueprints"`
	Effects    []blueprintDef `json:"effects"`
}

var blueprintCatalog []blueprintDef
var effectCatalog []blueprintDef

func init() {
	var vendored vendoredBlueprintData
	if err := json.Unmarshal(blueprintsJSON, &vendored); err != nil {
		panic("failed to parse embedded blueprint catalog: " + err.Error())
	}
	blueprintCatalog = vendored.Blueprints
	effectCatalog = vendored.Effects
}

type engineerProgressEntry struct {
	Engineer     string `json:"Engineer"`
	Progress     string `json:"Progress"`
	Rank         int    `json:"Rank"`
	RankProgress int    `json:"RankProgress"`
}

type engineerProgressEvent struct {
	Engineers []engineerProgressEntry `json:"Engineers"`
}

// latestEngineerProgress mirrors latestMaterialsSnapshot's "scan every RawEvent, keep the one
// with the latest real timestamp" approach (materials.go) -- EngineerProgress is the same shape
// of event, a periodic full snapshot fired on login. It ALSO fires as a single-engineer delta at
// the moment of an actual rank-up (flat fields, no "Engineers" array) -- those unmarshal to a
// zero-length Engineers slice here and get skipped, since only the full snapshot answers "which
// engineers do I currently have, in total".
func latestEngineerProgress(store *Store) ([]engineerProgressEntry, bool) {
	var best []engineerProgressEntry
	var bestTS string
	found := false
	for _, e := range store.RawEvents {
		if e.Event != "EngineerProgress" {
			continue
		}
		if found && e.Timestamp <= bestTS {
			continue
		}
		var v engineerProgressEvent
		if json.Unmarshal([]byte(e.Raw), &v) != nil || len(v.Engineers) == 0 {
			continue
		}
		best = v.Engineers
		bestTS = e.Timestamp
		found = true
	}
	return best, found
}

var quotedNicknameRe = regexp.MustCompile(`'[^']*'`)

// normalizeEngineerName strips out any single-quoted nickname (e.g. "Tod 'The Blaster' McQuinn"
// -> "Tod McQuinn") before the usual letters-only normalization -- confirmed against this
// project's own real journal data as the one real case, of the 25 real engineers referenced in
// the vendored blueprint data, where the vendored source's name for an engineer and the
// journal's own name for that same real engineer differ enough that plain normalization
// wouldn't match them (every other real name matched exactly once nicknames were stripped).
func normalizeEngineerName(s string) string {
	s = quotedNicknameRe.ReplaceAllString(s, "")
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c+('a'-'A'))
		}
	}
	return string(out)
}

type engineerStatusOut struct {
	Name     string `json:"name"`               // the real journal name if this commander has any record of this engineer, else the vendored name as a fallback
	Progress string `json:"progress,omitempty"` // "Unlocked" / "Invited" / "Known" -- "" (omitted) means no record at all in this commander's own EngineerProgress
	Rank     int    `json:"rank,omitempty"`
}

type plannerIngredientOut struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type plannerBlueprintOut struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Grade       int                    `json:"grade"`
	Engineers   []engineerStatusOut    `json:"engineers"`
	Ingredients []plannerIngredientOut `json:"ingredients"`
}

// plannerEffectOut is the same shape as plannerBlueprintOut minus Grade -- an Experimental
// Effect isn't tied to a specific grade of the base upgrade, see this file's header comment.
type plannerEffectOut struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Engineers   []engineerStatusOut    `json:"engineers"`
	Ingredients []plannerIngredientOut `json:"ingredients"`
}

type plannerData struct {
	GeneratedAt     string                `json:"generatedAt"`
	Blueprints      []plannerBlueprintOut `json:"blueprints"`
	Effects         []plannerEffectOut    `json:"effects"`
	HeldMaterials   map[string]int64      `json:"heldMaterials"`
	HasSnapshot     bool                  `json:"hasSnapshot"`
	SnapshotAt      string                `json:"snapshotAt,omitempty"`
	HasEngineerData bool                  `json:"hasEngineerData"`
}

func resolveEngineers(names []string, engineerByKey map[string]engineerProgressEntry) []engineerStatusOut {
	out := make([]engineerStatusOut, 0, len(names))
	for _, eng := range names {
		if real, ok := engineerByKey[normalizeEngineerName(eng)]; ok {
			out = append(out, engineerStatusOut{Name: real.Engineer, Progress: real.Progress, Rank: real.Rank})
		} else {
			out = append(out, engineerStatusOut{Name: eng})
		}
	}
	return out
}

func resolveIngredients(ings []blueprintIngredient) []plannerIngredientOut {
	out := make([]plannerIngredientOut, 0, len(ings))
	for _, ing := range ings {
		out = append(out, plannerIngredientOut{Key: ing.Key, Name: materialDisplayName(ing.Key), Count: ing.Count})
	}
	return out
}

func BuildPlannerData(store *Store) plannerData {
	snapshot, snapshotTS, hasSnapshot := latestMaterialsSnapshot(store)
	held := map[string]int64{}
	for _, cat := range [][]materialEntry{snapshot.Raw, snapshot.Manufactured, snapshot.Encoded} {
		for _, m := range cat {
			held[normalizeMaterialKey(m.Name)] += m.Count
		}
	}

	engineers, hasEngineerData := latestEngineerProgress(store)
	engineerByKey := map[string]engineerProgressEntry{}
	for _, e := range engineers {
		engineerByKey[normalizeEngineerName(e.Engineer)] = e
	}

	blueprints := make([]plannerBlueprintOut, 0, len(blueprintCatalog))
	for _, bp := range blueprintCatalog {
		blueprints = append(blueprints, plannerBlueprintOut{
			Type: bp.Type, Name: bp.Name, Grade: bp.Grade,
			Engineers:   resolveEngineers(bp.Engineers, engineerByKey),
			Ingredients: resolveIngredients(bp.Ingredients),
		})
	}

	effects := make([]plannerEffectOut, 0, len(effectCatalog))
	for _, ef := range effectCatalog {
		effects = append(effects, plannerEffectOut{
			Type: ef.Type, Name: ef.Name,
			Engineers:   resolveEngineers(ef.Engineers, engineerByKey),
			Ingredients: resolveIngredients(ef.Ingredients),
		})
	}

	data := plannerData{
		GeneratedAt:     time.Now().UTC().Format("2006-01-02 15:04 MST"),
		Blueprints:      blueprints,
		Effects:         effects,
		HeldMaterials:   held,
		HasSnapshot:     hasSnapshot,
		HasEngineerData: hasEngineerData,
	}
	if hasSnapshot {
		data.SnapshotAt = snapshotTS
	}
	return data
}

func RenderEngineeringPlanner(store *Store) (string, int, error) {
	data := BuildPlannerData(store)
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", 0, err
	}
	escaped := strings.ReplaceAll(string(dataJSON), "</", "<\\/")
	html := strings.Replace(engineeringTemplate, "__DATA_JSON__", escaped, 1)
	return html, len(data.Blueprints), nil
}
