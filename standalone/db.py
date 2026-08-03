"""Schema + connection helper for standalone's own local DB.

Unlike edexotracker/datasource.py's explodata.db (a live file owned by someone else's plugin,
always read-only), this file is entirely ours -- read-write throughout. Deliberately a
SINGLE-COMMANDER schema (no commander_id join tables at all) -- each standalone DB belongs to
exactly one commander on one machine, the same simplifying assumption edexotracker/venues.py
already makes. See docs/StandaloneJournalParser.md for why this project exists and what it can
and can't derive from the journal alone.
"""

from __future__ import annotations

import os
import sqlite3

SCHEMA_VERSION = 1

_SCHEMA = """
CREATE TABLE IF NOT EXISTS meta (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS parse_progress (
    -- Incremental-reparse bookkeeping: one row per journal file. `line_count` is how many
    -- lines of this file we've already processed -- journal files are append-only while a
    -- session is live, so a file that has grown just needs the NEW lines processed, and a file
    -- whose mtime hasn't changed since last run can be skipped entirely. See parse_journals.py.
    file_name TEXT PRIMARY KEY,
    file_mtime REAL,
    line_count INTEGER
);

CREATE TABLE IF NOT EXISTS commander (
    -- Single row -- the one commander this DB belongs to.
    name TEXT,
    fid TEXT
);

CREATE TABLE IF NOT EXISTS systems (
    system_address INTEGER PRIMARY KEY,
    name TEXT,
    x REAL, y REAL, z REAL,
    region TEXT,
    population INTEGER,
    faction TEXT,
    government TEXT,
    allegiance TEXT,
    security TEXT,
    claimed_by_commander INTEGER DEFAULT 0,
    body_count_total INTEGER,  -- from FSSDiscoveryScan's own BodyCount, includes stars
    honked INTEGER DEFAULT 0,
    fully_scanned INTEGER DEFAULT 0,
    updated_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_systems_name ON systems(name);

CREATE TABLE IF NOT EXISTS stars (
    system_address INTEGER,
    body_id INTEGER,
    name TEXT,
    distance REAL,
    type TEXT,
    subclass INTEGER,
    luminosity TEXT,
    mass REAL,
    was_discovered INTEGER,
    was_footfalled INTEGER,
    updated_at TEXT,
    PRIMARY KEY (system_address, body_id)
);

CREATE TABLE IF NOT EXISTS planets (
    system_address INTEGER,
    body_id INTEGER,
    name TEXT,
    type TEXT,
    landable INTEGER,
    mass REAL,
    distance REAL,
    parent_star_body_id INTEGER,  -- nearest parent star's BodyID (from Scan's own Parents field)
                                   -- -- resolved to a NAME only at read time (see viewer.py), never
                                   -- at parse time: confirmed against real data that a planet's own
                                   -- AutoScan can fire chronologically BEFORE its parent star's own
                                   -- Scan event (e.g. real system 2MASS J21394793+5726427: body "A 1"
                                   -- scanned at 01:35:00, star "A" not scanned until 01:35:02) -- a
                                   -- parse-time name lookup would silently and permanently miss the
                                   -- star in that case, since this body only ever gets scanned once.
    terraform_state TEXT,
    atmosphere TEXT,
    gravity REAL,
    temp REAL,
    pressure REAL,
    discovered INTEGER,
    was_discovered INTEGER,
    mapped INTEGER,
    was_mapped INTEGER,
    efficient INTEGER,
    was_footfalled INTEGER,
    bio_signal_count INTEGER DEFAULT 0,
    updated_at TEXT,
    PRIMARY KEY (system_address, body_id)
);

CREATE TABLE IF NOT EXISTS flora_scans (
    system_address INTEGER,
    body_id INTEGER,
    genus TEXT,
    species TEXT,
    variant TEXT,
    color TEXT,
    count INTEGER DEFAULT 0,  -- 0=genus hint only, 1=Log, 2=Sample, 3=Analyse (complete)
    was_logged INTEGER,       -- from the completing (Analyse) ScanOrganic's own WasLogged field
    scanned_at TEXT,          -- timestamp of the most recent (highest-count) scan step seen
    PRIMARY KEY (system_address, body_id, genus, species)
);

CREATE TABLE IF NOT EXISTS exobio_sales (
    sold_at TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS deaths (
    died_at TEXT PRIMARY KEY,
    in_ship INTEGER
);

CREATE TABLE IF NOT EXISTS resurrections (
    resurrected_at TEXT PRIMARY KEY,
    option TEXT  -- e.g. "rebuy" -- only some resurrection types carry loss risk, see reconcile.py
);

CREATE TABLE IF NOT EXISTS system_sales (
    -- MultiSellExplorationData: aggregate-only, no per-system/per-body breakdown available at
    -- all (confirmed against a real event -- see docs/StandaloneJournalParser.md's permanent-gap
    -- note). Kept for a historical "total exploration credits earned" figure only.
    sold_at TEXT PRIMARY KEY,
    base_value INTEGER,
    bonus INTEGER,
    total_earnings INTEGER,
    system_names TEXT  -- comma-joined, informational only -- cannot attribute value per system
);
"""


def _migrate(con: sqlite3.Connection) -> None:
    """Lightweight column-add migration for DBs created before a schema change -- SQLite has no
    ADD COLUMN IF NOT EXISTS, so check pragma table_info first. Currently just the
    parent_stars -> parent_star_body_id rename/type-change (dropping the old text column isn't
    worth the complexity here; it's just left unused if present)."""
    existing_columns = {row['name'] for row in con.execute('PRAGMA table_info(planets)')}
    if 'parent_star_body_id' not in existing_columns:
        con.execute('ALTER TABLE planets ADD COLUMN parent_star_body_id INTEGER')
        con.commit()


def connect(db_path: str) -> sqlite3.Connection:
    """Read-write connection to standalone's own DB -- creates the file/schema on first use."""
    dirname = os.path.dirname(db_path)
    if dirname:
        os.makedirs(dirname, exist_ok=True)
    con = sqlite3.connect(db_path)
    con.row_factory = sqlite3.Row
    con.executescript(_SCHEMA)
    _migrate(con)
    con.execute(
        "INSERT INTO meta (key, value) VALUES ('schema_version', ?) "
        "ON CONFLICT(key) DO UPDATE SET value = excluded.value",
        (str(SCHEMA_VERSION),),
    )
    con.commit()
    return con
