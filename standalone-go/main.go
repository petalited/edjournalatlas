// standalone -- a local "explore what you've found" viewer for Elite Dangerous. Parses your
// own journal files directly (no ExploData/BioScan/Pioneer dependency), keeps its own small
// local cache for fast incremental re-runs, and builds a browsable HTML report. See
// docs/StandaloneJournalParser.md for the full mechanics writeup and README.md for how to use
// this program. This is the Go rewrite of edexotracker/standalone/ -- same mechanics, same
// verified numbers, much smaller/faster binary; see that directory's own docs for why.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const defaultDBPath = "standalone.db"
const defaultOutPath = "standalone_viewer.html"
const defaultEventsOutPath = "standalone_events.html"
const defaultSummaryOutPath = "standalone_summary.html"
const defaultUnmodeledOutPath = "standalone_unmodeled.json"
const defaultMaterialsOutPath = "standalone_materials.html"

func main() {
	journalDir := flag.String("journal-dir", "", "Directory containing Journal.*.log files (auto-detected if omitted)")
	dbPath := flag.String("db", defaultDBPath, "Path to the local cache file")
	outPath := flag.String("out", defaultOutPath, "Path to write the viewer HTML")
	eventsOutPath := flag.String("events-out", defaultEventsOutPath, "Path to write the full-journal search page")
	noEvents := flag.Bool("no-events", false, "Skip building the full-journal search page (it's the whole raw journal, so it's the slower/bigger of the two)")
	summaryOutPath := flag.String("summary-out", defaultSummaryOutPath, "Path to write the career recap page")
	noSummary := flag.Bool("no-summary", false, "Skip building the career recap page")
	unmodeledOutPath := flag.String("unmodeled-out", defaultUnmodeledOutPath, "Path to write the unmodeled-event-types coverage report")
	noUnmodeled := flag.Bool("no-unmodeled", false, "Skip building the unmodeled-event-types coverage report")
	materialsOutPath := flag.String("materials-out", defaultMaterialsOutPath, "Path to write the materials inventory page")
	noMaterials := flag.Bool("no-materials", false, "Skip building the materials inventory page")
	noOpen := flag.Bool("no-open", false, "Don't open the viewer in a browser automatically")
	flag.Parse()

	fmt.Println("standalone -- Elite Dangerous journal viewer")
	fmt.Println()

	dir := *journalDir
	if dir == "" {
		dir = resolveJournalDir()
		if dir == "" {
			pauseOnWindows()
			os.Exit(1)
		}
	}
	fmt.Println("Using journal folder:", dir)
	fmt.Println()
	fmt.Println("Parsing your Elite Dangerous journal...")

	store, err := LoadStore(*dbPath)
	if err != nil {
		fmt.Println("Error loading local cache:", err)
		pauseOnWindows()
		os.Exit(1)
	}

	filesTouched, newLines, err := parseJournalDirectory(store, dir)
	if err != nil {
		fmt.Println("Error reading journal folder:", err)
		pauseOnWindows()
		os.Exit(1)
	}

	// RawEvents is append-only, in file-processing order for THIS run -- not necessarily true
	// chronological order overall. Importing an old commander's journals (e.g. from another PC)
	// after already having parsed newer ones appends that old journal's events at the END of the
	// slice, even though their real in-game timestamps are much earlier. Every event already
	// carries its own real timestamp straight from the journal line itself (not derived from the
	// filename or any file metadata), so re-sort on that directly -- stable, so events that
	// share the same one-second timestamp resolution keep their real relative order.
	sort.SliceStable(store.RawEvents, func(i, j int) bool {
		return store.RawEvents[i].Timestamp < store.RawEvents[j].Timestamp
	})

	if filesTouched == 0 {
		// "Up to date" implies there WAS a previous successful parse -- misleading the first time
		// this runs against a folder that genuinely has no Journal.*.log files in it yet (e.g. a
		// brand new commander who hasn't undocked once). len(store.RawEvents) == 0 is the real
		// signal for "nothing has ever been parsed into this store", not just "nothing changed
		// since last time" -- distinct problems with distinct fixes, same reasoning as the
		// empty-state messaging on the generated HTML pages themselves.
		if len(store.RawEvents) == 0 {
			fmt.Println("No journal activity found yet -- go play some Elite Dangerous, then run this again.")
		} else {
			fmt.Println("Up to date -- no new journal data since last time.")
		}
	} else {
		fmt.Printf("Parsed %d new line(s) across %d file(s).\n", newLines, filesTouched)
	}

	if err := SaveStore(*dbPath, store); err != nil {
		fmt.Println("Error saving local cache:", err)
		pauseOnWindows()
		os.Exit(1)
	}

	html, systemCount, err := Render(store)
	if err != nil {
		fmt.Println("Error building viewer:", err)
		pauseOnWindows()
		os.Exit(1)
	}
	if err := os.WriteFile(*outPath, []byte(html), 0o644); err != nil {
		fmt.Println("Error writing viewer file:", err)
		pauseOnWindows()
		os.Exit(1)
	}
	fmt.Printf("Wrote %s (%d systems)\n", *outPath, systemCount)

	if !*noEvents {
		eventsHTML, eventCount, err := RenderEvents(store)
		if err != nil {
			fmt.Println("Error building full-journal search page:", err)
		} else if err := os.WriteFile(*eventsOutPath, []byte(eventsHTML), 0o644); err != nil {
			fmt.Println("Error writing full-journal search page:", err)
		} else {
			fmt.Printf("Wrote %s (%d journal events, every event type -- open it separately to search your whole journal)\n", *eventsOutPath, eventCount)
		}
	}

	if !*noSummary {
		summaryHTML, err := RenderSummary(store)
		if err != nil {
			fmt.Println("Error building career recap page:", err)
		} else if err := os.WriteFile(*summaryOutPath, []byte(summaryHTML), 0o644); err != nil {
			fmt.Println("Error writing career recap page:", err)
		} else {
			fmt.Printf("Wrote %s (combat/trading/mining/ships recap)\n", *summaryOutPath)
		}
	}

	if !*noUnmodeled {
		unmodeledJSON, unmodeledCount, err := RenderUnmodeledReport(store)
		if err != nil {
			fmt.Println("Error building unmodeled-event coverage report:", err)
		} else if err := os.WriteFile(*unmodeledOutPath, []byte(unmodeledJSON), 0o644); err != nil {
			fmt.Println("Error writing unmodeled-event coverage report:", err)
		} else if unmodeledCount > 0 {
			fmt.Printf("Wrote %s (%d journal event types your recap doesn't summarize yet -- safe to share, only lists event/field names, never personal values)\n", *unmodeledOutPath, unmodeledCount)
		}
	}

	if !*noMaterials {
		materialsHTML, materialCount, err := RenderMaterials(store)
		if err != nil {
			fmt.Println("Error building materials inventory page:", err)
		} else if err := os.WriteFile(*materialsOutPath, []byte(materialsHTML), 0o644); err != nil {
			fmt.Println("Error writing materials inventory page:", err)
		} else {
			fmt.Printf("Wrote %s (%d distinct materials -- current Raw/Manufactured/Encoded inventory)\n", *materialsOutPath, materialCount)
		}
	}

	if !*noOpen {
		openBrowser(*outPath)
	}
	pauseOnWindows()
}

