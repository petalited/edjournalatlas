"""Event-by-event journal parser -- builds standalone's own DB directly from raw journal
entries, no ExploData/BioScan/Pioneer dependency. See docs/StandaloneJournalParser.md for the
mechanics this implements and what's grounded against real journal data vs. genuinely uncertain.

Almost every mechanic here is a direct field read off a journal event (confirmed against both
real journal samples and the actual installed ExploData/BioScan/Pioneer source, see the doc) --
the one piece requiring real state accumulation is sold/lost reconciliation, which lives in
reconcile.py, not here.

Feed entries in chronological order (both backfill and live-tailing do this naturally, since
journal files are append-only and read top-to-bottom). This module doesn't enforce ordering
itself -- see venues.py's timestamp-guard lesson if this DB ever needs to tolerate reordering.
"""

from __future__ import annotations

import sqlite3

from . import region_map

# Deaths/resurrections that carry real loss risk for carried exobiology samples -- mirrors
# edexotracker/datasource.py's already-proven LOSS_RISK_RESURRECTION_TYPES constant exactly.
# ExploData's own real source (journal_parse.py) stores the raw journal `Option` field verbatim
# as its `type` column, confirming these are real Option values, not an ExploData-internal
# category -- this commander's own real Resurrect events are all "rebuy" (not in this set), so
# none of them individually carry loss risk under this rule.
LOSS_RISK_RESURRECTION_TYPES = ('escape', 'recover', 'rejoin')


