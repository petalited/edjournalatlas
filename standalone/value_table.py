"""Loads standalone's own vendored species value table. See vendor/bio_value_data/README.md.
Deliberately identical interface to edexotracker/value_table.py, but reads standalone's own
copies -- no import dependency on the edexotracker package."""

from __future__ import annotations

import json
import os
from typing import NamedTuple, Optional

_VALUE_TABLE_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'vendor', 'bio_value_data', 'value_table.json')
_GENUS_NAMES_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'vendor', 'bio_value_data', 'genus_names.json')


class SpeciesInfo(NamedTuple):
    name: str
    value: int


_TABLE: Optional[dict[str, dict[str, SpeciesInfo]]] = None


def load() -> dict[str, dict[str, SpeciesInfo]]:
    with open(_VALUE_TABLE_PATH) as f:
        raw = json.load(f)
    return {
        genus_key: {
            species_key: SpeciesInfo(name=info['name'], value=info['value'])
            for species_key, info in species_map.items()
        }
        for genus_key, species_map in raw.items()
    }


def lookup(genus: str, species: str) -> Optional[SpeciesInfo]:
    global _TABLE
    if _TABLE is None:
        _TABLE = load()
    if not genus or not species:
        return None
    return _TABLE.get(genus, {}).get(species)


_GENUS_NAMES: Optional[dict[str, str]] = None


def genus_display_name(genus: str) -> str:
    global _GENUS_NAMES
    if _GENUS_NAMES is None:
        with open(_GENUS_NAMES_PATH) as f:
            _GENUS_NAMES = json.load(f)
    return _GENUS_NAMES.get(genus, genus)