// parseJournalDirectory walks every Journal.*.log in the given directory in chronological
// (filename) order, incrementally: an unchanged file (same size/mtime as last recorded) is
// skipped entirely without being opened; a grown or new file resumes from its saved line count.
// Mirrors edexotracker/standalone/parse_journals.py's incremental design exactly.
func parseJournalDirectory(store *Store, dir string) (filesTouched, newLinesTotal int, err error) {
	matches, err := filepath.Glob(filepath.Join(dir, journalGlob))
	if err != nil {
		return 0, 0, err
	}
	sort.Strings(matches)

	parser := NewParser(store)
	for _, path := range matches {
		newLines, ferr := processFileIncremental(store, parser, path)
		if ferr != nil {
			return filesTouched, newLinesTotal, ferr
		}
		if newLines > 0 {
			filesTouched++
			newLinesTotal += newLines
		}
	}
	return filesTouched, newLinesTotal, nil
}

func processFileIncremental(store *Store, parser *Parser, path string) (int, error) {
	fileName := filepath.Base(path)
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	mtime := float64(info.ModTime().UnixNano()) / 1e9

	progress, hadProgress := store.ParseProgress[fileName]
	if hadProgress && progress.Mtime == mtime {
		return 0, nil // unchanged since last run -- skip without even opening it
	}
	alreadyProcessed := 0
	if hadProgress {
		alreadyProcessed = progress.LineCount
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // journal lines can be long (e.g. SellOrganicData)
	lineCount := 0
	newLines := 0
	for scanner.Scan() {
		lineCount++
		if lineCount <= alreadyProcessed {
			continue
		}
		newLines++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		parser.ProcessLine(line)
	}
	if err := scanner.Err(); err != nil {
		return newLines, err
	}

	store.ParseProgress[fileName] = FileProgress{Mtime: mtime, LineCount: lineCount}
	return newLines, nil
}

func openBrowser(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	url := "file://" + abs

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start() // best-effort -- never fail the whole run just because no browser opened
}

// Keeps a double-clicked console window open on Windows long enough to read the output
// (Windows closes the console the instant a launched .exe exits) -- a no-op everywhere else,
// AND a no-op if stdin isn't actually an interactive terminal (a real bug found while testing
// this Windows build under Wine: blocking unconditionally on stdin hangs forever when stdin is
// a pipe/redirected/non-interactive, e.g. run from a script or CI, not just when double-clicked).
// resolveJournalDir is the first thing that happens on every run without --journal-dir: it asks
// before it acts, rather than silently guessing and only presenting a choice once auto-detection
// has already failed. When stdin isn't an interactive terminal (scripted/piped run), it falls
// back to the old one-shot auto-detect-or-fail behavior, since there's nobody there to answer a
// prompt.
func resolveJournalDir() string {
	if !isInteractiveStdin() {
		dir := FindJournalDir()
		if dir == "" {
			printAutoDetectFailure()
			fmt.Println("Pass it explicitly with --journal-dir PATH")
			return ""
		}
		return dir
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("Where's your Elite Dangerous journal folder?")
		fmt.Println("  [Enter]  auto-detect it for me")
		fmt.Println("  or type the full folder path and press Enter")
		fmt.Print("> ")

		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line == "" {
			dir := FindJournalDir()
			if dir != "" {
				return dir
			}
			printAutoDetectFailure()
			fmt.Println("Try typing the path instead.")
			fmt.Println()
			continue
		}

		info, err := os.Stat(line)
		if err != nil || !info.IsDir() {
			fmt.Println("That doesn't look like a folder -- try again.")
			fmt.Println()
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(line, journalGlob))
		if len(matches) == 0 {
			fmt.Println("No Journal.*.log files found in that folder -- try again.")
			fmt.Println()
			continue
		}
		return line
	}
}

func printAutoDetectFailure() {
	candidates := FindJournalDirCandidates()
	if len(candidates) > 0 {
		fmt.Println("Found more than one possible journal folder, can't pick automatically:")
		for _, c := range candidates {
			fmt.Println("  " + c)
		}
	} else {
		fmt.Println("Could not find your Elite Dangerous journal folder automatically.")
	}
}

func isInteractiveStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func pauseOnWindows() {
	if runtime.GOOS != "windows" || !isInteractiveStdin() {
		return
	}
	fmt.Print("\nPress Enter to close this window...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}
