# Vendored engineering blueprint data

`blueprints.json` here is a filtered, trimmed copy of
[msarilar/EDEngineer](https://github.com/msarilar/EDEngineer)'s
`EDEngineer/Resources/Data/blueprints.json` (MIT licensed), fetched 2026-08-04, used by
`engineeringplanner.go` to power the engineering upgrade planner page.

Copyright (c) msarilar and EDEngineer contributors. Used here under the MIT License:
https://github.com/msarilar/EDEngineer/blob/master/LICENSE

## Shape

`{"blueprints": [...786 entries...], "effects": [...154 entries...]}`. Each entry has `type`,
`name`, `engineers`, `ingredients` (`ingredients[].key` already resolved to this project's own
internal material key, see below); `blueprints` entries additionally have `grade` (1-5) --
`effects` entries don't, since an Experimental Effect isn't tied to a specific grade of the base
upgrade.

## What changed from the upstream file

The upstream file has 1172 entries covering every blueprint AND every Experimental Effect (and
some Odyssey/Guardian-adjacent entries this project can't use). Split and filtered into the two
lists above:

**`blueprints`** (786 of the upstream's 907 `Grade`-having entries):
- Dropped every entry whose `Type` is `"Suit"` or `"Weapon"` (Odyssey on-foot suit/weapon
  engineering -- a separate resource system this project has no inventory data for at all;
  confirmed at vendoring time that every one of their ingredients, e.g. "Aerogel",
  "Microelectrode", "Weapon Schematic", doesn't match anything in this project's own
  materials table).
- Dropped every entry whose only `Engineers` value is `"@Synthesis"` (ammo synthesis, craftable
  anywhere without visiting an engineer -- doesn't fit "which engineer can give me this").

**`effects`** (154 of the upstream's 265 grade-less entries):
- Dropped every entry whose only `Engineers` value is `"@Technology"` (Technology Broker
  unlocks, not a real Engineer).
- Dropped every entry whose `Type` is `"Unlock"` (these are ENGINEER INVITE requirements, e.g.
  50x Aberrant Shield Pattern Analysis to unlock The Sarge -- a different concept entirely from
  applying an effect to a module you already have).
- Dropped `"Suit"`/`"Weapon"` types, same reasoning as above.

Both lists, after filtering:
- Trimmed each remaining entry down to `type`, `name`, (`grade` for blueprints only),
  `engineers`, `ingredients` (dropped `Effects` and `CoriolisGuid` -- not used by the planner).
- Resolved every `Ingredients[].Name` to this project's own internal material key (see
  `materialgrades.go`) at vendoring time, so `engineeringplanner.go` doesn't need its own
  separate display-name matching table. Two of the upstream names didn't match this project's
  table verbatim and needed an explicit alias: `"Abnormal Compact Emission Data"` (upstream's
  singular "Emission" vs. the real material's plural "Emissions Data") and
  `"Guardian Wreckage Components"` (upstream's shorthand for the real material
  "Guardian Sentinel Wreckage Components").

## To refresh after a game update

Re-fetch `EDEngineer/Resources/Data/blueprints.json` from the upstream repo, then re-run the same
filter/trim/resolve steps above (done as a one-off Python script during vendoring, not checked
into this repo -- straightforward to redo: split on `Grade != null`; filter `blueprints` on
`Type not in ("Suit", "Weapon")` and `Engineers != ["@Synthesis"]`; filter `effects` on
`Type not in ("Suit", "Weapon", "Unlock")` and `Engineers != ["@Technology"]`; then resolve each
ingredient's `Name` against `materialDisplayNames` in `materialgrades.go`, normalizing both sides
the same way `normalizeMaterialKey` does).
