package main

// Best-effort auto-detection of the Elite Dangerous journal folder -- ported from
// edexotracker's standalone/journal_locate.py. Same reasoning: no single standard path exists
// on Linux (unlike Windows/Mac's fixed locations), so this is best-effort with an explicit
// override otherwise, same problem EDMC itself has to solve.

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

const journalGlob = "Journal.*.log"

func candidateDirs() []string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			userProfile = home
		}
		return []string{filepath.Join(userProfile, "Saved Games", "Frontier Developments", "Elite Dangerous")}

	case "darwin":
		// Not verified against a real Mac install (none available in this project's
		// development environment) -- ED's documented Mac save-data convention.
		return []string{filepath.Join(home, "Library", "Application Support", "Frontier Developments", "Elite Dangerous")}

	default:
		// Linux has no single standard location -- glob across common Proton/Wine/Lutris
		// layouts. The Steam/Proton pattern is confirmed against a real install; the others
		// are reasonable, unverified guesses at equally common setups.
		patterns := []string{
			filepath.Join(home, ".local", "share", "Steam", "steamapps", "compatdata", "*", "pfx",
				"drive_c", "users", "*", "Saved Games", "Frontier Developments", "Elite Dangerous"),
			filepath.Join(home, ".steam", "steam", "steamapps", "compatdata", "*", "pfx",
				"drive_c", "users", "*", "Saved Games", "Frontier Developments", "Elite Dangerous"),
			filepath.Join(home, ".wine", "drive_c", "users", "*", "Saved Games", "Frontier Developments", "Elite Dangerous"),
			filepath.Join(home, "Games", "*", "drive_c", "users", "*", "Saved Games", "Frontier Developments", "Elite Dangerous"),
		}
		var found []string
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(pattern)
			found = append(found, matches...)
		}
		return found
	}
}

// FindJournalDirCandidates returns all distinct real directories (deduped via realpath, e.g.
// ~/.steam/steam is commonly a symlink to ~/.local/share/Steam) that actually contain journal
// files.
func FindJournalDirCandidates() []string {
	seen := make(map[string]bool)
	var result []string
	for _, d := range candidateDirs() {
		info, err := os.Stat(d)
		if err != nil || !info.IsDir() {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(d, journalGlob))
		if len(matches) == 0 {
			continue
		}
		real, err := filepath.EvalSymlinks(d)
		if err != nil {
			real = d
		}
		if !seen[real] {
			seen[real] = true
			result = append(result, real)
		}
	}
	sort.Strings(result)
	return result
}

// FindJournalDir returns the journal directory if exactly one unambiguous candidate is found;
// empty string otherwise (caller should fall back to an explicit path).
func FindJournalDir() string {
	candidates := FindJournalDirCandidates()
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}
