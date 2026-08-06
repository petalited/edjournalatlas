package main

// summary.go builds a third page: a whole-career recap (combat, trading, mining, ships,
// missions, rivalries) computed from the same universal RawEvent capture eventsearch.go uses,
// pre-aggregated into a curated set of interesting numbers instead of a raw searchable log.
// Go-only -- see docs/StandaloneJournalParser.md for why the Python side stopped getting new
// features partway through this session.
//
// Every stat here is grounded against this commander's real journal data before being written,
// same practice as the rest of this project: e.g. ship internal keys (Loadout's "Ship" field,
// like "smallcombat01_nx") have no display name of their own -- confirmed that ShipyardBuy/
// ShipyardSwap/ShipyardNew carry a ShipType_Localised field for the same key, so that's built
// into a real lookup map from this commander's own data rather than a hand-maintained/guessed
// ship database that could silently go stale as new ships are added.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

//go:embed summary_template.html
var summaryTemplate string

type bountyEvent struct {
	Target          string `json:"Target"`
	TargetLocalised string `json:"Target_Localised"`
	TotalReward     int64  `json:"TotalReward"`
	VictimFaction   string `json:"VictimFaction"`
}

type factionKillBondEvent struct {
	Reward        int64  `json:"Reward"`
	VictimFaction string `json:"VictimFaction"`
}

// RedeemVoucher.Amount is the ACTUAL money credited (net of any broker fee, per the journal
// manual) at the moment of redemption -- confirmed real vs. Bounty/FactionKillBond, which only
// record a voucher being EARNED, not collected (real vouchers are lost entirely if the ship is
// destroyed before redeeming). Type casing is confirmed inconsistent in real data ("bounty",
// "trade" lowercase; "CombatBond" camelCase in the same commander's own journal), so matched
// case-insensitively rather than assuming one casing convention.
type redeemVoucherEvent struct {
	Type   string `json:"Type"`
	Amount int64  `json:"Amount"`
}

// User: "missons would be good, rep gained, influence gained, idk something. its just so empty" --
// real MissionCompleted events already carry rich FactionEffects data (confirmed against this
// commander's own real data) this project never parsed before, only ever reading Reward. Faction
// is who actually gave the mission (the real employer); TargetFaction (not modeled here) is who
// the mission was FOR, a different real field. ReputationTrend's real values are confirmed
// "UpGood"/"UpBad"/"DownGood"/"DownBad"/"None" -- the Up/Down prefix is the direction that matters
// for "reputation gained/lost" (Good/Bad describes whether that direction helps or hurts standing
// with THAT faction specifically, not needed for a simple gained-vs-lost count).
type missionFactionInfluence struct {
	SystemAddress int64 `json:"SystemAddress"`
}
type missionFactionEffect struct {
	Faction         string                    `json:"Faction"`
	ReputationTrend string                    `json:"ReputationTrend"`
	Influence       []missionFactionInfluence `json:"Influence"`
}
type missionCompletedEvent struct {
	Faction        string                 `json:"Faction"`
	Reward         int64                  `json:"Reward"`
	FactionEffects []missionFactionEffect `json:"FactionEffects"`
}

// KillerName isn't always a real name -- confirmed against real data (a third-party tester's own
// journal, not this project's own): an NPC with no distinct personal identity (a generic
// Federation Navy patrol ship, not a named pilot) has KillerName as a raw, unresolved
// localization key ("$ShipName_Military_Federation;") with the real display text sitting in the
// sibling KillerName_Localised field ("Federal Navy Ship") instead -- same pattern as other
// enum-ish fields elsewhere in this journal, just not one this project's own real data ever
// happened to exercise. KillerShip also isn't always a ship: the same real tester's data has
// genuine kills by a settlement's defenses ("outpostindustrial") and an on-foot Odyssey NPC
// combatant ("assaultsuitai_class1") -- see the highlights-building code for how that's handled.
type diedEvent struct {
	KillerName          string `json:"KillerName"`
	KillerNameLocalised string `json:"KillerName_Localised"`
	KillerShip          string `json:"KillerShip"`
	// Timestamp isn't part of the real "Died" event JSON -- set manually from the RawEvent's own
	// Timestamp right after unmarshaling. User: "under last session it needs timestamps for the
	// events it shows there".
	Timestamp string `json:"-"`
}

// isRealPlayerKiller: real Elite Dangerous journal convention, confirmed directly against this
// commander's own real Died events -- a KillerName caused by another real commander is literally
// prefixed "Cmdr " (e.g. "Cmdr Toxictrain369"); an NPC killer's name never carries that prefix
// (e.g. "Cai Myson"). User: "prioritize CMDR deaths, since those are players".
func isRealPlayerKiller(killerName string) bool {
	return strings.HasPrefix(killerName, "Cmdr ")
}

type hullDamageEvent struct {
	Health      float64 `json:"Health"`
	PlayerPilot bool    `json:"PlayerPilot"`
	Fighter     bool    `json:"Fighter"`
}

type marketSellEvent struct {
	Type          string `json:"Type"`
	TypeLocalised string `json:"Type_Localised"`
	Count         int64  `json:"Count"`
	TotalSale     int64  `json:"TotalSale"`
	AvgPricePaid  int64  `json:"AvgPricePaid"`
}

type marketBuyEvent struct {
	Count int64 `json:"Count"`
}

type miningRefinedEvent struct {
	Type          string `json:"Type"`
	TypeLocalised string `json:"Type_Localised"`
}

type loadoutEvent struct {
	Ship string `json:"Ship"`
}

type shipyardTypeEvent struct {
	ShipType          string `json:"ShipType"`
	ShipTypeLocalised string `json:"ShipType_Localised"`
	ShipPrice         int64  `json:"ShipPrice"`
}

type fsdJumpEvent struct {
	JumpDist float64 `json:"JumpDist"`
}

type scanDiscoveryEvent struct {
	WasDiscovered *bool `json:"WasDiscovered"`
}

// WingAdd fires once per commander added to your wing, carrying their name directly -- confirmed
// against real data as the right event to count from (vs. WingJoin's "Others" array, which covers
// the same people but only at the moment *you* join/form the wing, not each addition).
type wingAddEvent struct {
	Name string `json:"Name"`
}

type engineerCraftEvent struct {
	Engineer      string `json:"Engineer"`
	BlueprintName string `json:"BlueprintName"`
}

type materialCollectedEvent struct {
	Category      string `json:"Category"`
	Name          string `json:"Name"`
	NameLocalised string `json:"Name_Localised"`
	Count         int64  `json:"Count"`
}

// Powerplay fires periodically (roughly on login/rank change) with the commander's CURRENT
// standing -- Rank/Merits/TimePledged here are snapshots of where things stand right now, not
// deltas. Confirmed against real data these can lag a fraction behind PowerplayRank's own
// rank-up moments by minutes/hours, so "latest by timestamp across every Powerplay-family event"
// (see the loop below) is the right way to find the true current state, not just "last Powerplay
// event seen".
type powerplayEvent struct {
	Power       string `json:"Power"`
	Rank        int    `json:"Rank"`
	Merits      int64  `json:"Merits"`
	TimePledged int64  `json:"TimePledged"` // seconds
}

type powerplayRankEvent struct {
	Power string `json:"Power"`
	Rank  int    `json:"Rank"`
}

// NpcCrewPaidWage fires per-payment for a hired NPC crew member -- Amount is that single
// payment, not a running total.
type npcCrewPaidWageEvent struct {
	NpcCrewName string `json:"NpcCrewName"`
	Amount      int64  `json:"Amount"`
}

type carrierJumpSystemEvent struct {
	StarSystem string `json:"StarSystem"`
}

// CarrierStats is the carrier-management snapshot (fired when checking the carrier services
// screen) -- Finance.CarrierBalance is the carrier's own bank balance, separate from the
// commander's personal wallet (which this project doesn't track at all, journal has no such
// event).
type carrierStatsEvent struct {
	Callsign string `json:"Callsign"`
	Name     string `json:"Name"`
	Finance  struct {
		CarrierBalance int64 `json:"CarrierBalance"`
	} `json:"Finance"`
}

// Docked carries both MarketID and StarSystem/StationName together -- the only real way to turn
// a bare MarketID (all ColonisationConstructionDepot/ColonisationContribution have is the
// numeric ID, no system/station name at all) into something a player would recognize. Confirmed
// against real data: cross-referencing a colonisation depot's MarketID against this commander's
// own real Docked history resolves cleanly to a real system+station name.
type dockedEvent struct {
	MarketID    int64  `json:"MarketID"`
	StarSystem  string `json:"StarSystem"`
	StationName string `json:"StationName"`
	StationType string `json:"StationType"`
}

type colonisationDepotEvent struct {
	MarketID             int64   `json:"MarketID"`
	ConstructionProgress float64 `json:"ConstructionProgress"`
	ConstructionComplete bool    `json:"ConstructionComplete"`
}

type colonisationContributionEvent struct {
	MarketID      int64 `json:"MarketID"`
	Contributions []struct {
		Amount int64 `json:"Amount"`
	} `json:"Contributions"`
}

// Rank/Promotion share the same eight track names as real journal field keys -- confirmed
// against this commander's real Rank event, e.g. {"Combat":3,"Trade":6,"Explore":7,"Soldier":4,
// "Exobiologist":0,"Empire":12,"Federation":6,"CQC":0}. Promotion fires with exactly ONE of these
// present (whichever track just went up), so its own struct keeps every field a pointer to tell
// "not present" apart from "present at rank 0".
type rankEvent struct {
	Combat       int `json:"Combat"`
	Trade        int `json:"Trade"`
	Explore      int `json:"Explore"`
	Soldier      int `json:"Soldier"`
	Exobiologist int `json:"Exobiologist"`
	Empire       int `json:"Empire"`
	Federation   int `json:"Federation"`
	CQC          int `json:"CQC"`
}

type promotionEvent struct {
	Combat       *int `json:"Combat"`
	Trade        *int `json:"Trade"`
	Explore      *int `json:"Explore"`
	Soldier      *int `json:"Soldier"`
	Exobiologist *int `json:"Exobiologist"`
	Empire       *int `json:"Empire"`
	Federation   *int `json:"Federation"`
	CQC          *int `json:"CQC"`
}

// CommitCrime carries EITHER Fine (minor infractions -- the only variant confirmed against this
// commander's real data, all 9 real events) OR Bounty (serious crimes: assault, murder, piracy --
// per Elite Dangerous's documented journal schema, never exercised by this commander's own
// history since they've never committed one). Both fields are modeled here even though only one
// has ever been seen in real data, specifically so a commander who HAS committed bounty-tier
// crimes doesn't get that data silently dropped just because it was never locally verified --
// see BuildRecap's Crime section comment for how this gets flagged as schema-only, not
// data-verified, in the generated page itself.
type commitCrimeEvent struct {
	CrimeType       string `json:"CrimeType"`
	Faction         string `json:"Faction"`
	Fine            int64  `json:"Fine"`
	Bounty          int64  `json:"Bounty"`
	VictimLocalised string `json:"Victim_Localised"`
}

