# edjournalatlas

A local "explore what you've found" viewer for Elite Dangerous. It reads your own game journal
files directly and builds a browsable page showing every system you've visited: region,
population, how many bodies you've scanned, notable finds (Earthlike/water/ammonia worlds, rare
star types, first discoveries), and a presumed credit value for anything biological you've
scanned.

**Nothing else has to be installed, no account needed, nothing is ever sent anywhere over the
network** — it only reads files already on your own computer, and only ever writes its own small
local cache file next to itself.

Two implementations of the exact same tool are included, so pick whichever fits:

- **`standalone-go/`** — a single ~3MB self-contained binary (Linux/Windows/Mac Intel/Mac Apple
  Silicon), no runtime dependency of any kind. This is the one to grab if you just want to run
  it.
- **`standalone/`** — the original Python version. Same features, same output, no packages to
  install (stdlib only) if running from source; also buildable into a double-click program via
  PyInstaller, at the cost of a larger file (~11.6MB) and builds that only work on whatever OS
  you build them from.

Both keep their own local cache and viewer output, so it's fine to have both around, or to
switch between them freely.

## Running it (pre-built binary)

If you have `edexotracker-standalone-linux` / `-mac-intel` / `-mac-arm` / `-windows.exe` — that's
the whole program. Put it in its own folder and double-click it (or run it from a terminal). It
finds your journal, builds/updates its own small local cache, and opens the viewer in your
browser.

Run it again any time — only new journal activity since last time gets processed, not everything
from scratch, so it's fast after the first run.

If it can't find your journal folder automatically (common on Linux, rarer on Windows/Mac), it'll
ask you to point it there directly:

```
edexotracker-standalone --journal-dir "PATH TO YOUR JOURNAL FOLDER"
```

Your journal folder is normally named `Elite Dangerous`, under a `Saved Games` folder somewhere.
If you already use EDMC or another Elite Dangerous tool, its settings usually show the exact path
it's using — point this at the same one.

## Building it yourself

**Go version** — requires only the Go toolchain, no external modules:

```
cd standalone-go
./build.sh
```

Builds all four platforms in one go (Go cross-compiles natively — no need to run the build
separately per OS). Optional: install
[`go-winres`](https://github.com/tc-hib/go-winres)
(`go install github.com/tc-hib/go-winres@latest`) first — `build.sh` will use it automatically to
embed a proper version-info/manifest resource into the Windows build. Skipping it still produces
a working `.exe`, just one with no version metadata at all, which is the kind of "bare" PE that
some antivirus heuristics (Windows Defender's `Wacatac.B!ml` in particular) are more likely to
flag as suspicious purely on structural grounds.

**Python version** — requires Python 3.9+, no packages to install:

```
python3 standalone/run.py
```

`standalone/parse_journals.py` and `standalone/build_viewer.py` also work standalone if you want
to run just one step (e.g. rebuild the viewer without re-checking the journal). To build a
double-click program from it, see `standalone/build_executable.py` (uses PyInstaller; only
produces a binary for whichever OS you run the build on).

## What's in the viewer

- A sortable/searchable table of every visited system: region, population, bodies scanned,
  notable-find count, presumed exobiology value.
- Click a system to expand it: stars, bodies grouped under whichever star they orbit (moons
  nested under their parent planet), bonus/first-discovery badges, and — for anything with life
  on it — what's been scanned, sold, or lost.
- Filters for "only notable finds" / "only has bio value", a search box, and a reset-filters
  button.
- Copy-to-clipboard and Discord-formatted export buttons per system.

## Honest limitations (not bugs — the game just doesn't record this)

- **The value of biological samples you're still carrying** (not yet sold) comes from a
  community-maintained reference table of known species values (`vendor/`), not your journal —
  the game never writes down a price for something unsold. Bundled in already, but it's a
  snapshot; a game update adding new life forms would need this table refreshed.
- **Exploration/Cartographics value has no sold-vs-unsold split at all.** Selling exploration data
  for multiple systems at once only produces one combined total in the journal — there's no way
  to attribute it back to individual systems. So exploration value here is always a "presumed if
  sold" figure.

## Everything else

Read-only: it only ever reads your journal files and its own local cache — never touches your
game, your saves, or anything else on your machine.
