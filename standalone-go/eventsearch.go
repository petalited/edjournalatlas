package main

// A second, separate generated page from the same local cache: a searchable browser over every
// single journal line ever seen (RawEvent, see types.go/parse.go), not just the
// exploration-relevant subset the main viewer.go focuses on. Deliberately its own file rather
// than folded into the main viewer's JSON -- a real journal has tens of thousands of lines
// across 170+ event types (confirmed against real data: 67,756 events, 72MB of raw JSON, for
// one commander's history so far), and embedding all of that into the SAME static HTML that's
// regenerated every run would balloon it by two orders of magnitude for something most runs
// don't need to open at all.

import (
	_ "embed"
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
	escaped := strings.ReplaceAll(string(dataJSON), "</", "<\\/")
	html := strings.Replace(eventsTemplate, "__DATA_JSON__", escaped, 1)
	return html, len(events), nil
}
