package main

// Data model -- mirrors the Python standalone/db.py schema in shape, but held in memory as
// plain structs and persisted as a single gob-encoded cache file (see store.go) instead of
// SQLite. Deliberate deviation from the Python version: no cgo, no external Go module, keeps
// the binary genuinely minimal and the whole thing pure standard library.

type System struct {
	SystemAddress     int64
	Name              string
	X, Y, Z           float64
	Region            string
	Population        int64
	Faction           string
	Government        string
	Allegiance        string
	Security          string
	ClaimedByCmdr     bool
	BodyCountTotal    int
	Honked            bool
	FullyScanned      bool
	UpdatedAt         string
	Stars             map[int]*Star   // keyed by BodyID
	Planets           map[int]*Planet // keyed by BodyID
}

type Star struct {
	BodyID       int
	Name         string
	Distance     float64
	Type         string
	Subclass     int
	Luminosity   string
	Mass         float64
	WasDiscovered *bool
	WasFootfalled *bool
	UpdatedAt    string
}

type Planet struct {
	BodyID           int
	Name             string
	Type             string
	Landable         bool
	Mass             float64
	Distance         float64
	ParentStarBodyID *int
	TerraformState   string
	Atmosphere       string
	Gravity          float64
	Temp             float64
	Pressure         float64
	Discovered       bool
	WasDiscovered    *bool
	Mapped           bool
	WasMapped        *bool
	Efficient        bool
	WasFootfalled    *bool
	BioSignalCount   int
	Flora            map[string]*FloraScan // keyed by genus+"|"+species
	UpdatedAt        string
}

type FloraScan struct {
	Genus      string
	Species    string
	Variant    string
	Count      int // 0=genus hint only, 1=Log, 2=Sample, 3=Analyse
	WasLogged  *bool
	ScannedAt  string
}

// Store is the whole in-memory database -- persisted as one gob file between runs.
type Store struct {
	Commander      string
	CommanderFID   string
	Systems        map[int64]*System // keyed by SystemAddress
	SaleTimes      []string          // exobio_sales.sold_at
	DeathTimes     []string
	Resurrections  []Resurrection
	SystemSales    []SystemSale
	ParseProgress  map[string]FileProgress // keyed by journal filename
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