type recapStat struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Sub   string `json:"sub,omitempty"`
	// Danger: user, on seeing a real "Closest call" of 0.2265% displayed as a rounded-off "0%"
	// (indistinguishable from an actual death at a glance): "itd be cool if it showed that with a
	// like, special effect for barely evading". Generic flag (not hardcoded to this one stat) so
	// the template can give a genuinely dramatic reading its own visual treatment.
	Danger bool `json:"danger,omitempty"`
	// DangerExtreme: user, after seeing the full holographic foil shine trigger at the same 10%
	// hull-health bar as the red pulsing outline: "outline is fine at 10 but the full shine
	// shouldnt happen until its super low". A second, stricter tier (<=1%) so the two visual
	// treatments read as genuinely different severities -- Danger alone still gets the red border/
	// pulse at 10%, but only DangerExtreme additionally gets the full cursor-tracking foil shine
	// (currently only set on the same "Closest call" stat as Danger).
	DangerExtreme bool `json:"dangerExtreme,omitempty"`
	// Rare: user, on seeing Danger's glow effect: "make all rare stuff kinda pop up slightly and
	// wobble and shine holographically" -- a second, separate flag for genuinely notable-but-not-
	// dangerous stats (currently just Notable/rare finds), sharing the same holographic hover
	// treatment as Danger cards but without Danger's red tint/pulse.
	Rare bool `json:"rare,omitempty"`
	// EliteTier: 0 = not Elite, 1 = "Elite" itself, 2-6 = the real "Elite I".."Elite V" prestige
	// tiers, 6 being max ("Elite V"). User: "when you have elite it should get a gold border in a
	// rank and each prestige should make it more and max it gets full foil".
	EliteTier int `json:"eliteTier,omitempty"`
	// Breakdown: user, on "Notable/rare finds" (which only ever showed the single most-common
	// type in Sub, discarding the rest): "clicking on stuff like 'notable/rare finds' should bring
	// up a modal listing how many of each rare find" -- then, generalized: "look through them all
	// and think other categories that could use it". Every "top 1 of a map" stat in this file
	// already aggregates the FULL real breakdown internally just to find the winner; this is that
	// same real data, kept instead of thrown away, for a click-to-expand modal. Nil (omitted) for
	// stats with nothing to expand into (a single scalar, not an aggregate).
	Breakdown []recapBreakdownItem `json:"breakdown,omitempty"`
}

type recapBreakdownItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// breakdownByCount/breakdownByValue expand a full nameCount map into every real item (not just
// the top 1 topByCount/topByValue already return elsewhere), sorted the same way. breakdownFromIntMap
// mirrors this for the plainer map[string]int aggregates (rep gains/losses, notable-body counts)
// that never needed a *nameCount's extra Value field.
func breakdownByCount(m map[string]*nameCount) []recapBreakdownItem {
	list := topByCount(m, len(m))
	out := make([]recapBreakdownItem, len(list))
	for i, v := range list {
		out[i] = recapBreakdownItem{Label: v.Name, Value: fmtInt(int64(v.Count))}
	}
	return out
}

func breakdownByValue(m map[string]*nameCount) []recapBreakdownItem {
	list := topByValue(m, len(m))
	out := make([]recapBreakdownItem, len(list))
	for i, v := range list {
		out[i] = recapBreakdownItem{Label: v.Name, Value: formatCr(v.Value)}
	}
	return out
}

func breakdownFromIntMap(m map[string]int) []recapBreakdownItem {
	type kv struct {
		k string
		v int
	}
	list := make([]kv, 0, len(m))
	for k, v := range m {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	out := make([]recapBreakdownItem, len(list))
	for i, e := range list {
		out[i] = recapBreakdownItem{Label: e.k, Value: fmtInt(int64(e.v))}
	}
	return out
}

type recapSection struct {
	Title string      `json:"title"`
	Stats []recapStat `json:"stats"`
}

// recapHighlight: user, "under last session it needs timestamps for the events it shows there" --
// Highlights used to be a flat []string with no time context at all. Timestamp is empty for the
// rare highlight that has no single natural moment to point at (the "...and N earlier deaths"
// summary line).
type recapHighlight struct {
	Text      string `json:"text"`
	Timestamp string `json:"timestamp,omitempty"`
}

type recapData struct {
	Commander   string `json:"commander"`
	GeneratedAt string `json:"generatedAt"`
	// Session is the most recent play session's own recap (user: "add a recent session recap ...
	// shows best things of your last journal"), same recapData shape nested inside itself, reusing
	// every stat this all-time recap already computes rather than a second parallel calculation.
	// Nil when no real LoadGame event exists yet to anchor a session boundary from.
	Session    *sessionRecapData `json:"session,omitempty"`
	Sections   []recapSection    `json:"sections"`
	Highlights []recapHighlight  `json:"highlights"`
}

type nameCount struct {
	Name  string
	Count int
	Value int64
}

func topByCount(m map[string]*nameCount, n int) []*nameCount {
	list := make([]*nameCount, 0, len(m))
	for _, v := range m {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Count > list[j].Count })
	if len(list) > n {
		list = list[:n]
	}
	return list
}

func formatCr(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmtInt(n)
	if neg {
		s = "-" + s
	}
	return s + " cr"
}

// Same thousands-separator convention as the rest of this project's JS (toLocaleString('en-US')),
// done in Go since this runs at generation time, not in the browser.
func fmtInt(n int64) string {
	s := ""
	digits := []byte{}
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if len(digits) == 0 {
		return "0"
	}
	for i, d := range digits {
		if i > 0 && i%3 == 0 {
			s = "," + s
		}
		s = string(d) + s
	}
	return s
}

// topByValue mirrors topByCount but ranks by accumulated credit Value instead of event Count --
// used where "most" means "most paid/earned", not "happened most often" (e.g. NPC crew wages).
func topByValue(m map[string]*nameCount, n int) []*nameCount {
	list := make([]*nameCount, 0, len(m))
	for _, v := range m {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Value > list[j].Value })
	if len(list) > n {
		list = list[:n]
	}
	return list
}

// Pilots Federation rank ladders (14 levels, 0-13) plus the two superpower Navy ladders (15
// levels, 0-14). These titles are NOT present anywhere in the journal itself -- Rank/Promotion
// only ever give the numeric level -- so unlike everything else in this file, they can't be
// grounded against this commander's own data. The base 9 (0-8, up to plain "Elite") were sourced
// and cross-checked against multiple independent community references (elite-dangerous.fandom.com,
// the Player Journal manual appendix, and direct confirmation of the Exobiologist ladder via real
// player discussion threads using these exact title names).
//
// The 5 further "Elite" prestige tiers (indices 9-13, added with Odyssey) were a real gap this
// project had until a real bug report: a commander at Elite I-V got an empty title (rankTitle
// returning "" for any level > 8) and the "Rank" card silently fell back to showing the raw
// "level/max" fraction instead -- with `max` itself hardcoded to 8, so the fraction looked
// visibly broken (level exceeding its own stated max) rather than just unlabeled. Confirmed the
// numeric encoding is a plain linear continuation (9=Elite I ... 13=Elite V, same simple
// "Elite N" naming for all 6 of these ladders, no per-track variation for the prestige tiers
// specifically) against EDDiscovery/EliteDangerousCore's Ranks.cs (Apache 2.0 licensed,
// https://github.com/EDDiscovery/EliteDangerousCore -- real production code parsing this exact
// same journal Rank field), cross-checked against a second, independent source confirming
// exactly 5 Elite prestige tiers exist (a Frontier forums thread on the topic) before trusting
// it. Federation/Empire have no prestige tiers -- confirmed by the same EDDiscovery source
// (their rank enums stop at 14/King and 14/Admiral with nothing beyond), consistent with this
// project's pre-existing 0-14 handling for those two, left unchanged.
var pilotsFederationRankTitles = map[string][]string{
	"Combat": {"Harmless", "Mostly Harmless", "Novice", "Competent", "Expert", "Master", "Dangerous", "Deadly",
		"Elite", "Elite I", "Elite II", "Elite III", "Elite IV", "Elite V"},
	"Trade": {"Penniless", "Mostly Penniless", "Peddler", "Dealer", "Merchant", "Broker", "Entrepreneur", "Tycoon",
		"Elite", "Elite I", "Elite II", "Elite III", "Elite IV", "Elite V"},
	"Explore": {"Aimless", "Mostly Aimless", "Scout", "Surveyor", "Trailblazer", "Pathfinder", "Ranger", "Pioneer",
		"Elite", "Elite I", "Elite II", "Elite III", "Elite IV", "Elite V"},
	"CQC": {"Helpless", "Mostly Helpless", "Amateur", "Semi Professional", "Professional", "Champion", "Hero", "Legend",
		"Elite", "Elite I", "Elite II", "Elite III", "Elite IV", "Elite V"},
	"Soldier": {"Defenceless", "Mostly Defenceless", "Rookie", "Soldier", "Gunslinger", "Warrior", "Gladiator", "Deadeye",
		"Elite", "Elite I", "Elite II", "Elite III", "Elite IV", "Elite V"},
	"Exobiologist": {"Directionless", "Mostly Directionless", "Compiler", "Collector", "Cataloguer", "Taxonomist", "Ecologist", "Geneticist",
		"Elite", "Elite I", "Elite II", "Elite III", "Elite IV", "Elite V"},
}
var federationRankTitles = []string{"None", "Recruit", "Cadet", "Midshipman", "Petty Officer", "Chief Petty Officer", "Warrant Officer", "Ensign", "Lieutenant", "Lieutenant Commander", "Post Commander", "Post Captain", "Rear Admiral", "Vice Admiral", "Admiral"}
var empireRankTitles = []string{"None", "Outsider", "Serf", "Master", "Squire", "Knight", "Lord", "Baron", "Viscount", "Count", "Earl", "Marquis", "Duke", "Prince", "King"}

// rankTrackLabel/rankTitle: "Soldier" is the raw journal field name for what the game's own UI
// calls the "Mercenary" rank -- the label shown to the player uses the in-game name, while the
// title-lookup keys off the journal's own field name.
var rankTrackLabels = map[string]string{
	"Combat": "Combat rank", "Trade": "Trade rank", "Explore": "Exploration rank",
	"CQC": "CQC rank", "Soldier": "Mercenary rank", "Exobiologist": "Exobiologist rank",
	"Federation": "Federation rank", "Empire": "Empire rank",
}

