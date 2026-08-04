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
}

type recapSection struct {
	Title string      `json:"title"`
	Stats []recapStat `json:"stats"`
}

type recapData struct {
	Commander   string         `json:"commander"`
	GeneratedAt string         `json:"generatedAt"`
	Sections    []recapSection `json:"sections"`
	Highlights  []string       `json:"highlights"`
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

// Pilots Federation rank ladders (9 levels, 0-8) plus the two superpower Navy ladders (15 levels,
// 0-14). These titles are NOT present anywhere in the journal itself -- Rank/Promotion only ever
// give the numeric level -- so unlike everything else in this file, they can't be grounded
// against this commander's own data. Sourced and cross-checked against multiple independent
// community references (elite-dangerous.fandom.com, the Player Journal manual appendix, and
// direct confirmation of the Exobiologist ladder via real player discussion threads using these
// exact title names) rather than a single source, the same "vendored community reference data"
// category as vendor/value_table.json's species values -- not runtime-fetched, not guaranteed to
// never drift from a future in-game rebalance, but the best available offline ground truth.
var pilotsFederationRankTitles = map[string][]string{
	"Combat":       {"Harmless", "Mostly Harmless", "Novice", "Competent", "Expert", "Master", "Dangerous", "Deadly", "Elite"},
	"Trade":        {"Penniless", "Mostly Penniless", "Peddler", "Dealer", "Merchant", "Broker", "Entrepreneur", "Tycoon", "Elite"},
	"Explore":      {"Aimless", "Mostly Aimless", "Scout", "Surveyor", "Trailblazer", "Pathfinder", "Ranger", "Pioneer", "Elite"},
	"CQC":          {"Helpless", "Mostly Helpless", "Amateur", "Semi Professional", "Professional", "Champion", "Hero", "Legend", "Elite"},
	"Soldier":      {"Defenceless", "Mostly Defenceless", "Rookie", "Soldier", "Gunslinger", "Warrior", "Gladiator", "Deadeye", "Elite"},
	"Exobiologist": {"Directionless", "Mostly Directionless", "Compiler", "Collector", "Cataloguer", "Taxonomist", "Ecologist", "Geneticist", "Elite"},
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

func BuildRecap(store *Store) recapData {
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

		missionsCompleted int
		missionsAccepted  int

		fsdJumps            int
		totalLightYears     float64
		bodiesScanned       int
		firstDiscoveries    int
		fullSurfaceScans    int
		explorationEarnings int64
		explorationSales    int

		wingmateCounts = map[string]*nameCount{}

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

		crewWages = map[string]*nameCount{}

		carrierJumps         int
		carrierDestSystems   = map[string]bool{}
		carrierName          string
		carrierCallsign      string
		carrierBalance       int64
		latestCarrierStatsTS string

		dockedStations = map[int64]struct{ Station, System, Type string }{}
		stationVisits  = map[int64]*nameCount{} // keyed by MarketID, non-carrier stations only
		depotProgress  = map[int64]struct {
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
				realKillers = append(realKillers, v)
			}
		case "Interdicted":
			timesIntdctd++
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
		case "MissionAccepted":
			missionsAccepted++
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
				pledgedPower = v.Power
				if latestPPTimestamp == "" || e.Timestamp >= latestPPTimestamp {
					latestPPTimestamp = e.Timestamp
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
				pledgedPower = v.Power
				if latestPPTimestamp == "" || e.Timestamp >= latestPPTimestamp {
					latestPPTimestamp = e.Timestamp
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
	for _, fv := range ComputeFloraValues(store) {
		if fv.HasValue && !fv.Lost {
			bioValue += fv.Value
		}
	}

	explorationStats := []recapStat{
		{Label: "Systems visited", Value: fmtInt(int64(systemsVisited))},
		{Label: "Light-years travelled", Value: fmt.Sprintf("%.1f LY", totalLightYears), Sub: fmtInt(int64(fsdJumps)) + " FSD jumps"},
		{Label: "Bodies scanned", Value: fmtInt(int64(bodiesScanned)), Sub: fmtInt(int64(firstDiscoveries)) + " first discoveries"},
		{Label: "Full surface scans", Value: fmtInt(int64(fullSurfaceScans))},
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
		explorationStats = append(explorationStats, recapStat{Label: "Notable/rare finds", Value: fmtInt(int64(totalNotable)), Sub: sub})
	}
	if first, err1 := time.Parse(time.RFC3339, firstTimestamp); err1 == nil {
		sub := ""
		if last, err2 := time.Parse(time.RFC3339, lastTimestamp); err2 == nil {
			days := int(last.Sub(first).Hours()/24) + 1
			sub = fmtInt(int64(days)) + " days of logged activity"
		}
		explorationStats = append(explorationStats, recapStat{Label: "Commander since", Value: first.Format("Jan 2, 2006"), Sub: sub})
	}
	sections = append(sections, recapSection{Title: "Exploration", Stats: explorationStats})

	// Only tracks the commander has actually made progress in get a card -- a rank-0 track is
	// just the default starting state for everyone, not a real achievement to report (same
	// "don't force a boring stat" standard as the rest of this file). Federation/Empire go last
	// since they're the two tracks with real name recognition beyond the Pilots Federation ones.
	if haveRank {
		var rankStats []recapStat
		trackOrder := []string{"Combat", "Trade", "Explore", "CQC", "Soldier", "Exobiologist", "Federation", "Empire"}
		levelByTrack := map[string]int{
			"Combat": latestRank.Combat, "Trade": latestRank.Trade, "Explore": latestRank.Explore,
			"CQC": latestRank.CQC, "Soldier": latestRank.Soldier, "Exobiologist": latestRank.Exobiologist,
			"Federation": latestRank.Federation, "Empire": latestRank.Empire,
		}
		maxLevel := map[string]int{"Federation": 14, "Empire": 14}
		for _, track := range trackOrder {
			level := levelByTrack[track]
			if level <= 0 {
				continue
			}
			max := 8
			if m, ok := maxLevel[track]; ok {
				max = m
			}
			title := rankTitle(track, level)
			sub := fmtInt(int64(level)) + "/" + fmtInt(int64(max))
			value := title
			if value == "" {
				value = sub // title lookup failure (shouldn't happen for a valid 0-14/0-8 level) falls back to the raw progress instead of an empty card
			}
			rankStats = append(rankStats, recapStat{Label: rankTrackLabels[track], Value: value, Sub: sub})
		}
		if len(rankStats) > 0 {
			sections = append(sections, recapSection{Title: "Rank", Stats: rankStats})
		}
	}

	combatStats := []recapStat{
		{Label: "Total kills", Value: fmtInt(int64(totalKills))},
		{Label: "Combat earnings", Value: formatCr(totalCombatEarnings)},
	}
	if top := topByCount(killsByTargetShip, 1); len(top) > 0 {
		combatStats = append(combatStats, recapStat{Label: "Favourite target", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " destroyed"})
	}
	if top := topByCount(killsByOwnShip, 1); len(top) > 0 {
		combatStats = append(combatStats, recapStat{Label: "Favourite combat ship", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " kills flying it"})
	}
	if len(killsByVictimFac) > 0 {
		list := make([]*nameCount, 0, len(killsByVictimFac))
		for _, v := range killsByVictimFac {
			list = append(list, v)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Count > list[j].Count })
		combatStats = append(combatStats, recapStat{Label: "Biggest rival faction", Value: list[0].Name, Sub: fmtInt(int64(list[0].Count)) + " of their ships destroyed"})
	}
	if biggestBounty > 0 {
		combatStats = append(combatStats, recapStat{Label: "Biggest single bounty", Value: formatCr(biggestBounty), Sub: biggestBountyTarget})
	}
	combatStats = append(combatStats,
		recapStat{Label: "Times destroyed", Value: fmtInt(int64(timesKilled))},
		recapStat{Label: "Times interdicted", Value: fmtInt(int64(timesIntdctd))},
	)
	sections = append(sections, recapSection{Title: "Combat", Stats: combatStats})

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
			crimeStats = append(crimeStats, recapStat{Label: "Most common offence", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " times"})
		}
		if top := topByCount(crimeFactionCounts, 1); len(top) > 0 {
			crimeStats = append(crimeStats, recapStat{Label: "Most trouble with", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " incidents"})
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
			wingStats = append(wingStats, recapStat{Label: "Most frequent wingmate", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " " + sessionWord + " together"})
		}
		wingStats = append(wingStats, recapStat{Label: "Distinct wingmates", Value: fmtInt(int64(len(wingmateCounts)))})
		sections = append(sections, recapSection{Title: "Wing", Stats: wingStats})
	}

	// Only built if the commander has actually pledged to a power -- real data confirms this
	// commander has (1,038 PowerplayMerits events, 69 Powerplay snapshots), but the field is
	// deliberately gated the same way Wing is, for anyone whose journal has none of this.
	if pledgedPower != "" {
		ppStats := []recapStat{
			{Label: "Pledged to", Value: pledgedPower},
			{Label: "Powerplay rank", Value: fmtInt(int64(currentPPRank))},
			{Label: "Current merits", Value: fmtInt(currentPPMerits)},
		}
		if currentPPTimePledged > 0 {
			ppStats = append(ppStats, recapStat{Label: "Time pledged", Value: formatDays(currentPPTimePledged)})
		}
		if meritEvents > 0 {
			ppStats = append(ppStats, recapStat{Label: "Merits earned", Value: fmtInt(totalMeritsGained), Sub: fmtInt(int64(meritEvents)) + " merit-earning actions"})
		}
		sections = append(sections, recapSection{Title: "Powerplay", Stats: ppStats})
	}

	tradeStats := []recapStat{
		{Label: "Total trade profit", Value: formatCr(totalTradeProfit)},
		{Label: "Units sold", Value: fmtInt(totalUnitsSold)},
		{Label: "Units bought", Value: fmtInt(totalUnitsBought)},
	}
	{
		list := make([]*nameCount, 0, len(tradeByCommodity))
		for _, v := range tradeByCommodity {
			list = append(list, v)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Value > list[j].Value })
		if len(list) > 0 {
			tradeStats = append(tradeStats, recapStat{Label: "Most profitable commodity", Value: list[0].Name, Sub: formatCr(list[0].Value) + " profit"})
		}
	}
	if biggestSale > 0 {
		tradeStats = append(tradeStats, recapStat{Label: "Biggest single sale", Value: formatCr(biggestSale), Sub: biggestSaleGood})
	}
	sections = append(sections, recapSection{Title: "Trading", Stats: tradeStats})

	miningStats := []recapStat{
		{Label: "Total refined", Value: fmtInt(int64(totalRefined))},
	}
	if top := topByCount(miningByMat, 1); len(top) > 0 {
		miningStats = append(miningStats, recapStat{Label: "Top material", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " units refined"})
	}
	sections = append(sections, recapSection{Title: "Mining", Stats: miningStats})

	if totalMaterials > 0 {
		materialStats := []recapStat{
			{Label: "Materials collected", Value: fmtInt(int64(totalMaterials)), Sub: fmt.Sprintf("%d raw · %d encoded · %d manufactured", rawCollected, encodedCollected, manufCollected)},
		}
		if top := topByCount(materialCounts, 1); len(top) > 0 {
			materialStats = append(materialStats, recapStat{Label: "Most collected", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " units"})
		}
		sections = append(sections, recapSection{Title: "Materials", Stats: materialStats})
	}

	if totalCrafts > 0 {
		engStats := []recapStat{
			{Label: "Blueprint upgrades applied", Value: fmtInt(int64(totalCrafts))},
			{Label: "Engineers visited", Value: fmtInt(int64(len(engineerCounts)))},
		}
		if top := topByCount(engineerCounts, 1); len(top) > 0 {
			engStats = append(engStats, recapStat{Label: "Favourite engineer", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " upgrades"})
		}
		if top := topByCount(blueprintCounts, 1); len(top) > 0 {
			engStats = append(engStats, recapStat{Label: "Most-applied modification", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + "x"})
		}
		sections = append(sections, recapSection{Title: "Engineering", Stats: engStats})
	}

	shipStats := []recapStat{
		{Label: "Ships purchased", Value: fmtInt(int64(shipsPurchased))},
		{Label: "Spent on ships", Value: formatCr(totalShipSpend)},
	}
	sections = append(sections, recapSection{Title: "Ships", Stats: shipStats})

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
		crewStats := []recapStat{}
		if top := topByValue(crewWages, 1); len(top) > 0 {
			crewStats = append(crewStats, recapStat{Label: "NPC crew", Value: top[0].Name, Sub: fmtInt(int64(top[0].Count)) + " payments"})
			crewStats = append(crewStats, recapStat{Label: "Wages paid", Value: formatCr(top[0].Value)})
		}
		sections = append(sections, recapSection{Title: "Crew", Stats: crewStats})
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
		sections = append(sections, recapSection{Title: "Stations", Stats: []recapStat{
			{Label: "Home base", Value: topVisits.Name, Sub: sub},
		}})
	}

	missionStats := []recapStat{
		{Label: "Missions completed", Value: fmtInt(int64(missionsCompleted))},
		{Label: "Missions accepted", Value: fmtInt(int64(missionsAccepted))},
	}
	sections = append(sections, recapSection{Title: "Missions", Stats: missionStats})

	var highlights []string
	// Capped to the most recent few -- confirmed against a real, more PvP-active tester's data
	// that this can otherwise grow into a wall of text (realKillers has no size limit of its own,
	// since it's every real Died event with a killer name, and someone who dies a lot has a lot).
	// Highlights are meant to stay highlight-sized; the full history is what the events search
	// page is for.
	const maxDeathHighlights = 5
	shownKillers := realKillers
	omittedDeaths := 0
	if len(shownKillers) > maxDeathHighlights {
		omittedDeaths = len(shownKillers) - maxDeathHighlights
		shownKillers = shownKillers[omittedDeaths:] // most recent N -- realKillers is chronological
	}
	for _, k := range shownKillers {
		name := k.KillerNameLocalised
		if name == "" {
			name = k.KillerName
		}
		// Not always literally a ship -- confirmed against real data (settlement defenses, an
		// on-foot Odyssey combatant) alongside real ship kills, and there's no reliable field to
		// tell the two apart. "(X)" reads correctly either way; "flying a X" only reads correctly
		// for the ship case and was actively wrong ("flying a Outpostindustrial") for the other.
		line := "Destroyed by " + name
		if k.KillerShip != "" {
			line += " (" + displayShip(k.KillerShip) + ")"
		}
		highlights = append(highlights, line)
	}
	if omittedDeaths > 0 {
		plural := "s"
		if omittedDeaths == 1 {
			plural = ""
		}
		highlights = append(highlights, fmt.Sprintf("...and %d earlier death%s not shown here", omittedDeaths, plural))
	}
	// Only called out as a highlight once it's genuinely "frequent" (2+ sessions) -- a single
	// shared wing session isn't a "best friend" story, just a data point already covered by the
	// Wing section's stat card above.
	if top := topByCount(wingmateCounts, 1); len(top) > 0 && top[0].Count >= 2 {
		highlights = append(highlights, fmt.Sprintf("Flew wing with CMDR %s %d times -- your most frequent wingmate", top[0].Name, top[0].Count))
	}
	// Only the single most recent promotion, not all of them -- with 5 real Promotion events in
	// this commander's history, listing every one would crowd out the rest of this list; "most
	// recent milestone" is the one actually worth calling out "wrapped"-style.
	if mostRecentPromoTrack != "" {
		if title := rankTitle(mostRecentPromoTrack, mostRecentPromoLevel); title != "" {
			trackLabel := strings.TrimSuffix(rankTrackLabels[mostRecentPromoTrack], " rank")
			highlights = append(highlights, fmt.Sprintf("Promoted to %s (%s rank %d)", title, trackLabel, mostRecentPromoLevel))
		}
	}

	return recapData{
		Commander:   store.Commander,
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04 MST"),
		Sections:    sections,
		Highlights:  highlights,
	}
}

func RenderSummary(store *Store) (string, error) {
	data := BuildRecap(store)
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	escaped := strings.ReplaceAll(string(dataJSON), "</", "<\\/")
	return strings.Replace(summaryTemplate, "__DATA_JSON__", escaped, 1), nil
}
