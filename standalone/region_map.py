"""Offline (x, y, z) -> galactic region name lookup. No network call -- confirmed against
ExploData's real installed source that this is fully derivable from a FSDJump/Location event's
own StarPos field alone; ExploData's identical function also has an EDSM-API fallback for
name-only lookups, which standalone never needs since it always has StarPos directly.

Reimplemented from ExploData's RegionMap.py's findRegion() (GPLv2) -- see
vendor/explodata_region_data/README.md for the vendored coordinate table this reads.
"""

from __future__ import annotations

from .vendor.explodata_region_data.RegionMapData import regionmap, regions

# Galactic coordinate grid origin -- same constants ExploData's own findRegion() uses; the grid
# is a fixed 83-column run-length-encoded map of the galaxy starting at this offset.
_X0 = -49985
_Z0 = -24105


def find_region(x: float, y: float, z: float) -> str | None:
    """Region display name for a galactic coordinate, or None if it falls outside the mapped
    grid (e.g. far outside the charted galaxy)."""
    px = int((x - _X0) * 83 / 4096)
    pz = int((z - _Z0) * 83 / 4096)

    if px < 0 or pz < 0 or pz >= len(regionmap):
        return None

    row = regionmap[pz]
    rx = 0
    region_id = 0
    for run_length, value in row:
        if px < rx + run_length:
            region_id = value
            break
        rx += run_length

    if region_id == 0:
        return None
    return regions[region_id]