class JournalParser:
    """Holds current parse-time context (current system, commander name) across a stream of
    journal entries fed one at a time via process_entry(). One instance per commander DB."""

    def __init__(self, con: sqlite3.Connection):
        self.con = con
        self.current_system_address: int | None = None
        self.current_system_name: str | None = None
        self.current_commander: str | None = None

    def process_entry(self, entry: dict) -> None:
        event = entry.get('event')
        handler = _HANDLERS.get(event)
        if handler:
            handler(self, entry)

    # -- Commander identity --------------------------------------------------------------

    def _on_commander(self, entry: dict) -> None:
        name = entry.get('Name') or entry.get('Commander')
        fid = entry.get('FID')
        if not name:
            return
        self.current_commander = name
        self.con.execute('DELETE FROM commander')
        self.con.execute('INSERT INTO commander (name, fid) VALUES (?, ?)', (name, fid))
        self.con.commit()

    # -- System / venue-ish facts ---------------------------------------------------------

    def _on_jump_or_location(self, entry: dict) -> None:
        system_address = entry.get('SystemAddress')
        system_name = entry.get('StarSystem')
        if system_address is None or not system_name:
            return
        self.current_system_address = system_address
        self.current_system_name = system_name

        star_pos = entry.get('StarPos')
        region = region_map.find_region(*star_pos) if star_pos else None
        timestamp = entry.get('timestamp', '')

        self.con.execute(
            """
            INSERT INTO systems (system_address, name, x, y, z, region, population, faction,
                                  government, allegiance, security, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(system_address) DO UPDATE SET
                name = excluded.name,
                x = COALESCE(excluded.x, x), y = COALESCE(excluded.y, y), z = COALESCE(excluded.z, z),
                region = COALESCE(excluded.region, region),
                population = COALESCE(excluded.population, population),
                faction = COALESCE(excluded.faction, faction),
                government = COALESCE(excluded.government, government),
                allegiance = COALESCE(excluded.allegiance, allegiance),
                security = COALESCE(excluded.security, security),
                updated_at = excluded.updated_at
            """,
            (
                system_address, system_name,
                star_pos[0] if star_pos else None, star_pos[1] if star_pos else None, star_pos[2] if star_pos else None,
                region, entry.get('Population'), (entry.get('SystemFaction') or {}).get('Name'),
                entry.get('SystemGovernment_Localised'), entry.get('SystemAllegiance') or None,
                entry.get('SystemSecurity_Localised'), timestamp,
            ),
        )
        self.con.commit()

    def _on_fss_discovery_scan(self, entry: dict) -> None:
        # Different key name from every other system-scoped event -- SystemName, not StarSystem.
        system_address = entry.get('SystemAddress')
        if system_address is None:
            return
        fully_scanned = entry.get('Progress', 0) >= 1.0
        self.con.execute(
            """
            INSERT INTO systems (system_address, name, body_count_total, honked, fully_scanned, updated_at)
            VALUES (?, ?, ?, 1, ?, ?)
            ON CONFLICT(system_address) DO UPDATE SET
                body_count_total = excluded.body_count_total,
                honked = 1,
                fully_scanned = MAX(fully_scanned, excluded.fully_scanned),
                updated_at = excluded.updated_at
            """,
            (
                system_address, entry.get('SystemName'), entry.get('BodyCount'),
                1 if fully_scanned else 0, entry.get('timestamp', ''),
            ),
        )
        self.con.commit()

    def _on_colonisation_claim(self, entry: dict) -> None:
        system_address = entry.get('SystemAddress')
        if system_address is None:
            return
        self.con.execute(
            """
            INSERT INTO systems (system_address, name, claimed_by_commander, updated_at)
            VALUES (?, ?, 1, ?)
            ON CONFLICT(system_address) DO UPDATE SET claimed_by_commander = 1, updated_at = excluded.updated_at
            """,
            (system_address, entry.get('StarSystem'), entry.get('timestamp', '')),
        )
        self.con.commit()

    # -- Body/star discovery ---------------------------------------------------------------

    def _on_scan(self, entry: dict) -> None:
        system_address = entry.get('SystemAddress') or self.current_system_address
        body_id = entry.get('BodyID')
        if system_address is None or body_id is None:
            return
        timestamp = entry.get('timestamp', '')

        if 'StarType' in entry:
            self.con.execute(
                """
                INSERT INTO stars (system_address, body_id, name, distance, type, subclass,
                                    luminosity, mass, was_discovered, was_footfalled, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(system_address, body_id) DO UPDATE SET
                    name = excluded.name, distance = excluded.distance, type = excluded.type,
                    subclass = excluded.subclass, luminosity = excluded.luminosity, mass = excluded.mass,
                    was_discovered = excluded.was_discovered, was_footfalled = excluded.was_footfalled,
                    updated_at = excluded.updated_at
                """,
                (
                    system_address, body_id, entry.get('BodyName'), entry.get('DistanceFromArrivalLS'),
                    entry.get('StarType'), entry.get('Subclass'), entry.get('Luminosity'),
                    entry.get('StellarMass'), _bool_to_int(entry.get('WasDiscovered')),
                    _bool_to_int(entry.get('WasFootfalled')), timestamp,
                ),
            )
        elif 'PlanetClass' in entry:
            self.con.execute(
                """
                INSERT INTO planets (system_address, body_id, name, type, landable, mass, distance,
                                      parent_star_body_id, terraform_state, atmosphere, gravity, temp, pressure,
                                      discovered, was_discovered, was_mapped, was_footfalled, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
                ON CONFLICT(system_address, body_id) DO UPDATE SET
                    name = excluded.name, type = excluded.type, landable = excluded.landable,
                    mass = excluded.mass, distance = excluded.distance,
                    parent_star_body_id = excluded.parent_star_body_id,
                    terraform_state = excluded.terraform_state, atmosphere = excluded.atmosphere,
                    gravity = excluded.gravity, temp = excluded.temp, pressure = excluded.pressure,
                    discovered = 1, was_discovered = excluded.was_discovered,
                    was_mapped = excluded.was_mapped, was_footfalled = excluded.was_footfalled,
                    updated_at = excluded.updated_at
                """,
                (
                    system_address, body_id, entry.get('BodyName'), entry.get('PlanetClass'),
                    _bool_to_int(entry.get('Landable')), entry.get('MassEM'), entry.get('DistanceFromArrivalLS'),
                    _parent_star_body_id(entry.get('Parents')),
                    entry.get('TerraformState') or '', entry.get('AtmosphereType') or '',
                    entry.get('SurfaceGravity'), entry.get('SurfaceTemperature'), entry.get('SurfacePressure'),
                    _bool_to_int(entry.get('WasDiscovered')), _bool_to_int(entry.get('WasMapped')),
                    _bool_to_int(entry.get('WasFootfalled')), timestamp,
                ),
            )
        self.con.commit()

    def _on_saa_scan_complete(self, entry: dict) -> None:
        system_address = entry.get('SystemAddress') or self.current_system_address
        body_id = entry.get('BodyID')
        if system_address is None or body_id is None:
            return
        probes_used = entry.get('ProbesUsed')
        efficiency_target = entry.get('EfficiencyTarget')
        efficient = probes_used is not None and efficiency_target is not None and efficiency_target >= probes_used
        self.con.execute(
            """
            INSERT INTO planets (system_address, body_id, mapped, was_mapped, efficient, updated_at)
            VALUES (?, ?, 1, ?, ?, ?)
            ON CONFLICT(system_address, body_id) DO UPDATE SET
                mapped = 1, was_mapped = excluded.was_mapped, efficient = excluded.efficient,
                updated_at = excluded.updated_at
            """,
            (
                system_address, body_id, _bool_to_int(entry.get('WasMapped')),
                1 if efficient else 0, entry.get('timestamp', ''),
            ),
        )
        self.con.commit()

    def _on_signals(self, entry: dict) -> None:
        """FSSBodySignals or SAASignalsFound -- just the biological signal COUNT; genus hints
        (Genuses, present on SAASignalsFound only) aren't stored separately here since the actual
        genus/species identification comes from ScanOrganic, same as edexotracker's own model."""
        system_address = entry.get('SystemAddress') or self.current_system_address
        body_id = entry.get('BodyID')
        if system_address is None or body_id is None:
            return
        bio_count = 0
        for signal in entry.get('Signals') or []:
            if signal.get('Type') == '$SAA_SignalType_Biological;':
                bio_count = signal.get('Count', 0)
        if bio_count == 0:
            return
        self.con.execute(
            """
            INSERT INTO planets (system_address, body_id, bio_signal_count, updated_at)
            VALUES (?, ?, ?, ?)
            ON CONFLICT(system_address, body_id) DO UPDATE SET
                bio_signal_count = excluded.bio_signal_count, updated_at = excluded.updated_at
            """,
            (system_address, body_id, bio_count, entry.get('timestamp', '')),
        )
        self.con.commit()

    def _on_scan_organic(self, entry: dict) -> None:
        # ScanOrganic uses `Body`, every other body-scoped event uses `BodyID` -- a real FDev
        # journal-schema inconsistency, confirmed against real samples.
        system_address = entry.get('SystemAddress') or self.current_system_address
        body_id = entry.get('Body')
        genus = entry.get('Genus')
        species = entry.get('Species')
        if system_address is None or body_id is None or not genus or not species:
            return
        scan_type = entry.get('ScanType')
        count = {'Log': 1, 'Sample': 2, 'Analyse': 3}.get(scan_type, 0)
        was_logged = _bool_to_int(entry.get('WasLogged')) if scan_type == 'Analyse' else None
        self.con.execute(
            """
            INSERT INTO flora_scans (system_address, body_id, genus, species, variant, color,
                                      count, was_logged, scanned_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(system_address, body_id, genus, species) DO UPDATE SET
                variant = excluded.variant,
                count = MAX(count, excluded.count),
                was_logged = COALESCE(excluded.was_logged, was_logged),
                scanned_at = excluded.scanned_at
            """,
            (
                system_address, body_id, genus, species, entry.get('Variant'), None,
                count, was_logged, entry.get('timestamp', ''),
            ),
        )
        self.con.commit()

    # -- Sales / deaths ---------------------------------------------------------------------

    def _on_sell_organic_data(self, entry: dict) -> None:
        self.con.execute(
            'INSERT OR IGNORE INTO exobio_sales (sold_at) VALUES (?)', (entry.get('timestamp', ''),),
        )
        self.con.commit()

    def _on_multi_sell_exploration_data(self, entry: dict) -> None:
        systems = ','.join(d.get('SystemName', '') for d in entry.get('Discovered') or [])
        self.con.execute(
            """
            INSERT OR IGNORE INTO system_sales (sold_at, base_value, bonus, total_earnings, system_names)
            VALUES (?, ?, ?, ?, ?)
            """,
            (entry.get('timestamp', ''), entry.get('BaseValue'), entry.get('Bonus'),
             entry.get('TotalEarnings'), systems),
        )
        self.con.commit()

    def _on_died(self, entry: dict) -> None:
        self.con.execute('INSERT OR IGNORE INTO deaths (died_at, in_ship) VALUES (?, NULL)', (entry.get('timestamp', ''),))
        self.con.commit()

    def _on_resurrect(self, entry: dict) -> None:
        self.con.execute(
            'INSERT OR IGNORE INTO resurrections (resurrected_at, option) VALUES (?, ?)',
            (entry.get('timestamp', ''), entry.get('Option')),
        )
        self.con.commit()