// promotedTrack picks out whichever single field a real Promotion event actually carries --
// confirmed against real data that exactly one of the eight is ever present per event (e.g.
// {"Exobiologist":4} alone, nothing else).
func promotedTrack(v promotionEvent) (track string, level int) {
	switch {
	case v.Combat != nil:
		return "Combat", *v.Combat
	case v.Trade != nil:
		return "Trade", *v.Trade
	case v.Explore != nil:
		return "Explore", *v.Explore
	case v.CQC != nil:
		return "CQC", *v.CQC
	case v.Soldier != nil:
		return "Soldier", *v.Soldier
	case v.Exobiologist != nil:
		return "Exobiologist", *v.Exobiologist
	case v.Federation != nil:
		return "Federation", *v.Federation
	case v.Empire != nil:
		return "Empire", *v.Empire
	}
	return "", 0
}

func rankTitle(track string, level int) string {
	var ladder []string
	switch track {
	case "Federation":
		ladder = federationRankTitles
	case "Empire":
		ladder = empireRankTitles
	default:
		ladder = pilotsFederationRankTitles[track]
	}
	if level < 0 || level >= len(ladder) {
		return ""
	}
	return ladder[level]
}

// EngineerCraft's BlueprintName is a raw internal key (e.g. "Weapon_Overcharged",
// "PowerPlant_Boosted") -- unlike ship/target names elsewhere in this file, there's no
// _Localised sibling field to prefer, so this is display-only underscore-to-space formatting,
// not a real localization.
func formatBlueprintName(raw string) string {
	return strings.ReplaceAll(raw, "_", " ")
}

// A colonisation construction site's Docked.StationName can be a raw, unresolved localization
// key glued to the real name (confirmed against real data: "$EXT_PANEL_ColonisationShip; Tasman
// Prominence", no separate _Localised field at all) while ordinary stations never have this
// prefix -- strips it when present, leaves ordinary names untouched.
var stationKeyPrefix = regexp.MustCompile(`^\$[^;]+;\s*`)

func cleanStationName(raw string) string {
	return stationKeyPrefix.ReplaceAllString(raw, "")
}

