# edjournalatlas

A local, offline toolkit for Elite Dangerous, built entirely from your own journal files — no
account, no companion app, no network calls. Seven browsable pages:

- **Systems explorer** — every system you've visited: region, population, bodies scanned, notable
  finds, presumed exobiology value; drill into stars/moons/exobiology detail per body.
- **Journal search** — every event you've ever logged, not just exploration ones, free-text
  searchable.
- **Career recap** — a "wrapped"-style stat summary across 17 categories (Combat, Crime, Wing,
  Powerplay, Trading, Mining, Engineering, Crew, Stations, Missions, and more); click any stat for
  its full real breakdown, not just the headline number. A separate "last session" recap shows only
  what actually changed since you last loaded in.
- **Materials inventory** — current Raw/Manufactured/Encoded holdings, grouped into the game's own
  engineering families.
- **Engineering planner** — queue up a basket of blueprint upgrades and see exactly what materials
  and engineers each one needs, correctly accounting for every lower grade a high-grade roll costs.
- **Shipyard** — your real fleet (active + stored), fitted modules, per-ship engineering completion
  %, and export to EDSY/Coriolis.
- **Powerplay** — current standing, a full merit/delivery/collection history log, a real
  cumulative-merits chart, and a weekly-cycle viewer for reviewing any past Powerplay cycle.

A trade calculator (materials page and engineering planner) ties the inventory and the planner
together — short on a material, click it to see what you could give up instead, and the nearest
Material Trader (of the right type) you've actually used before.

**Nothing else has to be installed, no account needed, nothing is ever sent anywhere over the
network** — it only reads files already on your own computer, and only ever writes its own small
local cache file next to itself.

Two implementations exist, but they're not equivalent anymore:

- **`standalone-go/`** — a single ~3MB self-contained binary (Linux/Windows), no runtime
  dependency of any kind. **This is the actively developed, full-featured version** — all seven
  pages above, plus a small coverage-report JSON file listing which journal event types aren't
  summarized yet (safe to share: event names and field names only, never your actual data). No
  Mac build: Elite Dangerous has no supported native Mac client anymore, so there's no realistic
  audience with journal files to point one at.
- **`standalone/`** — the original Python version. Still works, but only has the systems viewer
  (no full-journal search, no career recap, no materials/engineering pages) — it's no longer being
  extended. Use it if you'd rather run from source with just the stdlib, but the Go version is
  where new features land.

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

### Windows Defender false positives