def _parent_star_body_id(parents: list[dict] | None) -> int | None:
    """`Parents` is an ordered ancestor chain (closest first) keyed by BodyID, e.g.
    `[{'Star': 1}, {'Null': 0}]` -- 'Null' entries are barycenters, not real bodies. Returns the
    first real 'Star' ancestor's BodyID, unresolved (name resolution happens at read time in
    viewer.py, not here -- see db.py's parent_star_body_id column comment for why: a planet's
    own Scan can fire before its parent star's, confirmed against real data, so a parse-time
    name lookup can permanently miss it). Simplification: only the single nearest star ancestor,
    not a full circumbinary set."""
    if not parents:
        return None
    for entry in parents:
        if 'Star' in entry:
            return entry['Star']
    return None


def _bool_to_int(value: bool | None) -> int | None:
    return None if value is None else int(value)


_HANDLERS = {
    'Commander': JournalParser._on_commander,
    'LoadGame': JournalParser._on_commander,
    'FSDJump': JournalParser._on_jump_or_location,
    'Location': JournalParser._on_jump_or_location,
    'CarrierJump': JournalParser._on_jump_or_location,
    'FSSDiscoveryScan': JournalParser._on_fss_discovery_scan,
    'ColonisationSystemClaim': JournalParser._on_colonisation_claim,
    'Scan': JournalParser._on_scan,
    'SAAScanComplete': JournalParser._on_saa_scan_complete,
    'FSSBodySignals': JournalParser._on_signals,
    'SAASignalsFound': JournalParser._on_signals,
    'ScanOrganic': JournalParser._on_scan_organic,
    'SellOrganicData': JournalParser._on_sell_organic_data,
    'MultiSellExplorationData': JournalParser._on_multi_sell_exploration_data,
    'Died': JournalParser._on_died,
    'Resurrect': JournalParser._on_resurrect,
}