// CommitCrime's CrimeType arrives lowerCamelCase with no localised display form at all (confirmed
// against real data: "collidedAtSpeedInNoFireZone", "dockingMinorBlockingAirlock" -- unlike most
// other enum-ish fields in this journal, there's no "_Localised" sibling to fall back to), so this
// is a generic camelCase splitter rather than a lookup table, since a lookup table would need a
// manually-maintained entry per crime type and this project has favored deriving display text
// from structure over hand-maintained/guessable translation tables (see ship-name/variant-name
// handling elsewhere in this file for the same reasoning).
var camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func formatCrimeType(raw string) string {
	if raw == "" {
		return "Unknown"
	}
	spaced := camelBoundary.ReplaceAllString(raw, "$1 $2")
	spaced = strings.ReplaceAll(spaced, "_", " ")
	words := strings.Fields(spaced)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// TimePledged (Powerplay) arrives in whole seconds -- converted to days since that's the unit a
// player actually thinks in for "how long have I been pledged".
func formatDays(seconds int64) string {
	days := float64(seconds) / 86400
	return fmt.Sprintf("%.1f days", days)
}

// isSession: true when called from BuildSessionRecap, scoped to a single play session rather
// than the whole career -- suppresses stats that are inherently career-scoped and would just be
// misleading/redundant restated at session granularity (see the "Commander since" guard below;
// the session's own date range is already shown by the session recap's own header).
func BuildRecap(store *Store, isSession bool) recapData {
	shipNames := map[string]string{}
	captureShipName := func(key, localised string) {
		if key != "" && localised != "" {
			shipNames[strings.ToLower(key)] = localised
		}
	}
	// Falls back to a title-cased version of the raw internal key when there's no better name --
	// confirmed against real data this genuinely happens even for ordinary, non-obscure ships
	// (e.g. all 44 of this commander's real "eagle" Bounty kills have no Target_Localised field
	// at all), so showing the raw lowercase key verbatim isn't a hypothetical edge case to skip.
	prettifyKey := func(key string) string {
		if key == "" {
			return "Unknown"
		}
		return strings.ToUpper(key[:1]) + key[1:]
	}
	displayShip := func(key string) string {
		if key == "" {
			return "Unknown"
		}
		if name, ok := shipNames[strings.ToLower(key)]; ok {
			return name
		}
		return prettifyKey(key)
	}

	var (
		totalKills          int
		totalCombatEarnings int64
		killsByTargetShip   = map[string]*nameCount{}
		killsByVictimFac    = map[string]*nameCount{}
		killsByOwnShip      = map[string]*nameCount{}
		biggestBounty       int64
		biggestBountyTarget string

		timesKilled  int
		realKillers  []diedEvent
		timesIntdctd int

		// "Closest to death" -- user: "if its revealed maybe a closest to death would be cool,
		// lowest health 3% or something". Real journal event "HullDamage" (Health 0.0-1.0,
		// PlayerPilot bool) -- gated to PlayerPilot true so an NPC crew member's or a multicrew
		// captain's own hull damage doesn't get misattributed as the commander's own close call.
		haveHullDamage   bool
		lowestHullHealth float64
		lowestHullAt     string
		lowestHullSystem string

		totalTradeProfit int64
		tradeByCommodity = map[string]*nameCount{}
		totalUnitsSold   int64
		totalUnitsBought int64
		biggestSale      int64
		biggestSaleGood  string

		totalRefined int
		miningByMat  = map[string]*nameCount{}

		shipsPurchased int
		totalShipSpend int64

		missionsCompleted  int
		missionsAccepted   int
		totalMissionReward int64

		voucherBountyBonds int64 // RedeemVoucher Type "bounty"/"CombatBond" -- real credited money, see BuildRecap's Career Earnings comment for why this isn't the same as Combat's "Bounty"/"FactionKillBond" earned-but-not-yet-collected totals
		voucherTrade       int64 // RedeemVoucher Type "trade" -- a real, separate mechanic from direct MarketSell
		voucherOther       int64 // RedeemVoucher Type "settlement"/"scannable" -- rare, real if present

		fsdJumps            int
		totalLightYears     float64
		bodiesScanned       int
		firstDiscoveries    int
		fullSurfaceScans    int
		explorationEarnings int64
		explorationSales    int

		wingmateCounts   = map[string]*nameCount{}
		wingmateLatestTS = map[string]string{}

		crimeCount         int
		totalFines         int64
		totalBountiesOwed  int64
		crimeTypeCounts    = map[string]*nameCount{}
		crimeFactionCounts = map[string]*nameCount{}

		firstTimestamp string
		lastTimestamp  string

		totalCrafts     int
		engineerCounts  = map[string]*nameCount{}
		blueprintCounts = map[string]*nameCount{}

		totalMaterials   int
		materialCounts   = map[string]*nameCount{}
		rawCollected     int
		encodedCollected int
		manufCollected   int

		pledgedPower         string
		currentPPRank        int
		currentPPMerits      int64
		currentPPTimePledged int64
		latestPPTimestamp    string
		meritEvents          int
		totalMeritsGained    int64

		// User: "powerplay info would be useful as well, merits gained, influence pushed, etc" --
		// merits already tracked above; "influence pushed" is the real separate Powerplay mechanic
		// of delivering commodities to reinforce/expand your power's control of a system (real
		// journal events PowerplayDeliver/PowerplayCollect -- confirmed present in this commander's
		// own real data, if rarely used: 1 of each on record).
		totalPowerplayDelivered int64
		powerplayDeliverEvents  int
		totalPowerplayCollected int64

		crewWages = map[string]*nameCount{}

		carrierJumps         int
		carrierDestSystems   = map[string]bool{}
		carrierName          string
		carrierCallsign      string
		carrierBalance       int64
		latestCarrierStatsTS string

		dockedStations    = map[int64]struct{ Station, System, Type string }{}
		stationVisits     = map[int64]*nameCount{}  // keyed by MarketID, non-carrier stations only
		stationTypeCounts = map[string]*nameCount{} // real StationType per real Docked event -- user: "stations is basically empty, itd be nice for more"

		// Missions: real FactionEffects data every "MissionCompleted" event already carries but
		// this project never parsed -- user: "missons would be good, rep gained, influence
		// gained". repGainedFactions/repLostFactions counted by real ReputationTrend
		// ("UpGood"/"DownGood" etc, confirmed against this commander's own real data); employerCounts
		// is who actually gave the mission (real Faction field, not TargetFaction, which is who the
		// mission was FOR, a different thing).
		repGainedFactions = map[string]int{}
		repLostFactions   = map[string]int{}
		employerCounts    = map[string]*nameCount{}
		influencedSystems = map[int64]bool{}
		depotProgress     = map[int64]struct {
			Progress  float64
			Complete  bool
			Timestamp string
		}{}
		contributionEvents int
		contributionUnits  int64
		systemsClaimed     []string

		latestRank           rankEvent
		haveRank             bool
		latestRankTS         string
		mostRecentPromoTrack string
		mostRecentPromoLevel int
		mostRecentPromoTS    string
	)

	currentShip := ""

	for _, e := range store.RawEvents {
		// Timestamps are ISO 8601 (e.g. "2026-08-03T04:55:11Z"), which sorts correctly as a plain
		// string -- no need to parse every event just to find the career's first/last activity.
		if firstTimestamp == "" || e.Timestamp < firstTimestamp {
			firstTimestamp = e.Timestamp
		}
		if e.Timestamp > lastTimestamp {
			lastTimestamp = e.Timestamp
		}
		switch e.Event {
		case "Loadout":
			var v loadoutEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Ship != "" {
				currentShip = v.Ship
			}
		case "ShipyardBuy", "ShipyardSwap", "ShipyardNew":
			var v shipyardTypeEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				captureShipName(v.ShipType, v.ShipTypeLocalised)
				if v.ShipType != "" {
					currentShip = v.ShipType
				}
				if e.Event == "ShipyardBuy" {
					shipsPurchased++
					totalShipSpend += v.ShipPrice
				}
			}
		case "Bounty":
			var v bountyEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				totalKills++
				totalCombatEarnings += v.TotalReward
				target := v.TargetLocalised
				if target == "" {
					target = prettifyKey(v.Target)
				}
				if target != "" {
					if killsByTargetShip[target] == nil {
						killsByTargetShip[target] = &nameCount{Name: target}
					}
					killsByTargetShip[target].Count++
				}
				if v.VictimFaction != "" {
					if killsByVictimFac[v.VictimFaction] == nil {
						killsByVictimFac[v.VictimFaction] = &nameCount{Name: v.VictimFaction}
					}
					killsByVictimFac[v.VictimFaction].Count++
					killsByVictimFac[v.VictimFaction].Value += v.TotalReward
				}
				own := displayShip(currentShip)
				if killsByOwnShip[own] == nil {
					killsByOwnShip[own] = &nameCount{Name: own}
				}
				killsByOwnShip[own].Count++
				if v.TotalReward > biggestBounty {
					biggestBounty = v.TotalReward
					biggestBountyTarget = target
				}
			}
		case "FactionKillBond":
			var v factionKillBondEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				totalKills++
				totalCombatEarnings += v.Reward
				if v.VictimFaction != "" {
					if killsByVictimFac[v.VictimFaction] == nil {
						killsByVictimFac[v.VictimFaction] = &nameCount{Name: v.VictimFaction}
					}
					killsByVictimFac[v.VictimFaction].Count++
					killsByVictimFac[v.VictimFaction].Value += v.Reward
				}
				own := displayShip(currentShip)
				if killsByOwnShip[own] == nil {
					killsByOwnShip[own] = &nameCount{Name: own}
				}
				killsByOwnShip[own].Count++
			}
		case "Died":
			timesKilled++
			var v diedEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.KillerName != "" {
				v.Timestamp = e.Timestamp
				realKillers = append(realKillers, v)
			}
		case "Interdicted":
			timesIntdctd++
		case "HullDamage":
			var v hullDamageEvent
			// Real bug, caught in verification: this commander's own real minimum came back
			// exactly 0% -- in real Elite Dangerous, hull hitting 0% IS the ship's destruction
			// moment, not a close call that was survived (confirmed: this commander's own real
			// "Times destroyed" count is 8, consistent with genuine deaths producing 0% readings).
			// "Closest to death" is meant to be a scrape you lived through, so > 0 excludes the
			// destructions themselves and surfaces the real lowest SURVIVED reading instead.
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.PlayerPilot && v.Health > 0 && (!haveHullDamage || v.Health < lowestHullHealth) {
				haveHullDamage, lowestHullHealth, lowestHullAt, lowestHullSystem = true, v.Health, e.Timestamp, e.SystemName
			}
		case "MarketSell":
			var v marketSellEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				profit := v.TotalSale - v.AvgPricePaid*v.Count
				totalTradeProfit += profit
				totalUnitsSold += v.Count
				good := v.TypeLocalised
				if good == "" {
					good = v.Type
				}
				if good != "" {
					if tradeByCommodity[good] == nil {
						tradeByCommodity[good] = &nameCount{Name: good}
					}
					tradeByCommodity[good].Value += profit
				}
				if v.TotalSale > biggestSale {
					biggestSale = v.TotalSale
					biggestSaleGood = good
				}
			}
		case "MarketBuy":
			var v marketBuyEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				totalUnitsBought += v.Count
			}
		case "MiningRefined":
			var v miningRefinedEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				totalRefined++
				mat := v.TypeLocalised
				if mat == "" {
					mat = v.Type
				}
				if mat != "" {
					if miningByMat[mat] == nil {
						miningByMat[mat] = &nameCount{Name: mat}
					}
					miningByMat[mat].Count++
				}
			}
		case "MissionCompleted":
			missionsCompleted++
			var v missionCompletedEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				totalMissionReward += v.Reward
				if v.Faction != "" {
					if employerCounts[v.Faction] == nil {
						employerCounts[v.Faction] = &nameCount{Name: v.Faction}
					}
					employerCounts[v.Faction].Count++
				}
				for _, fe := range v.FactionEffects {
					switch {
					case strings.HasPrefix(fe.ReputationTrend, "Up"):
						repGainedFactions[fe.Faction]++
					case strings.HasPrefix(fe.ReputationTrend, "Down"):
						repLostFactions[fe.Faction]++
					}
					for _, inf := range fe.Influence {
						influencedSystems[inf.SystemAddress] = true
					}
				}
			}
		case "MissionAccepted":
			missionsAccepted++
		case "RedeemVoucher":
			var v redeemVoucherEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				switch strings.ToLower(v.Type) {
				case "bounty", "combatbond":
					voucherBountyBonds += v.Amount
				case "trade":
					voucherTrade += v.Amount
				default: // "settlement", "scannable", or anything future/unrecognized -- still real money, not dropped
					voucherOther += v.Amount
				}
			}
		case "CommitCrime":
			var v commitCrimeEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				crimeCount++
				totalFines += v.Fine
				totalBountiesOwed += v.Bounty
				crimeLabel := formatCrimeType(v.CrimeType)
				if crimeTypeCounts[crimeLabel] == nil {
					crimeTypeCounts[crimeLabel] = &nameCount{Name: crimeLabel}
				}
				crimeTypeCounts[crimeLabel].Count++
				if v.Faction != "" {
					if crimeFactionCounts[v.Faction] == nil {
						crimeFactionCounts[v.Faction] = &nameCount{Name: v.Faction}
					}
					crimeFactionCounts[v.Faction].Count++
				}
			}
		case "FSDJump":
			var v fsdJumpEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				fsdJumps++
				totalLightYears += v.JumpDist
			}
		case "Scan":
			bodiesScanned++
			var v scanDiscoveryEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.WasDiscovered != nil && !*v.WasDiscovered {
				firstDiscoveries++
			}
		case "SAAScanComplete":
			fullSurfaceScans++
		case "MultiSellExplorationData":
			var v multiSellEntry
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.TotalEarnings != nil {
				explorationEarnings += *v.TotalEarnings
				explorationSales++
			}
		case "WingAdd":
			var v wingAddEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Name != "" {
				if wingmateCounts[v.Name] == nil {
					wingmateCounts[v.Name] = &nameCount{Name: v.Name}
				}
				wingmateCounts[v.Name].Count++
				if e.Timestamp > wingmateLatestTS[v.Name] {
					wingmateLatestTS[v.Name] = e.Timestamp
				}
			}
		case "EngineerCraft":
			var v engineerCraftEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Engineer != "" {
				totalCrafts++
				if engineerCounts[v.Engineer] == nil {
					engineerCounts[v.Engineer] = &nameCount{Name: v.Engineer}
				}
				engineerCounts[v.Engineer].Count++
				if v.BlueprintName != "" {
					display := formatBlueprintName(v.BlueprintName)
					if blueprintCounts[display] == nil {
						blueprintCounts[display] = &nameCount{Name: display}
					}
					blueprintCounts[display].Count++
				}
			}
		case "MaterialCollected":
			var v materialCollectedEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				totalMaterials++
				switch v.Category {
				case "Raw":
					rawCollected++
				case "Encoded":
					encodedCollected++
				case "Manufactured":
					manufCollected++
				}
				// Raw materials genuinely have no Name_Localised at all (confirmed against real
				// data -- every real "Raw" MaterialCollected line here is just {"Name":"iron",...}
				// with nothing else) -- same title-case-the-raw-key fallback used for ship names.
				name := v.NameLocalised
				if name == "" {
					name = prettifyKey(v.Name)
				}
				if name != "" {
					if materialCounts[name] == nil {
						materialCounts[name] = &nameCount{Name: name}
					}
					materialCounts[name].Count += int(v.Count)
				}
			}
		case "Powerplay":
			var v powerplayEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Power != "" {
				if latestPPTimestamp == "" || e.Timestamp >= latestPPTimestamp {
					latestPPTimestamp = e.Timestamp
					pledgedPower = v.Power
					currentPPRank = v.Rank
					currentPPMerits = v.Merits
					currentPPTimePledged = v.TimePledged
				}
			}
		case "PowerplayRank":
			// PowerplayRank fires exactly at a rank-up, sometimes ahead of the next periodic
			// "Powerplay" snapshot (confirmed against real data) -- so the true current rank is
			// whichever of the two events is more recent, not just the last "Powerplay" seen.
			var v powerplayRankEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Power != "" {
				if latestPPTimestamp == "" || e.Timestamp >= latestPPTimestamp {
					latestPPTimestamp = e.Timestamp
					pledgedPower = v.Power
					currentPPRank = v.Rank
				}
			}
		case "PowerplayMerits":
			var v struct {
				MeritsGained int64 `json:"MeritsGained"`
			}
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				meritEvents++
				totalMeritsGained += v.MeritsGained
			}
		case "PowerplayDeliver":
			var v struct {
				Count int64 `json:"Count"`
			}
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				powerplayDeliverEvents++
				totalPowerplayDelivered += v.Count
			}
		case "PowerplayCollect":
			var v struct {
				Count int64 `json:"Count"`
			}
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				totalPowerplayCollected += v.Count
			}
		case "NpcCrewPaidWage":
			var v npcCrewPaidWageEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.NpcCrewName != "" {
				if crewWages[v.NpcCrewName] == nil {
					crewWages[v.NpcCrewName] = &nameCount{Name: v.NpcCrewName}
				}
				crewWages[v.NpcCrewName].Count++
				crewWages[v.NpcCrewName].Value += v.Amount
			}
		case "CarrierJump":
			carrierJumps++
			var v carrierJumpSystemEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.StarSystem != "" {
				carrierDestSystems[v.StarSystem] = true
			}
		case "CarrierStats":
			var v carrierStatsEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.Name != "" {
				if latestCarrierStatsTS == "" || e.Timestamp >= latestCarrierStatsTS {
					latestCarrierStatsTS = e.Timestamp
					carrierName = v.Name
					carrierCallsign = v.Callsign
					carrierBalance = v.Finance.CarrierBalance
				}
			}
		case "Docked":
			var v dockedEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.MarketID != 0 {
				dockedStations[v.MarketID] = struct{ Station, System, Type string }{
					Station: cleanStationName(v.StationName), System: v.StarSystem, Type: v.StationType,
				}
				if v.StationType != "FleetCarrier" {
					if stationVisits[v.MarketID] == nil {
						stationVisits[v.MarketID] = &nameCount{Name: cleanStationName(v.StationName)}
					}
					stationVisits[v.MarketID].Count++
					if v.StationType != "" {
						if stationTypeCounts[v.StationType] == nil {
							stationTypeCounts[v.StationType] = &nameCount{Name: v.StationType}
						}
						stationTypeCounts[v.StationType].Count++
					}
				}
			}
		case "ColonisationConstructionDepot":
			var v colonisationDepotEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.MarketID != 0 {
				existing, ok := depotProgress[v.MarketID]
				if !ok || e.Timestamp >= existing.Timestamp {
					depotProgress[v.MarketID] = struct {
						Progress  float64
						Complete  bool
						Timestamp string
					}{Progress: v.ConstructionProgress, Complete: v.ConstructionComplete, Timestamp: e.Timestamp}
				}
			}
		case "ColonisationContribution":
			var v colonisationContributionEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				contributionEvents++
				for _, c := range v.Contributions {
					contributionUnits += c.Amount
				}
			}
		case "ColonisationSystemClaim":
			var v colonisationClaimEntry
			if json.Unmarshal([]byte(e.Raw), &v) == nil && v.StarSystem != "" {
				systemsClaimed = append(systemsClaimed, v.StarSystem)
			}
		case "Rank":
			var v rankEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				if !haveRank || e.Timestamp >= latestRankTS {
					latestRankTS = e.Timestamp
					latestRank = v
					haveRank = true
				}
			}
		case "Promotion":
			var v promotionEvent
			if json.Unmarshal([]byte(e.Raw), &v) == nil {
				track, level := promotedTrack(v)
				if track != "" && (mostRecentPromoTS == "" || e.Timestamp >= mostRecentPromoTS) {
					mostRecentPromoTS = e.Timestamp
					mostRecentPromoTrack = track
					mostRecentPromoLevel = level
				}
			}
		}
	}

	sections := []recapSection{}

	// Furthest system visited and total systems visited come from the already-parsed Store, not
	// RawEvents -- Store.Systems' X/Y/Z is populated straight off FSDJump/Location's own StarPos
	// (see parse.go's onJumpOrLocation), and Sol sits at real galactic coordinate (0,0,0), so
	// distance-from-Sol is a plain Euclidean distance with no extra lookup needed.
	systemsVisited := len(store.Systems)
	var furthestName string
	var furthestDist float64
	for _, sys := range store.Systems {
		d := math.Sqrt(sys.X*sys.X + sys.Y*sys.Y + sys.Z*sys.Z)
		if d > furthestDist {
			furthestDist = d
			furthestName = sys.Name
		}
	}

	// Reuses viewer.go's own notable-body classification (classifyRareStarType/notableBodyTypes)
	// rather than re-deriving it -- same package, same rules, no risk of the two pages disagreeing
	// on what counts as "notable".
	notableCounts := map[string]int{}
	for _, sys := range store.Systems {
		for _, st := range sys.Stars {
			if label := classifyRareStarType(st.Type); label != "" {
				notableCounts[label]++
			}
		}
		for _, pl := range sys.Planets {
			if label := notableBodyTypes[pl.Type]; label != "" {
				notableCounts[label]++
			}
		}
	}
	var totalNotable int
	var topNotableLabel string
	var topNotableCount int
	for label, n := range notableCounts {
		totalNotable += n
		if n > topNotableCount {
			topNotableCount = n
			topNotableLabel = label
		}
	}

	// Same value formula the main viewer uses (ComputeFloraValues, reconcile.go) -- sold+unclaimed,
	// excluding lost, matching viewer.go's own bioValue aggregation exactly.
	var bioValue int64
	var bioSoldValue int64 // subset of bioValue that's actually been sold -- real money received, for Career Earnings below (bioValue itself includes still-unsold/unclaimed data, which isn't money yet)
	for _, fv := range ComputeFloraValues(store) {
		if fv.HasValue && !fv.Lost {
			bioValue += fv.Value
			if fv.Sold {
				bioSoldValue += fv.Value
			}
		}
	}

	// "Cumulative money made" (owner: "i think thats important") -- deliberately NOT just
	// totalCombatEarnings (Bounty/FactionKillBond) added to everything else: those two only
	// record a voucher being EARNED, not real credited money -- confirmed real per the journal
	// manual and cross-checked against this project's own commander's data, where redeemed
	// voucher totals didn't line up with raw earned totals (vouchers are lost entirely if the
	// ship's destroyed before reaching a station to redeem them). Using RedeemVoucher's Amount
	// instead avoids double-counting the same kill's bounty at both the earn and collect points,
	// and doesn't overcount vouchers that were earned but never actually collected. Every other
	// category here (trade, exploration, exobiology, missions) credits instantly with no
	// redemption step, so those ARE safe to use as-is from what's already tracked elsewhere in
	// this function. Deliberately excludes asset liquidation (selling ships/modules/drones back)
	// -- that's recovering value from something already owned, not new income.
	if voucherBountyBonds > 0 || voucherTrade > 0 || voucherOther > 0 || totalTradeProfit > 0 ||
		explorationEarnings > 0 || bioSoldValue > 0 || totalMissionReward > 0 {
		cumulativeMoney := voucherBountyBonds + voucherTrade + voucherOther + totalTradeProfit +
			explorationEarnings + bioSoldValue + totalMissionReward
		earningsStats := []recapStat{
			{Label: "Cumulative money made", Value: formatCr(cumulativeMoney), Sub: "bounties/bonds, trading, exploration, exobiology, and mission rewards actually collected"},
		}
		if voucherBountyBonds > 0 {
			earningsStats = append(earningsStats, recapStat{Label: "Bounties & combat bonds collected", Value: formatCr(voucherBountyBonds)})
		}
		if totalTradeProfit+voucherTrade > 0 {
			earningsStats = append(earningsStats, recapStat{Label: "Trading profit", Value: formatCr(totalTradeProfit + voucherTrade)})
		}
		if explorationEarnings > 0 {
			earningsStats = append(earningsStats, recapStat{Label: "Exploration & Cartographics", Value: formatCr(explorationEarnings)})
		}
		if bioSoldValue > 0 {
			earningsStats = append(earningsStats, recapStat{Label: "Exobiology sales", Value: formatCr(bioSoldValue)})
		}
		if totalMissionReward > 0 {
			earningsStats = append(earningsStats, recapStat{Label: "Mission rewards", Value: formatCr(totalMissionReward)})
		}
		if voucherOther > 0 {
			earningsStats = append(earningsStats, recapStat{Label: "Other vouchers redeemed", Value: formatCr(voucherOther), Sub: "settlement/scannable"})
		}
		sections = append(sections, recapSection{Title: "Career Earnings", Stats: earningsStats})
	}

	// User: "no point showing '0 bodies scanned'" -- every base stat gated on its own real count,
	// same principle as Combat/Trading/Mining above.
	var explorationStats []recapStat
	if systemsVisited > 0 {
		explorationStats = append(explorationStats, recapStat{Label: "Systems visited", Value: fmtInt(int64(systemsVisited))})
	}
	if fsdJumps > 0 {
		explorationStats = append(explorationStats, recapStat{Label: "Light-years travelled", Value: fmt.Sprintf("%.1f LY", totalLightYears), Sub: fmtInt(int64(fsdJumps)) + " FSD jumps"})
	}
	if bodiesScanned > 0 {
		explorationStats = append(explorationStats, recapStat{Label: "Bodies scanned", Value: fmtInt(int64(bodiesScanned)), Sub: fmtInt(int64(firstDiscoveries)) + " first discoveries"})
	}
	if fullSurfaceScans > 0 {
		explorationStats = append(explorationStats, recapStat{Label: "Full surface scans", Value: fmtInt(int64(fullSurfaceScans))})
	}
	if furthestName != "" {
		explorationStats = append(explorationStats, recapStat{Label: "Furthest system visited", Value: furthestName, Sub: fmt.Sprintf("%.1f LY from Sol", furthestDist)})
	}
	if bioValue > 0 {
		explorationStats = append(explorationStats, recapStat{Label: "Presumed bio value", Value: formatCr(bioValue)})
	}
	if explorationSales > 0 {
		explorationStats = append(explorationStats, recapStat{Label: "Cartographics earnings", Value: formatCr(explorationEarnings), Sub: fmtInt(int64(explorationSales)) + " sales"})
	}
	if totalNotable > 0 {
		sub := ""
		if topNotableLabel != "" {
			sub = fmtInt(int64(topNotableCount)) + "x " + topNotableLabel
		}
		explorationStats = append(explorationStats, recapStat{Label: "Notable/rare finds", Value: fmtInt(int64(totalNotable)), Sub: sub, Rare: true, Breakdown: breakdownFromIntMap(notableCounts)})
	}
	// "Commander since" is inherently a CAREER stat (first-ever real event timestamp) -- skipped
	// for the session-scoped recap, where it would just restate the session's own start date
	// (already shown by the session recap's own header) as if it meant "been playing since then".
	if !isSession {
		if first, err1 := time.Parse(time.RFC3339, firstTimestamp); err1 == nil {
			sub := ""
			if last, err2 := time.Parse(time.RFC3339, lastTimestamp); err2 == nil {
				days := int(last.Sub(first).Hours()/24) + 1
				sub = fmtInt(int64(days)) + " days of logged activity"
			}
			explorationStats = append(explorationStats, recapStat{Label: "Commander since", Value: first.Format("Jan 2, 2006"), Sub: sub})
		}
	}
	if len(explorationStats) > 0 {
		sections = append(sections, recapSection{Title: "Exploration", Stats: explorationStats})
	}

	// User: "recap only needs to show changes not everything, like its what changed" -- Rank is a
	// CURRENT-STANDING snapshot (whichever Rank/Progress event happened to be latest within the
	// window), not a delta of what happened this session, so it's skipped entirely for the
	// session-scoped recap -- same reasoning as "Commander since" above.
	//
	// Only tracks the commander has actually made progress in get a card -- a rank-0 track is
	// just the default starting state for everyone, not a real achievement to report (same
	// "don't force a boring stat" standard as the rest of this file). Federation/Empire go last
	// since they're the two tracks with real name recognition beyond the Pilots Federation ones.
	if haveRank && !isSession {
		var rankStats []recapStat
		trackOrder := []string{"Combat", "Trade", "Explore", "CQC", "Soldier", "Exobiologist", "Federation", "Empire"}
		levelByTrack := map[string]int{
			"Combat": latestRank.Combat, "Trade": latestRank.Trade, "Explore": latestRank.Explore,
			"CQC": latestRank.CQC, "Soldier": latestRank.Soldier, "Exobiologist": latestRank.Exobiologist,
			"Federation": latestRank.Federation, "Empire": latestRank.Empire,
		}
		maxLevel := map[string]int{"Federation": 14, "Empire": 14}
		var levelSum, maxSum int
		for _, track := range trackOrder {
			level := levelByTrack[track]
			if level <= 0 {
				continue
			}
			max := 13 // Pilots Federation ladders top out at Elite V (index 13), see pilotsFederationRankTitles' comment
			if m, ok := maxLevel[track]; ok {
				max = m
			}
			levelSum += level
			maxSum += max
			title := rankTitle(track, level)
			sub := fmtInt(int64(level)) + "/" + fmtInt(int64(max))
			value := title
			if value == "" {
				value = sub // title lookup failure (shouldn't happen for a valid 0-14/0-13 level) falls back to the raw progress instead of an empty card
			}
			stat := recapStat{Label: rankTrackLabels[track], Value: value, Sub: sub}
			// User: "when you have elite it should get a gold border in a rank and each prestige
			// should make it more and max it gets full foil" -- real mechanic (confirmed against
			// this project's own already-verified pilotsFederationRankTitles ladder): level 8 is
			// "Elite" itself, 9-13 are the real post-launch prestige tiers "Elite I" through
			// "Elite V" (13 = max). Only the 6 real Pilots Federation ladder tracks ever reach
			// Elite at all -- Federation/Empire (the `maxLevel` override map below) cap at
			// "Admiral"/"King" and never use "Elite" terminology, so explicitly excluded here.
			if _, isFedOrEmpire := maxLevel[track]; !isFedOrEmpire && level >= 8 {
				stat.EliteTier = level - 7 // 8 -> 1 ("Elite"), 13 -> 6 ("Elite V", max prestige)
			}
			rankStats = append(rankStats, stat)
		}
		if len(rankStats) > 0 {
			// Owner request: "add an overall rank% too? %to all max" -- one combined headline
			// stat across every track shown above (same tracks, same maxes), not a 9th separate
			// card. Placed first since it's the summary of everything that follows.
			overallPct := 0
			if maxSum > 0 {
				overallPct = levelSum * 100 / maxSum
			}
			rankStats = append([]recapStat{{
				Label: "Overall rank progress", Value: fmtInt(int64(overallPct)) + "%",
				Sub: fmtInt(int64(levelSum)) + "/" + fmtInt(int64(maxSum)) + " combined levels across " + fmtInt(int64(len(rankStats))) + " ranks",
			}}, rankStats...)
			sections = append(sections, recapSection{Title: "Rank", Stats: rankStats})
		}
	}

	// User: "recap only needs to show changes not everything, like its what changed" and later
	// "recap should only show something if somethings happened" -- every base stat in every
	// section (not just whole sections, which were already gated) now only appears when its own
	// real count/value is actually nonzero, so a quiet session (or, in principle, an all-time
	// recap for a very new commander) doesn't pad itself out with a wall of real-but-empty "0"
	// stats.
	var combatStats []recapStat
	if totalKills > 0 {
		combatStats = append(combatStats, recapStat{Label: "Total kills", Value: fmtInt(int64(totalKills))})
	}
	if totalCombatEarnings > 0 {
		combatStats = append(combatStats, recapStat{Label: "Combat earnings", Value: formatCr(totalCombatEarnings)})
	}
	if top := topByCount(killsByTargetShip, 1); len(top) > 0 {
		combatStats = append(combatStats, recapStat{Label: "Favourite target", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " destroyed", Breakdown: breakdownByCount(killsByTargetShip)})
	}
	if top := topByCount(killsByOwnShip, 1); len(top) > 0 {
		combatStats = append(combatStats, recapStat{Label: "Favourite combat ship", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " kills flying it", Breakdown: breakdownByCount(killsByOwnShip)})
	}
	if len(killsByVictimFac) > 0 {
		list := make([]*nameCount, 0, len(killsByVictimFac))
		for _, v := range killsByVictimFac {
			list = append(list, v)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Count > list[j].Count })
		combatStats = append(combatStats, recapStat{Label: "Biggest rival faction", Value: list[0].Name, Sub: fmtInt(int64(list[0].Count)) + " of their ships destroyed", Breakdown: breakdownByCount(killsByVictimFac)})
	}
	if biggestBounty > 0 {
		combatStats = append(combatStats, recapStat{Label: "Biggest single bounty", Value: formatCr(biggestBounty), Sub: biggestBountyTarget})
	}
	if timesKilled > 0 {
		combatStats = append(combatStats, recapStat{Label: "Times destroyed", Value: fmtInt(int64(timesKilled))})
	}
	if timesIntdctd > 0 {
		combatStats = append(combatStats, recapStat{Label: "Times interdicted", Value: fmtInt(int64(timesIntdctd))})
	}
	if haveHullDamage {
		sub := lowestHullSystem
		if len(lowestHullAt) >= 10 {
			date := lowestHullAt[:10]
			if sub != "" {
				sub = date + " -- " + sub
			} else {
				sub = date
			}
		}
		combatStats = append(combatStats, recapStat{
			Label:  "Closest call (lowest hull health)",
			Value:  fmt.Sprintf("%.4f%%", lowestHullHealth*100),
			Sub:    sub,
			Danger: lowestHullHealth <= 0.10, DangerExtreme: lowestHullHealth <= 0.01,
		})
	}
	if len(combatStats) > 0 {
		sections = append(sections, recapSection{Title: "Combat", Stats: combatStats})
	}

	// Deliberately not built if this commander has never committed a crime -- same "don't force a
	// hypothetical" standard as Wing below. Fine and Bounty are two different things (minor traffic-
	// ticket-style infractions vs. an actual bounty on your head for something serious like
	// assault/murder/piracy) so both get their own total rather than being summed together, even
	// though this commander's own real history only ever has Fine-type crimes -- see
	// commitCrimeEvent's own comment for why Bounty is still modeled and shown when present.
	if crimeCount > 0 {
		crimeStats := []recapStat{
			{Label: "Crimes committed", Value: fmtInt(int64(crimeCount))},
		}
		if totalFines > 0 {
			crimeStats = append(crimeStats, recapStat{Label: "Fines paid", Value: formatCr(totalFines)})
		}
		if totalBountiesOwed > 0 {
			crimeStats = append(crimeStats, recapStat{Label: "Bounties incurred", Value: formatCr(totalBountiesOwed)})
		}
		if top := topByCount(crimeTypeCounts, 1); len(top) > 0 {
			crimeStats = append(crimeStats, recapStat{Label: "Most common offence", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " times", Breakdown: breakdownByCount(crimeTypeCounts)})
		}
		if top := topByCount(crimeFactionCounts, 1); len(top) > 0 {
			crimeStats = append(crimeStats, recapStat{Label: "Most trouble with", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " incidents", Breakdown: breakdownByCount(crimeFactionCounts)})
		}
		sections = append(sections, recapSection{Title: "Crime", Stats: crimeStats})
	}

	// WingAdd (fires once per commander added to your wing) is the ground-truth source here,
	// confirmed against real data as more useful than WingJoin's "Others" array for this purpose:
	// same set of people, but counted per-addition rather than only at the moment of forming/
	// joining. Deliberately not built if nobody's ever been in a wing with this commander --
	// same "don't force a hypothetical" standard the rest of this recap holds to.
	if len(wingmateCounts) > 0 {
		wingStats := []recapStat{}
		if top := topByCount(wingmateCounts, 1); len(top) > 0 {
			sessionWord := "sessions"
			if top[0].Count == 1 {
				sessionWord = "session"
			}
			wingStats = append(wingStats, recapStat{Label: "Most frequent wingmate", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " " + sessionWord + " together", Breakdown: breakdownByCount(wingmateCounts)})
		}
		wingStats = append(wingStats, recapStat{Label: "Distinct wingmates", Value: fmtInt(int64(len(wingmateCounts)))})
		sections = append(sections, recapSection{Title: "Wing", Stats: wingStats})
	}

	// Only built if the commander has actually pledged to a power -- real data confirms this
	// commander has (1,038 PowerplayMerits events, 69 Powerplay snapshots), but the field is
	// deliberately gated the same way Wing is, for anyone whose journal has none of this.
	// Real gap, fixed while adding isSession: the section used to gate purely on pledgedPower
	// (only set by the periodic Powerplay/PowerplayRank snapshot events) -- but a real session can
	// easily have merit/delivery ACTIVITY (PowerplayMerits/PowerplayDeliver/PowerplayCollect)
	// without either snapshot event happening to also fire in that same session, which would have
	// hidden real session deltas behind a gate keyed on the wrong signal.
	if pledgedPower != "" || meritEvents > 0 || powerplayDeliverEvents > 0 || totalPowerplayCollected > 0 {
		// User: "recap only needs to show changes not everything, like its what changed" --
		// Pledged to/Powerplay rank/Current merits/Time pledged are all CURRENT-STANDING snapshot
		// values (from the periodic Powerplay event), not session deltas, so skipped for the
		// session-scoped recap -- same reasoning as Rank above. Merits earned/Goods delivered/
		// Goods collected below stay for both: those are real sums of session-scoped events.
		var ppStats []recapStat
		if !isSession {
			ppStats = append(ppStats,
				recapStat{Label: "Pledged to", Value: pledgedPower},
				recapStat{Label: "Powerplay rank", Value: fmtInt(int64(currentPPRank))},
				recapStat{Label: "Current merits", Value: fmtInt(currentPPMerits)},
			)
			if currentPPTimePledged > 0 {
				ppStats = append(ppStats, recapStat{Label: "Time pledged", Value: formatDays(currentPPTimePledged)})
			}
		}
		if meritEvents > 0 {
			ppStats = append(ppStats, recapStat{Label: "Merits earned", Value: fmtInt(totalMeritsGained), Sub: fmtInt(int64(meritEvents)) + " merit-earning actions"})
		}
		if powerplayDeliverEvents > 0 {
			ppStats = append(ppStats, recapStat{Label: "Goods delivered (influence pushed)", Value: fmtInt(totalPowerplayDelivered), Sub: fmtInt(int64(powerplayDeliverEvents)) + " deliveries"})
		}
		if totalPowerplayCollected > 0 {
			ppStats = append(ppStats, recapStat{Label: "Goods collected for delivery", Value: fmtInt(totalPowerplayCollected)})
		}
		// Real bug, fixed: the OUTER gate (pledgedPower != "" || meritEvents > 0 || ...) can pass
		// for a session-scoped call purely because a Powerplay/PowerplayRank snapshot happened to
		// fire that session, even when isSession=true skips the pledge/rank/merits/time-pledged
		// stats AND there was no real merit/delivery/collection activity in that same session --
		// leaving ppStats genuinely empty (nil) while still unconditionally appending the section.
		// A nil Stats slice marshals to JSON `null`, which broke the client's
		// `sections.flatMap(sec => sec.stats)` (a literal null landed in the flattened array,
		// crashing on `stat.danger`) -- confirmed via a real user-reported browser console error
		// ("can't access property 'danger', stat is null") that this project's own jsdom test
		// harness never caught, since the specific session data shape that triggers it (a
		// snapshot-only Powerplay event with zero session-scoped merit/delivery/collection
		// activity) wasn't present in this project's own test data. Same fix pattern as every
		// other section in this file: only append if there's real content.
		if len(ppStats) > 0 {
			sections = append(sections, recapSection{Title: "Powerplay", Stats: ppStats})
		}
	}

	var tradeStats []recapStat
	if totalTradeProfit > 0 {
		tradeStats = append(tradeStats, recapStat{Label: "Total trade profit", Value: formatCr(totalTradeProfit)})
	}
	if totalUnitsSold > 0 {
		tradeStats = append(tradeStats, recapStat{Label: "Units sold", Value: fmtInt(totalUnitsSold)})
	}
	if totalUnitsBought > 0 {
		tradeStats = append(tradeStats, recapStat{Label: "Units bought", Value: fmtInt(totalUnitsBought)})
	}
	{
		list := make([]*nameCount, 0, len(tradeByCommodity))
		for _, v := range tradeByCommodity {
			list = append(list, v)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Value > list[j].Value })
		if len(list) > 0 {
			tradeStats = append(tradeStats, recapStat{Label: "Most profitable commodity", Value: list[0].Name, Sub: formatCr(list[0].Value) + " profit", Breakdown: breakdownByValue(tradeByCommodity)})
		}
	}
	if biggestSale > 0 {
		tradeStats = append(tradeStats, recapStat{Label: "Biggest single sale", Value: formatCr(biggestSale), Sub: biggestSaleGood})
	}
	if len(tradeStats) > 0 {
		sections = append(sections, recapSection{Title: "Trading", Stats: tradeStats})
	}

	var miningStats []recapStat
	if totalRefined > 0 {
		miningStats = append(miningStats, recapStat{Label: "Total refined", Value: fmtInt(int64(totalRefined))})
	}
	if top := topByCount(miningByMat, 1); len(top) > 0 {
		miningStats = append(miningStats, recapStat{Label: "Top material", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " units refined", Breakdown: breakdownByCount(miningByMat)})
	}
	if len(miningStats) > 0 {
		sections = append(sections, recapSection{Title: "Mining", Stats: miningStats})
	}

	if totalMaterials > 0 {
		materialStats := []recapStat{
			{Label: "Materials collected", Value: fmtInt(int64(totalMaterials)), Sub: fmt.Sprintf("%d raw · %d encoded · %d manufactured", rawCollected, encodedCollected, manufCollected)},
		}
		if top := topByCount(materialCounts, 1); len(top) > 0 {
			materialStats = append(materialStats, recapStat{Label: "Most collected", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " units", Breakdown: breakdownByCount(materialCounts)})
		}
		sections = append(sections, recapSection{Title: "Materials", Stats: materialStats})
	}

	if totalCrafts > 0 {
		engStats := []recapStat{
			{Label: "Blueprint upgrades applied", Value: fmtInt(int64(totalCrafts))},
			{Label: "Engineers visited", Value: fmtInt(int64(len(engineerCounts)))},
		}
		if top := topByCount(engineerCounts, 1); len(top) > 0 {
			engStats = append(engStats, recapStat{Label: "Favourite engineer", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " upgrades", Breakdown: breakdownByCount(engineerCounts)})
		}
		if top := topByCount(blueprintCounts, 1); len(top) > 0 {
			engStats = append(engStats, recapStat{Label: "Most-applied modification", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + "x", Breakdown: breakdownByCount(blueprintCounts)})
		}
		sections = append(sections, recapSection{Title: "Engineering", Stats: engStats})
	}

	var shipStats []recapStat
	if shipsPurchased > 0 {
		shipStats = append(shipStats, recapStat{Label: "Ships purchased", Value: fmtInt(int64(shipsPurchased))})
	}
	if totalShipSpend > 0 {
		shipStats = append(shipStats, recapStat{Label: "Spent on ships", Value: formatCr(totalShipSpend)})
	}
	if len(shipStats) > 0 {
		sections = append(sections, recapSection{Title: "Ships", Stats: shipStats})
	}

	// Only one real carrier appears in this commander's data (CarrierStats always reports the
	// same CarrierID) -- if a commander somehow had more than one over their history, the latest
	// snapshot's name/balance would still win via latestCarrierStatsTS, same "current state"
	// pattern Powerplay uses above.
	if carrierName != "" || carrierJumps > 0 {
		carrierStats := []recapStat{}
		if carrierName != "" {
			label := carrierName
			if carrierCallsign != "" {
				label = carrierName + " (" + carrierCallsign + ")"
			}
			carrierStats = append(carrierStats, recapStat{Label: "Fleet carrier", Value: label})
			if carrierBalance > 0 {
				carrierStats = append(carrierStats, recapStat{Label: "Carrier balance", Value: formatCr(carrierBalance)})
			}
		}
		if carrierJumps > 0 {
			// Worded as "jumped aboard", not "jumped your carrier" -- CarrierJump fires for
			// anyone docked on a carrier when it jumps, not just its owner, confirmed against
			// real data that most of this commander's real fleet-carrier dockings (143 of 144)
			// were on a DIFFERENT carrier ("XHZ-8VB") than their own ("KNAC") -- almost certainly
			// a wingmate's, given the same commander dominates the Wing section above. Claiming
			// ownership of every one of these jumps would be a real overstatement of what the
			// data actually shows.
			sub := fmtInt(int64(len(carrierDestSystems))) + " destination systems"
			carrierStats = append(carrierStats, recapStat{Label: "Fleet carrier jumps", Value: fmtInt(int64(carrierJumps)), Sub: sub + " (yours or a wingmate's)"})
		}
		sections = append(sections, recapSection{Title: "Fleet Carrier", Stats: carrierStats})
	}

	if len(crewWages) > 0 {
		// User: "crew should have more info, time since hired, etc" -- real "CrewHire" events
		// (which would give a real hire date) never fired for this commander at all (checked
		// directly against their own real data -- 0 occurrences), so "time since hired" genuinely
		// isn't something this journal can answer for them; NpcCrewPaidWage (the only real crew
		// event this commander's data actually has) is a wage PAYMENT log, not a hire log. Real
		// data honestly can only stretch to: how many distinct crew, and the real grand total
		// across all of them (previously only the single top earner's own total was shown).
		var totalWages int64
		for _, c := range crewWages {
			totalWages += c.Value
		}
		// User: "last session recap can also end up with useless stuff like Total wages paid 0
		// cr, Highest-paid crew Garrett Bradford 0 cr · 1 payments" -- real gap, same class of bug
		// as the earlier zero-value cleanup: the section-level gate (len(crewWages) > 0, i.e. "at
		// least one wage-payment EVENT happened") doesn't guarantee the real Amount on that event
		// was actually nonzero. Every stat here now gated on its own real nonzero value too,
		// matching the rest of this recap.
		var crewStats []recapStat
		if len(crewWages) > 0 {
			crewStats = append(crewStats, recapStat{Label: "Distinct NPC crew", Value: fmtInt(int64(len(crewWages)))})
		}
		if totalWages > 0 {
			crewStats = append(crewStats, recapStat{Label: "Total wages paid", Value: formatCr(totalWages)})
		}
		if top := topByValue(crewWages, 1); len(top) > 0 && top[0].Value > 0 {
			crewStats = append(crewStats, recapStat{Label: "Highest-paid crew", Value: top[0].Name, Sub: formatCr(top[0].Value) + " · " + fmtInt(int64(top[0].Count)) + " payments", Breakdown: breakdownByValue(crewWages)})
		}
		if len(crewStats) > 0 {
			sections = append(sections, recapSection{Title: "Crew", Stats: crewStats})
		}
	}

	if len(depotProgress) > 0 || len(systemsClaimed) > 0 {
		colonyStats := []recapStat{}
		activeCount := 0
		var furthestLabel, furthestSystem string
		var furthestProgress float64
		for marketID, depot := range depotProgress {
			if depot.Complete {
				continue
			}
			activeCount++
			if depot.Progress >= furthestProgress {
				furthestProgress = depot.Progress
				if info, ok := dockedStations[marketID]; ok {
					furthestLabel = info.Station
					furthestSystem = info.System
				}
			}
		}
		if activeCount > 0 {
			colonyStats = append(colonyStats, recapStat{Label: "Active construction projects", Value: fmtInt(int64(activeCount))})
		}
		if furthestLabel != "" {
			colonyStats = append(colonyStats, recapStat{Label: "Furthest along", Value: furthestLabel, Sub: fmt.Sprintf("%s — %.1f%% complete", furthestSystem, furthestProgress*100)})
		}
		if len(systemsClaimed) > 0 {
			colonyStats = append(colonyStats, recapStat{Label: "Systems claimed", Value: fmtInt(int64(len(systemsClaimed))), Sub: systemsClaimed[len(systemsClaimed)-1]})
		}
		if contributionEvents > 0 {
			colonyStats = append(colonyStats, recapStat{Label: "Resources contributed", Value: fmtInt(contributionUnits), Sub: fmtInt(int64(contributionEvents)) + " deliveries"})
		}
		if len(colonyStats) > 0 {
			sections = append(sections, recapSection{Title: "Colonization", Stats: colonyStats})
		}
	}

	if len(stationVisits) > 0 {
		var topMarketID int64
		var topVisits *nameCount
		for marketID, v := range stationVisits {
			if topVisits == nil || v.Count > topVisits.Count {
				topMarketID = marketID
				topVisits = v
			}
		}
		sub := fmtInt(int64(topVisits.Count)) + " visits"
		if info, ok := dockedStations[topMarketID]; ok && info.System != "" {
			sub += " — " + info.System
		}
		// stationVisits is keyed by MarketID (int64), not name, since two different real stations
		// can share a name -- breakdownByCount can't take it directly (string-keyed nameCount maps
		// only), so the same real per-station Count is expanded by hand here instead.
		stationList := make([]*nameCount, 0, len(stationVisits))
		for _, v := range stationVisits {
			stationList = append(stationList, v)
		}
		sort.Slice(stationList, func(i, j int) bool { return stationList[i].Count > stationList[j].Count })
		stationBreakdown := make([]recapBreakdownItem, len(stationList))
		for i, v := range stationList {
			stationBreakdown[i] = recapBreakdownItem{Label: v.Name, Value: fmtInt(int64(v.Count)) + " visits"}
		}
		// User: "stations is basically empty, itd be nice for more" -- real Docked events already
		// carry StationType for every dock; this project just never counted it before.
		stationStats := []recapStat{
			{Label: "Home base", Value: topVisits.Name, Sub: sub, Breakdown: stationBreakdown},
			{Label: "Distinct stations visited", Value: fmtInt(int64(len(stationVisits)))},
		}
		if top := topByCount(stationTypeCounts, 1); len(top) > 0 {
			// breakdownByCount alone would leak the raw StationType key (Name stores it unprettified,
			// see where stationTypeCounts is populated above) -- prettifyKey applied per-item here,
			// same as the Value field on the line below already does for just the top one.
			typeBreakdown := breakdownByCount(stationTypeCounts)
			for i := range typeBreakdown {
				typeBreakdown[i].Label = prettifyKey(typeBreakdown[i].Label)
			}
			stationStats = append(stationStats, recapStat{Label: "Most common station type", Value: prettifyKey(top[0].Name), Sub: fmtInt(int64(top[0].Count)) + " dockings", Breakdown: typeBreakdown})
		}
		sections = append(sections, recapSection{Title: "Stations", Stats: stationStats})
	}

	// User: "missons would be good, rep gained, influence gained, idk something. its just so
	// empty" -- real per-mission FactionEffects data (see missionCompletedEvent's own comment),
	// not just the completed/accepted counts this section used to stop at.
	var missionStats []recapStat
	if missionsCompleted > 0 {
		missionStats = append(missionStats, recapStat{Label: "Missions completed", Value: fmtInt(int64(missionsCompleted))})
	}
	if missionsAccepted > 0 {
		missionStats = append(missionStats, recapStat{Label: "Missions accepted", Value: fmtInt(int64(missionsAccepted))})
	}
	if top := topByCount(employerCounts, 1); len(top) > 0 {
		missionStats = append(missionStats, recapStat{Label: "Most missions for", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " missions", Breakdown: breakdownByCount(employerCounts)})
	}
	totalRepGains, totalRepLosses := 0, 0
	for _, c := range repGainedFactions {
		totalRepGains += c
	}
	for _, c := range repLostFactions {
		totalRepLosses += c
	}
	if totalRepGains > 0 {
		sub := fmtInt(int64(len(repGainedFactions))) + " distinct factions"
		var topRepFaction string
		topRepCount := 0
		for faction, c := range repGainedFactions {
			if c > topRepCount {
				topRepCount, topRepFaction = c, faction
			}
		}
		if topRepFaction != "" {
			sub = "most with " + topRepFaction
		}
		missionStats = append(missionStats, recapStat{Label: "Reputation gained", Value: fmtInt(int64(totalRepGains)) + " missions", Sub: sub, Breakdown: breakdownFromIntMap(repGainedFactions)})
	}
	if totalRepLosses > 0 {
		missionStats = append(missionStats, recapStat{Label: "Reputation lost", Value: fmtInt(int64(totalRepLosses)) + " missions", Sub: fmtInt(int64(len(repLostFactions))) + " distinct factions", Breakdown: breakdownFromIntMap(repLostFactions)})
	}
	if len(influencedSystems) > 0 {
		missionStats = append(missionStats, recapStat{Label: "Systems influenced", Value: fmtInt(int64(len(influencedSystems)))})
	}
	if len(missionStats) > 0 {
		sections = append(sections, recapSection{Title: "Missions", Stats: missionStats})
	}

	// Real bug, fixed: a nil []string marshals to JSON `null`, not `[]` -- harmless for the
	// all-time recap (there's always at least one highlight in any real career), but the new
	// session-scoped recap (BuildSessionRecap) can legitimately have zero for a short/quiet
	// session, and the client-side session-recap renderer's `s.recap.highlights.length` crashed
	// outright on a real null. Initialized non-nil so JSON always gives `[]`, not `null`.
	var highlights []recapHighlight
	// User: "only show 3 deaths i think, (prioritize CMDR deaths, since those are players)" --
	// real Elite Dangerous journal convention (confirmed against this commander's own real Died
	// events, see isRealPlayerKiller's comment): a KillerName prefixed "Cmdr " is another real
	// commander, never an NPC. Every real CMDR kill is kept (there's rarely more than a couple in
	// a real history), with remaining slots up to the cap filled by the most recent NPC kills --
	// unless CMDR kills alone already exceed the cap, in which case it's the most recent CMDR
	// kills that win (still all real players, just trimmed like everything else here).
	const maxDeathHighlights = 3
	var cmdrKills, npcKills []diedEvent
	for _, k := range realKillers {
		if isRealPlayerKiller(k.KillerName) {
			cmdrKills = append(cmdrKills, k)
		} else {
			npcKills = append(npcKills, k)
		}
	}
	var shownKillers []diedEvent
	if len(cmdrKills) >= maxDeathHighlights {
		shownKillers = cmdrKills[len(cmdrKills)-maxDeathHighlights:]
	} else {
		shownKillers = append(shownKillers, cmdrKills...)
		remaining := maxDeathHighlights - len(shownKillers)
		start := len(npcKills) - remaining
		if start < 0 {
			start = 0
		}
		shownKillers = append(shownKillers, npcKills[start:]...)
	}
	omittedDeaths := len(realKillers) - len(shownKillers)
	sort.Slice(shownKillers, func(i, j int) bool { return shownKillers[i].Timestamp < shownKillers[j].Timestamp })
	for _, k := range shownKillers {
		name := k.KillerNameLocalised
		if name == "" {
			name = k.KillerName
		}
		isPlayer := isRealPlayerKiller(k.KillerName)
		// Not always literally a ship -- confirmed against real data (settlement defenses, an
		// on-foot Odyssey combatant) alongside real ship kills, and there's no reliable field to
		// tell the two apart. "(X)" reads correctly either way; "flying a X" only reads correctly
		// for the ship case and was actively wrong ("flying a Outpostindustrial") for the other.
		line := "Destroyed by " + name
		if isPlayer {
			line = "💀 " + line + " -- a real commander"
		}
		if k.KillerShip != "" {
			line += " (" + displayShip(k.KillerShip) + ")"
		}
		highlights = append(highlights, recapHighlight{Text: line, Timestamp: k.Timestamp})
	}
	if omittedDeaths > 0 {
		plural := "s"
		if omittedDeaths == 1 {
			plural = ""
		}
		highlights = append(highlights, recapHighlight{Text: fmt.Sprintf("...and %d earlier death%s not shown here", omittedDeaths, plural)})
	}
	// Only called out as a highlight once it's genuinely "frequent" (2+ sessions) -- a single
	// shared wing session isn't a "best friend" story, just a data point already covered by the
	// Wing section's stat card above.
	if top := topByCount(wingmateCounts, 1); len(top) > 0 && top[0].Count >= 2 {
		highlights = append(highlights, recapHighlight{
			Text:      fmt.Sprintf("Flew wing with CMDR %s %d times -- your most frequent wingmate", top[0].Name, top[0].Count),
			Timestamp: wingmateLatestTS[top[0].Name],
		})
	}
	// Only the single most recent promotion, not all of them -- with 5 real Promotion events in
	// this commander's history, listing every one would crowd out the rest of this list; "most
	// recent milestone" is the one actually worth calling out "wrapped"-style.
	if mostRecentPromoTrack != "" {
		if title := rankTitle(mostRecentPromoTrack, mostRecentPromoLevel); title != "" {
			trackLabel := strings.TrimSuffix(rankTrackLabels[mostRecentPromoTrack], " rank")
			highlights = append(highlights, recapHighlight{
				Text:      fmt.Sprintf("Promoted to %s (%s rank %d)", title, trackLabel, mostRecentPromoLevel),
				Timestamp: mostRecentPromoTS,
			})
		}
	}
	// User: "if its revealed maybe a closest to death would be cool, lowest health 3% or
	// something, idk" -- always shown as a plain stat above (Combat section) when any real
	// HullDamage data exists, but only promoted to a highlight-worthy callout once it's genuinely
	// dramatic (<=10%, same threshold as the stat's own Danger flag -- a real near-miss, not just
	// "took some damage"). Real display bug also caught here: the original %.0f rounding made a
	// genuinely-survived 0.2265% read as an indistinguishable "0%" -- the same number an actual
	// death would show (this commander's own real data has both) -- fixed with real decimal
	// precision (%.1f) so a close call reads as visibly different from a destruction.
	if haveHullDamage && lowestHullHealth <= 0.10 {
		line := fmt.Sprintf("🔥 Barely evaded death: hull dropped to %.4f%%", lowestHullHealth*100)
		if lowestHullSystem != "" {
			line += " in " + lowestHullSystem
		}
		highlights = append(highlights, recapHighlight{Text: line, Timestamp: lowestHullAt})
	}
	if highlights == nil {
		highlights = []recapHighlight{}
	}

	return recapData{
		Commander:   store.Commander,
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04 MST"),
		Sections:    sections,
		Highlights:  highlights,
	}
}

