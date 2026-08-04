# edjournalatlas

A local "explore what you've found" viewer for Elite Dangerous — and, beyond just exploration, a
whole-career recap and a full-journal search tool. It reads your own game journal files directly
and builds three browsable pages: every system you've visited (region, population, bodies
scanned, notable finds, presumed exobiology value), a searchable log of literally every journal
event you've ever generated (combat, trading, missions, engineering, powerplay, crew — not just
exploration), and a "wrapped"-style career recap (kills, favourite ship, biggest rival, trade
profit, engineering, powerplay rank, fleet carrier, crime record, and more).

**Nothing else has to be installed, no account needed, nothing is ever sent anywhere over the
network** — it only reads files already on your own computer, and only ever writes its own small
local cache file next to itself.

Two implementations exist, but they're not equivalent anymore:

- **`standalone-go/`** — a single ~3MB self-contained binary (Linux/Windows), no runtime
  dependency of any kind. **This is the actively developed, full-featured version** — all three
  pages above, plus a small coverage-report JSON file listing which journal event types aren't
  summarized yet (safe to share: event names and field names only, never your actual data). No
  Mac build: Elite Dangerous has no supported native Mac client anymore, so there's no realistic
  audience with journal files to point one at.
- **`standalone/`** — the original Python version. Still works, but only has the systems viewer
  (no full-journal search, no career recap) — it's no longer being extended. Use it if you'd
  rather run from source with just the stdlib, but the Go version is where new features land.

Both keep their own local cache and output files, so it's fine to have both around.

## Running it (pre-built binary)

If you have `edjournalatlas-linux` or `edjournalatlas-windows.exe` — that's the whole program.
Put it in its own folder and double-click it (or run it from a terminal). It'll ask where your
journal folder is (or you can just hit Enter to have it auto-detect), then build/update its own
small local cache and open the viewer in your browser.

Run it again any time — only new journal activity since last time gets processed, not everything
from scratch, so it's fast after the first run.

If auto-detect can't find it (common on Linux, rarer on Windows), just type the path in when
asked — or skip the prompt entirely by passing it up front:

```
edjournalatlas --journal-dir "PATH TO YOUR JOURNAL FOLDER"
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

Builds both platforms in one go (Go cross-compiles natively — no need to run the build
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

## The three pages (Go version)

Running the binary writes/updates all three every time — a small pill-tab bar at the top of each
page switches between them.

**`standalone_viewer.html` — systems explorer.** A sortable/searchable table of every visited
system: region, population, bodies scanned, notable-find count, presumed exobiology value. Click
a system to expand it: stars, bodies grouped under whichever star (or shared binary-star
barycenter — circumbinary bodies are grouped correctly, not just attached to the nearest single
star) they orbit, moons nested under their parent planet, click a body for full detail (mass,
gravity, atmosphere, temperature, pressure, terraform state, full flora-scan history). Bonus/
first-discovery badges, EDSM/Inara/full-journal-search links per system, a 📍 button to re-sort
the whole table by distance from any system, copy-to-clipboard and Discord-formatted export
buttons.

**`standalone_events.html` — full journal search.** Every single journal event you've ever
generated, not just exploration-relevant ones — combat, trading, missions, engineering, cargo,
powerplay, crew, the works. Free-text search or filter by event type, each result expandable to
pretty-printed raw JSON. This is a separate page from the systems viewer specifically because a
real journal history can be tens of thousands of events — keeping it out of the main viewer keeps
that page fast to open every time, even though this one can get large.

**`standalone_summary.html` — career recap.** Stat cards across however many of these
categories your own journal has real activity in (sections you have no data for just don't show
up — nothing forced): Exploration, Rank, Combat, Crime, Wing, Powerplay, Trading, Mining,
Materials, Engineering, Ships, Fleet Carrier, Crew, Colonization, Stations, Missions. A handful of
narrative highlights get pulled out too (e.g. who last destroyed you, and what they were flying).
Has its own Discord-export button for the whole recap.

**`standalone_unmodeled.json`** — not really meant to be read directly. A small coverage
report: which journal event types your data has that the recap doesn't summarize yet, with real
counts and field *names* (never your actual data/values) — useful if you want to request/build
support for something the recap doesn't cover.

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
