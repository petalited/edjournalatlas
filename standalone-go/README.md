# standalone (Go rewrite)

A local "explore what you've found" viewer for Elite Dangerous — same tool as `../standalone/`
(the Python version), rewritten in Go to be as light as possible: a single ~3MB file with the
entire Go runtime built in, vs. ~11.6MB for the Python/PyInstaller build. No Python, no
installer, no dependencies of any kind on the machine that runs it.

**This folder is everything you need.** No account, nothing sent over the network — it only
reads files already on your own computer.

## How to use it

If you were given a single file called `edjournalatlas-linux` / `-mac-intel` /
`-mac-arm` / `-windows.exe` — that's the whole program. Put it in its own folder and
double-click it (or run it from a terminal). It finds your journal, builds/updates its own
small local file, and opens the viewer in your browser.

**Run it again any time** — it's fast after the first run (only new journal activity gets
processed, not everything from scratch).

### If it can't find your journal folder automatically

Same as the Python version: it guesses the standard location for your OS, and if it can't pick
exactly one unambiguous folder (common on Linux, rare on Windows/Mac), it'll ask you to point it
there directly:

```
edjournalatlas --journal-dir "PATH TO YOUR JOURNAL FOLDER"
```

## Building it yourself

Requires the Go toolchain (`https://go.dev/dl/`) — nothing else, no external Go modules. From
this directory:

```
./build.sh
```

This builds **all four platforms in one go** (Linux, Windows, Mac Intel, Mac Apple Silicon) —
Go can cross-compile with zero extra setup, which is the main reason this is a genuine
improvement over the Python/PyInstaller build: that one can only ever produce a binary for
whatever OS actually runs the build.

Optional but recommended: install [`go-winres`](https://github.com/tc-hib/go-winres)
(`go install github.com/tc-hib/go-winres@latest`) before running `build.sh` — it embeds the
version-info/manifest resource in `winres/winres.json` into the Windows build. Without it, the
Windows `.exe` builds fine but is a "bare" PE with no product/version metadata, which is exactly
the shape Windows Defender's ML heuristic tends to flag as `Trojan:Win32/Wacatac.B!ml` (a common
false positive for small unsigned Go binaries in general).

## Why a second implementation exists

The Python version (`../standalone/`) works and is fully verified, but this Go version was built
to be as light as possible. It:
- Is roughly a third the size (~3MB vs ~11.6MB, both compressed/stripped)
- Builds for all 4 platforms from one machine, instead of needing to run the build separately
  on each OS
- Has no SQLite dependency at all — the local cache is a single JSON file (`encoding/json`), not
  a database file, since a full SQL engine wasn't needed just to persist a handful of Go structs
  between runs
- Was cross-checked number-for-number against the Python version's already-verified real data
  (same commander, same journal history)

One real bug was found and fixed *because* of writing this in Go that the Python version never
hit: Elite Dangerous's journal reuses the field name `"Body"` across incompatible event types
with different value types (a number on `ScanOrganic`, a string on `Location`/`Screenshot`). Go's
strict typing caught this immediately (a hard unmarshal error) where Python's loose `dict.get()`
never would have — worth knowing if this project's journal-parsing logic is ever ported to
another strictly-typed language again.
