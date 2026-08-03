"""Exobiology value/sold/lost reconciliation -- ported/adapted from
edexotracker/datasource.py's already-proven _reconcile_sale_status()/value-formula logic (see
docs/ExobiologyValueMechanics.md for the full mechanics writeup this implements), rebuilt here
against standalone's own schema so this project has zero import dependency on the edexotracker
package.

This is the one piece of standalone flagged as needing real care rather than a direct field
read -- SellOrganicData's BioData array has no body/system reference at all (confirmed real,
see docs/StandaloneJournalParser.md), so "was this scan sold" is inferred the same way BioScan
itself does: any sale after this scan completed and before a loss-risk event counts as sold.
"""

from __future__ import annotations

import bisect
import sqlite3

from . import value_table

FOOTFALL_BONUS_MULTIPLIER = 4
FIRST_LOGGED_BONUS_MULTIPLIER = 4

# Mirrors datasource.py's LOSS_RISK_RESURRECTION_TYPES exactly -- see journal_parse.py's own
# copy of this constant for the real-data grounding (this commander's own Resurrect events are
# all "rebuy", which is NOT in this set).
LOSS_RISK_RESURRECTION_TYPES = ('escape', 'recover', 'rejoin')


def _reconcile_sale_status(scanned_at: str | None, sale_times: list[str], loss_times: list[str]) -> tuple[bool, bool]:
    """Returns (sold, lost) for one completed (Analyse) flora scan, given this commander's
    sorted sale timestamps and sorted loss-risk-event timestamps (deaths + risky
    resurrections). Journal timestamps are ISO8601 UTC strings, which sort correctly as plain
    strings -- same property datasource.py's own timestamp format relies on."""
    if scanned_at is None:
        return False, False
    lost_idx = bisect.bisect_right(loss_times, scanned_at)
    lost_date = loss_times[lost_idx] if lost_idx < len(loss_times) else None
    sale_idx = bisect.bisect_right(sale_times, scanned_at)
    next_sale = sale_times[sale_idx] if sale_idx < len(sale_times) else None
    if lost_date is not None:
        sold = next_sale is not None and next_sale < lost_date
        return sold, not sold
    return next_sale is not None, False


class FloraValue:
    __slots__ = (
        'system_address', 'body_id',
        'genus', 'species', 'variant', 'genus_name', 'species_name', 'count', 'was_logged',
        'base_value', 'footfall_bonus', 'first_logged_bonus', 'value', 'sold', 'lost',
        'predicted_min', 'predicted_max',
    )

    def __init__(self, **kwargs):
        for k, v in kwargs.items():
            setattr(self, k, v)


def compute_flora_values(con: sqlite3.Connection) -> list[FloraValue]:
    """One FloraValue per (system, body, genus, species) flora_scans row, with value/sold/lost
    fully resolved. Loads all sale/loss timestamps once, same batched approach datasource.py
    uses rather than a query per scan."""
    sale_times = sorted(row['sold_at'] for row in con.execute('SELECT sold_at FROM exobio_sales'))
    death_times = [row['died_at'] for row in con.execute('SELECT died_at FROM deaths')]
    placeholders = ','.join('?' for _ in LOSS_RISK_RESURRECTION_TYPES)
    resurrection_times = [
        row['resurrected_at'] for row in con.execute(
            f'SELECT resurrected_at FROM resurrections WHERE option IN ({placeholders})',
            LOSS_RISK_RESURRECTION_TYPES,
        )
    ]
    loss_times = sorted(death_times + resurrection_times)

    results: list[FloraValue] = []
    for row in con.execute(
        """
        SELECT f.*, p.was_footfalled, s.population
        FROM flora_scans f
        JOIN planets p ON p.system_address = f.system_address AND p.body_id = f.body_id
        JOIN systems s ON s.system_address = f.system_address
        """
    ):
        genus, species = row['genus'], row['species']
        info = value_table.lookup(genus, species)
        base_value = info.value if info else None
        species_name = info.name if info else None
        genus_name = value_table.genus_display_name(genus)

        was_footfalled = bool(row['was_footfalled']) if row['was_footfalled'] is not None else False
        population = row['population'] if row['population'] is not None else 0
        footfall_bonus = (not was_footfalled) and population == 0
        first_logged_bonus = row['was_logged'] == 0  # False, not None/unknown
        bonus_units = (
            (FOOTFALL_BONUS_MULTIPLIER if footfall_bonus else 0)
            + (FIRST_LOGGED_BONUS_MULTIPLIER if first_logged_bonus else 0)
        )
        count = row['count']
        value = base_value * (1 + bonus_units) if (count == 3 and base_value is not None) else None
        sold, lost = (
            _reconcile_sale_status(row['scanned_at'], sale_times, loss_times) if count == 3 else (False, False)
        )
        predicted_range = value_table.species_value_range(genus) if not species else None
        predicted_min, predicted_max = predicted_range if predicted_range else (None, None)

        results.append(FloraValue(
            system_address=row['system_address'], body_id=row['body_id'],
            genus=genus, species=species, variant=row['variant'],
            genus_name=genus_name, species_name=species_name,
            count=count, was_logged=bool(row['was_logged']) if row['was_logged'] is not None else None,
            base_value=base_value,
            footfall_bonus=footfall_bonus and count == 3,
            first_logged_bonus=first_logged_bonus and count == 3,
            value=value, sold=sold, lost=lost,
            predicted_min=predicted_min, predicted_max=predicted_max,
        ))
    return results