// sessionRecapData wraps the same recapData shape BuildRecap already produces (reused verbatim,
// zero duplicated stat logic) with the real session's own start/end for display context.
type sessionRecapData struct {
	Recap       recapData `json:"recap"`
	SessionFrom string    `json:"sessionFrom"`
	SessionTo   string    `json:"sessionTo"`
}

// BuildSessionRecap scopes BuildRecap to just the most recent play session. Session boundary =
// the LAST real "LoadGame" event's timestamp -- confirmed directly against this project's own
// earlier finding (materials.go's header comment): "Materials" fires "immediately after Commander
// and immediately before LoadGame ... once per game session login," meaning LoadGame is a real,
// verified once-per-session marker. Deliberately NOT "Fileheader" despite it superficially looking
// like a cleaner one-per-journal-file marker -- a real Fileheader carries its own "part" field
// (confirmed against this commander's own real data), meaning the game splits a SINGLE session
// across multiple journal files/Fileheaders when a file gets rotated, which would incorrectly
// fragment one real session into several if used as the boundary.
func BuildSessionRecap(store *Store) (sessionRecapData, bool) {
	var sessionStart string
	for _, e := range store.RawEvents {
		if e.Event == "LoadGame" && e.Timestamp > sessionStart {
			sessionStart = e.Timestamp
		}
	}
	if sessionStart == "" {
		return sessionRecapData{}, false
	}
	var sessionEvents []RawEvent
	var sessionEnd string
	for _, e := range store.RawEvents {
		if e.Timestamp < sessionStart {
			continue
		}
		sessionEvents = append(sessionEvents, e)
		if e.Timestamp > sessionEnd {
			sessionEnd = e.Timestamp
		}
	}
	if len(sessionEvents) == 0 {
		return sessionRecapData{}, false
	}
	// Real bug, caught in verification: BuildRecap also draws several stats (Systems visited,
	// Furthest system, Presumed bio value, Notable/rare finds) straight from Store.Systems, NOT
	// from RawEvents -- sharing the full career-spanning map unfiltered made those stats silently
	// show lifetime totals inside what's supposed to be a single-session recap (confirmed against
	// real data: "Systems visited: 482" for a real ~1-hour session). Filtered to only systems this
	// project's own real UpdatedAt (set on every touch, see parse.go) falls within the session
	// window, so these stats now genuinely reflect just this session's own activity.
	sessionSystems := map[int64]*System{}
	for addr, sys := range store.Systems {
		if sys.UpdatedAt >= sessionStart && sys.UpdatedAt <= sessionEnd {
			sessionSystems[addr] = sys
		}
	}
	sessionStore := &Store{Commander: store.Commander, Systems: sessionSystems, RawEvents: sessionEvents}
	return sessionRecapData{
		Recap:       BuildRecap(sessionStore, true),
		SessionFrom: sessionStart,
		SessionTo:   sessionEnd,
	}, true
}

func RenderSummary(store *Store) (string, error) {
	data := BuildRecap(store, false)
	if session, ok := BuildSessionRecap(store); ok {
		data.Session = &session
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	escaped := strings.ReplaceAll(string(dataJSON), "</", "<\\/")
	return strings.Replace(summaryTemplate, "__DATA_JSON__", escaped, 1), nil
}
