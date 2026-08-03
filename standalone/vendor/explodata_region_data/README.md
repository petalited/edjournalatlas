# Vendored region-lookup data (from EDMC-ExploData)

Source: https://github.com/Silarn/EDMC-ExploData (GPLv2) — `src/ExploData/explo_data/
RegionMapData.py`. Same vendoring precedent already used elsewhere in this project (BioScan's
value table, ExploData's genus names): unmodified copy of static reference data, not code
logic — `region_map.py` (sibling module, not vendored) re-implements the small lookup function
that reads it, so this directory holds only the data table itself.

`regions` is a list of region display names indexed by region ID; `regionmap` is the
run-length-encoded coordinate grid `findRegion(x, y, z)` walks to find which region ID a given
galactic coordinate falls in. See ExploData's own `RegionMap.py` for the original
implementation this project's `region_map.py` is a plain-language reimplementation of — fully
offline, deterministic, no network call (ExploData's original file also has an EDSM-API-backed
fallback function for system-name lookups; standalone never needs that path, since it always
has `StarPos` directly from the journal).
