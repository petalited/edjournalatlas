package main

// A second, separate generated page from the same local cache: a searchable browser over every
// single journal line ever seen (RawEvent, see types.go/parse.go), not just the
// exploration-relevant subset the main viewer.go focuses on. Deliberately its own file rather
// than folded into the main viewer's JSON -- a real journal easily reaches hundreds of thousands
// of lines across 170+ event types, and embedding all of that into the SAME static HTML that's
// regenerated every run would balloon it by orders of magnitude for something most runs don't
// need to open at all.

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

//go:embed events_template.html
var eventsTemplate string

type rawEventOut struct {
	Timestamp  string `json:"timestamp"`
	Event      string `json:"event"`
	SystemName string `json:"systemName,omitempty"`
	BodyID     *int   `json:"bodyId,omitempty"`
	Raw        string `json:"raw"`
}

type eventsData struct {
	GeneratedAt string        `json:"generatedAt"`
	Events      []rawEventOut `json:"events"`
}

func RenderEvents(store *Store) (string, int, error) {
	events := make([]rawEventOut, 0, len(store.RawEvents))
	for _, e := range store.RawEvents {
		events = append(events, rawEventOut{
			Timestamp: e.Timestamp, Event: e.Event, SystemName: e.SystemName, BodyID: e.BodyID, Raw: e.Raw,
		})
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Timestamp < events[j].Timestamp })

	data := eventsData{
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04 MST"),
		Events:      events,
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", 0, err
	}

	// This page's raw JSON is real journal data -- tens of thousands of events sharing the same
	// field names, event-type strings, and (for anyone parked in one system a while) the same
	// StarSystem/SystemAddress over and over. Confirmed against a real 147MB export: gzip alone
	// gets it down to ~12MB, a ~12x reduction just from that repetition -- worth paying for with
	// a client-side DecompressionStream call (native to every evergreen browser, no library)
	// rather than shipping the raw text. base64 is needed since the compressed bytes have to live
	// inside a text-only <script> tag.
	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	if _, err := gw.Write(dataJSON); err != nil {
		return "", 0, err
	}
	if err := gw.Close(); err != nil {
		return "", 0, err
	}
	encoded := base64.StdEncoding.EncodeToString(gz.Bytes())
	html := strings.Replace(eventsTemplate, "__DATA_GZIP_B64__", encoded, 1)
	return html, len(events), nil
}