Windows may flag `edjournalatlas-windows.exe` (commonly as `Trojan:Win32/Wacatac.B!ml`) even
though nothing here does anything a real trojan does — it never touches the network, never
modifies anything outside its own folder, and its full source is right here. This isn't a real
detection, it's Defender's ML heuristic reacting to the *shape* of the binary: a small, unsigned,
freshly-built Go executable looks structurally similar to a lot of actual malware to that
heuristic, regardless of what it actually does. The release build already embeds real version
info/manifest metadata (`winres/winres.json`) specifically because a completely bare, metadata-
less exe is even more likely to trip it — but that only reduces the odds, it doesn't eliminate
them, and there isn't really a full fix available: a proper fix (code-signing certificate + built-
up reputation over many downloads) costs real money for a free personal project. If it gets
flagged for you, the actual free options are: restore it from quarantine and add an exclusion, or
[submit it to Microsoft as a false positive](https://www.microsoft.com/en-us/wdsi/filesubmission)
(this genuinely can get a specific build reclassified, it just isn't instant). Building from
source yourself (below) sidesteps this entirely, since you're not running a binary someone else
built.

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

## The seven pages (Go version)

Running the binary writes/updates all seven every time — a small pill-tab bar at the top of each
page switches between them. Every page also has a light/dark toggle (top-right corner) that
overrides your OS/browser preference and follows you across pages, in case the automatic
light/dark detection isn't what you want.

**`standalone_viewer.html` — systems explorer.** A sortable/searchable table of every visited
system: region, population, bodies scanned, notable-find count, presumed exobiology value. Click
a system to expand it: stars, bodies grouped under whichever star (or shared binary-star
barycenter — circumbinary bodies are grouped correctly, not just attached to the nearest single
star) they orbit, moons nested under their parent planet (shown as a small connected tree, with
each body's distance from the arrival star), click a body for full detail (mass, gravity,
atmosphere, temperature, pressure, terraform state, full flora-scan history). Bonus/
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
up — nothing forced): Career Earnings, Exploration, Rank, Combat, Crime, Wing, Powerplay, Trading,
Mining, Materials, Engineering, Ships, Fleet Carrier, Crew, Colonization, Stations, Missions. Any
stat built from a real "top of many" aggregate (favourite target, most frequent wingmate, notable
finds, and a dozen more) is clickable — opens the full breakdown, not just the single winner. A
handful of narrative highlights get pulled out too (e.g. who last destroyed you, and what they
were flying), alongside a separate "last session" recap showing only what changed since you last
loaded in. Discord-export buttons for the whole recap, individual sections, and the session recap.

**`standalone_materials.html` — materials inventory.** Your current Engineering materials
(Raw/Manufactured/Encoded) as of the last time the game reported them to the journal, shown either
as a flat sorted/searchable list or a grid grouped into the game's own material families (one
column per grade, so you can see the real shape of a family — what you hold, what's missing).
Click any material to open the trade calculator (see below).

**`standalone_engineering.html` — engineering upgrade planner.** Queue up a "basket" of
engineering upgrades (module, grade, quantity, optional Experimental Effect) and see a combined
summary of every material you'd need — correctly accounting for the real in-game mechanic that
reaching grade N means rolling through every lower grade first (a grade 5 upgrade costs
1+2+3+4+5 = 15 rolls' worth of materials at max engineer rank, not just grade 5's own cost) — held
against your actual inventory, plus which of your unlocked engineers can provide each upgrade
(click an engineer's name to copy their home system for route-plotting). Anything you're short on
opens the same trade calculator a click away.

**Trade calculator** (materials page and engineering planner). Click a tradeable material to see
a family × grade grid of what you could give up instead, color-coded by how good a deal it is
(cheap/plentiful, costly-but-possible, or — on the engineering planner — needed elsewhere in your
own plan, so trading it away would just create a different shortfall), using the real Material
Trader exchange-ratio mechanic (grade difference plus a cross-family penalty, worked out from
community references and cross-checked against real trade examples — see
`standalone-go/materialtrader.go`'s own header comment for the full derivation). A button inside
shows the nearest known trader of the right type — sourced only from stations you've personally
traded materials at before (the journal doesn't record trader *type* any other way, and this tool
never makes a network call to guess), sorted by distance from wherever your journal last placed
you.

**`standalone_shipyard.html` — shipyard / fleet planner.** Your real fleet — whichever ship is
currently active plus every stored ship — as a grid of cards, each showing fitted modules (hover
for a quick preview, click for full detail including Experimental Effects), a real per-ship
engineering completion percentage, and jump range. Push any fitted module's build into the
engineering planner's basket in one click, or export a ship's real current build to
EDSY/Coriolis (and import one back) via the same Journal-loadout format both those tools already
accept.

**`standalone_powerplay.html` — Powerplay status and history.** Current standing (power, rank,
merits, time pledged), real per-system totals for how much influence you've pushed where, and a
full activity log of every merit gain, delivery, collection, and rank-up — each entry colored by
the power it belonged to. A cumulative-merits chart, tinted to your power's own theme color. A
"View by Cycle" button breaks your whole history down by Powerplay's real weekly cycle (resets
every Thursday 07:00 UTC, not the same thing as a system's own BGS tick), so you can review any
past cycle's merits/deliveries/rank-ups on their own — each with its own Discord-export summary,
alongside the page-wide one.

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

## License

GPLv2 (see [`LICENSE`](LICENSE)) — matches the license of most of the vendored reference data this
project builds on (species values from EDMC-BioScan, region data from EDMC-ExploData). The
engineering planner's blueprint data is a filtered copy of
[msarilar/EDEngineer](https://github.com/msarilar/EDEngineer)'s data, MIT licensed — see
`standalone-go/vendor/blueprints_README.md` for details.

## A note on how this was built

Large parts of this codebase were written with AI assistance.
