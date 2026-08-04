package main

// Data model -- mirrors the Python standalone/db.py schema in shape, but held in memory as
// plain structs and persisted as a single gob-encoded cache file (see store.go) instead of
// SQLite. Deliberate deviation from the Python version: no cgo, no external Go module, keeps
// the binary genuinely minimal and the whole thing pure standard library.

type System struct {
	SystemAddress  int64
	Name           string
	X, Y, Z        float64
	Region         string
	Population     int64
	Faction        string
	Government     string
	Allegiance     string
	Security       string
	ClaimedByCmdr  bool
	BodyCountTotal int
	Honked         bool
	FullyScanned   bool
	UpdatedAt      string
	Stars          map[int]*Star   // keyed by BodyID
	Planets        map[int]*Planet // keyed by BodyID
}

type Star struct {
	BodyID        int
	Name          string
	Distance      float64
	Type          string
	Subclass      int
	Luminosity    string
	Mass          float64
	BarycenterIDs []int // set when this star is one half (or more) of a binary/multi pair
	WasDiscovered *bool
	WasFootfalled *bool
	UpdatedAt     string
}

type Planet struct {
	BodyID           int
	Name             string
	Type             string
	Landable         bool
	Mass             float64
	Distance         float64
	ParentStarBodyID *int
	BarycenterIDs    []int // set instead of ParentStarBodyID for circumbinary bodies -- orbits
	// the shared center of two+ stars, not any single one of them (see parentAncestry in parse.go)
	OrbitsPlanetBodyID *int // set when this body's closest real Parents entry is another Planet, not a Star -- a moon, orbiting a planet rather than the primary directly (see orbitsPlanet in parse.go)
	TerraformState     string
	Atmosphere         string
	Gravity            float64
	Temp               float64
	Pressure           float64
	Discovered         bool
	WasDiscovered      *bool
	Mapped             bool
	WasMapped          *bool
	Efficient          bool
	WasFootfalled      *bool
	BioSignalCount     int
	Flora              map[string]*FloraScan // keyed by genus+"|"+species
	UpdatedAt          string
}

type FloraScan struct {
	Genus     string
	Species   string
	Variant   string
	Count     int // 0=genus hint only, 1=Log, 2=Sample, 3=Analyse
	WasLogged *bool
	ScannedAt string
}

// Store is the whole in-memory database -- persisted as one gob file between runs.
type Store struct {
	Commander string // in-game commander name only -- deliberately no Frontier account ID (FID)
	// anywhere in this struct, see parse.go's onCommander/recordRawEvent comments for why
	Systems       map[int64]*System // keyed by SystemAddress
	SaleTimes     []string          // exobio_sales.sold_at
	DeathTimes    []string
	Resurrections []Resurrection
	SystemSales   []SystemSale
	ParseProgress map[string]FileProgress // keyed by journal filename
	RawEvents     []RawEvent              // every journal line ever seen, verbatim -- see RawEvent
}

// RawEvent is a verbatim capture of one journal line, regardless of whether any other part of
// this program has a bespoke handler for its event type -- the "parse the whole journal, make
// it searchable" requirement can't be met by only keeping what the exploration-focused parts of
// this program already care about. SystemName/BodyID are best-effort context (from the event's
// own fields when present, falling back to whatever system the commander was last known to be
// in otherwise) to make the raw log filterable without needing to re-parse Raw on every search.
type RawEvent struct {
	Timestamp  string
	Event      string
	SystemName string
	BodyID     *int
	Raw        string // the exact journal line, unmodified
}

type Resurrection struct {
	ResurrectedAt string
	Option        string
}

type SystemSale struct {
	SoldAt        string
	BaseValue     int64
	Bonus         int64
	TotalEarnings int64
	SystemNames   []string
}

type FileProgress struct {
	Mtime     float64
	LineCount int
}

func NewStore() *Store {
	return &Store{
		Systems:       make(map[int64]*System),
		ParseProgress: make(map[string]FileProgress),
	}
}

func (s *Store) getOrCreateSystem(systemAddress int64) *System {
	sys, ok := s.Systems[systemAddress]
	if !ok {
		sys = &System{
			SystemAddress: systemAddress,
			Stars:         make(map[int]*Star),
			Planets:       make(map[int]*Planet),
		}
		s.Systems[systemAddress] = sys
	}
	return sys
}
