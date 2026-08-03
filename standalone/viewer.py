"""Builds a self-contained local-EDSM-style HTML viewer directly from standalone's own DB --
"which Earthlike worlds have I found," across every system this commander has visited, no
ExploData/BioScan/Pioneer dependency. Same self-contained-single-file, no-server, no-build-step
approach as edexotracker/report.py, and reuses its CSS/JS conventions where they still fit, but
this is deliberately a simpler, standalone-scoped view: no exploration/Cartographics value
(that's the one permanent gap documented in docs/StandaloneJournalParser.md), no
claimed/unclaimed exploration split -- just what's been found and what it's worth if scanned.
"""

from __future__ import annotations

import json
import sqlite3
from datetime import datetime, timezone

from . import reconcile
from .viewer_template import TEMPLATE

# Same notable-body-type mapping as edexotracker/datasource.py's NOTABLE_BODY_TYPES -- kept in
# sync manually, same as that module's own comment about report.py's JS mirror.
NOTABLE_BODY_TYPES = {
    'Earthlike body': 'Earthlike world',
    'Water world': 'Water world',
    'Ammonia world': 'Ammonia world',
    'Water giant': 'Water giant',
}


def _classify_rare_star(type_code: str) -> str | None:
    """Same logic as edexotracker/datasource.py's classify_rare_star -- see that module for the
    real-data grounding of this prefix-matching order."""
    if type_code == 'SupermassiveBlackHole':
        return 'Supermassive black hole'
    if type_code == 'H':
        return 'Black hole'
    if type_code == 'N':
        return 'Neutron star'
    if type_code.startswith('D'):
        return 'White dwarf'
    if type_code.startswith('W'):
        return 'Wolf-Rayet star'
    if type_code == 'AeBe':
        return 'Herbig Ae/Be star'
    if type_code.startswith('C'):
        return 'Carbon star'
    if type_code in ('MS', 'S'):
        return f'{type_code}-type star'
    return None


def _load_data(con: sqlite3.Connection) -> dict:
    flora_by_body: dict[tuple[int, int], list[reconcile.FloraValue]] = {}
    for fv in reconcile.compute_flora_values(con):
        flora_by_body.setdefault((fv.system_address, fv.body_id), []).append(fv)

    systems_out = []
    for srow in con.execute('SELECT * FROM systems ORDER BY name'):
        system_address = srow['system_address']

        stars = []
        star_name_by_body_id: dict[int, str] = {}
        for st in con.execute('SELECT * FROM stars WHERE system_address = ? ORDER BY distance', (system_address,)):
            star_name_by_body_id[st['body_id']] = st['name']
            stars.append({
                'name': st['name'], 'distance': st['distance'], 'type': st['type'],
                'subclass': st['subclass'], 'luminosity': st['luminosity'],
                'wasDiscovered': bool(st['was_discovered']) if st['was_discovered'] is not None else None,
                'notableLabel': _classify_rare_star(st['type']) if st['type'] else None,
            })

        bodies = []
        for prow in con.execute('SELECT * FROM planets WHERE system_address = ? ORDER BY distance', (system_address,)):
            flora_values = flora_by_body.get((system_address, prow['body_id']), [])
            # Resolved here, at read time, rather than at parse time -- see db.py's
            # parent_star_body_id column comment for why (a planet's own Scan can fire before
            # its parent star's, so by parse time the star might not exist yet; by the time we
            # get here, every star this commander has ever recorded is guaranteed loaded).
            parent_star_name = star_name_by_body_id.get(prow['parent_star_body_id'], '')
            bodies.append({
                'name': prow['name'], 'type': prow['type'], 'landable': bool(prow['landable']),
                'mass': prow['mass'], 'distance': prow['distance'], 'parentStars': parent_star_name,
                'terraformable': prow['terraform_state'] in ('Terraformable', 'Terraforming', 'Terraformed'),
                'discovered': bool(prow['discovered']),
                'wasDiscovered': bool(prow['was_discovered']) if prow['was_discovered'] is not None else None,
                'mapped': bool(prow['mapped']), 'efficient': bool(prow['efficient']),
                'wasFootfalled': bool(prow['was_footfalled']) if prow['was_footfalled'] is not None else None,
                'bioSignalCount': prow['bio_signal_count'] or 0,
                'notableLabel': NOTABLE_BODY_TYPES.get(prow['type']),
                'flora': [
                    {
                        'genus': fv.genus_name, 'species': fv.species_name, 'count': fv.count,
                        'value': fv.value, 'baseValue': fv.base_value,
                        'sold': fv.sold, 'lost': fv.lost,
                        'footfallBonus': fv.footfall_bonus, 'firstLoggedBonus': fv.first_logged_bonus,
                    }
                    for fv in flora_values
                ],
            })

        notable_counts: dict[str, int] = {}
        for b in bodies:
            if b['notableLabel']:
                notable_counts[b['notableLabel']] = notable_counts.get(b['notableLabel'], 0) + 1
        for st in stars:
            if st['notableLabel']:
                notable_counts[st['notableLabel']] = notable_counts.get(st['notableLabel'], 0) + 1

        first_discovery_count = (
            sum(1 for b in bodies if b['discovered'] and b['wasDiscovered'] is False)
            + sum(1 for st in stars if st['wasDiscovered'] is False)
        )
        bio_value = sum(
            f['value'] for b in bodies for f in b['flora'] if f['value'] and not f['lost']
        )

        systems_out.append({
            'name': srow['name'], 'x': srow['x'], 'y': srow['y'], 'z': srow['z'],
            'region': srow['region'], 'population': srow['population'] or 0,
            'faction': srow['faction'], 'bodyCountTotal': srow['body_count_total'] or 0,
            'recordedBodyCount': len(bodies) + len(stars),
            'claimedByCommander': bool(srow['claimed_by_commander']),
            'notableCounts': notable_counts,
            'firstDiscoveryCount': first_discovery_count,
            'bioValue': bio_value,
            'stars': stars,
            'bodies': bodies,
        })

    return {
        'generatedAt': datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M UTC'),
        'systems': systems_out,
    }


def render(con: sqlite3.Connection) -> str:
    data = _load_data(con)
    data_json = json.dumps(data).replace('</', '<\\/')
    return TEMPLATE.replace('__DATA_JSON__', data_json)
