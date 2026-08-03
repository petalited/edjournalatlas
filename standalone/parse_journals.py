#!/usr/bin/env python3
"""Single-command entry point: parses Elite Dangerous journal files directly into standalone's
own local DB. No dependency on ExploData/BioScan/Pioneer, no network access. Safe to re-run any
time -- incremental by default (see docs/StandaloneJournalParser.md): already-fully-processed,
unchanged journal files are skipped entirely rather than reparsed from scratch.

Usage:
    python3 parse_journals.py [--journal-dir PATH] [--db PATH]

Both are optional. --journal-dir auto-detects the OS-conventional journal folder when omitted
(see journal_locate.py); if auto-detection can't pick a single unambiguous folder, this prints
what it found and asks for --journal-dir explicitly rather than guessing. --db defaults to
standalone.db in the current directory.
"""

from __future__ import annotations

import argparse
import glob
import json
import os
import sys

# Makes this runnable directly (`python3 standalone/parse_journals.py`, from anywhere) as well
# as via `python3 -m standalone.parse_journals` -- someone should be able to just copy the
# standalone/ folder on its own and run it, per the whole point of this being independently
# distributable.
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from standalone import db, journal_locate, journal_parse  # noqa: E402

DEFAULT_DB_PATH = 'standalone.db'


def _process_file_incremental(con, parser: journal_parse.JournalParser, path: str) -> int:
    """Returns the number of new lines actually parsed (0 if the file was unchanged since the
    last run and skipped entirely)."""
    file_name = os.path.basename(path)
    mtime = os.path.getmtime(path)
    row = con.execute(
        'SELECT file_mtime, line_count FROM parse_progress WHERE file_name = ?', (file_name,),
    ).fetchone()
    if row and row['file_mtime'] == mtime:
        return 0  # unchanged since last run -- nothing new here, skip without even opening it

    already_processed = row['line_count'] if row else 0
    new_lines = 0
    line_count = 0
    with open(path, encoding='utf-8') as f:
        for line in f:
            line_count += 1
            if line_count <= already_processed:
                continue  # already parsed this line on a previous run
            new_lines += 1
            line = line.strip()
            if not line:
                continue
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue
            parser.process_entry(entry)

    con.execute(
        """
        INSERT INTO parse_progress (file_name, file_mtime, line_count) VALUES (?, ?, ?)
        ON CONFLICT(file_name) DO UPDATE SET file_mtime = excluded.file_mtime, line_count = excluded.line_count
        """,
        (file_name, mtime, line_count),
    )
    con.commit()
    return new_lines


def parse_journal_directory(con, journal_dir: str) -> tuple[int, int]:
    """Returns (files_touched, new_lines_total)."""
    paths = sorted(glob.glob(os.path.join(journal_dir, journal_locate.JOURNAL_GLOB)))
    parser = journal_parse.JournalParser(con)
    files_touched = 0
    new_lines_total = 0
    for path in paths:
        new_lines = _process_file_incremental(con, parser, path)
        if new_lines:
            files_touched += 1
            new_lines_total += new_lines
    return files_touched, new_lines_total


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('--journal-dir', default=None, help='Directory containing Journal.*.log files')
    parser.add_argument('--db', default=DEFAULT_DB_PATH, help='Path to standalone.db (created if missing)')
    args = parser.parse_args()

    journal_dir = args.journal_dir
    if not journal_dir:
        journal_dir = journal_locate.find_journal_dir()
        if not journal_dir:
            candidates = journal_locate.find_journal_dir_candidates()
            if candidates:
                print("Found more than one possible journal folder, can't pick automatically:")
                for c in candidates:
                    print(f'  {c}')
            else:
                print('Could not find your Elite Dangerous journal folder automatically.')
            print('Pass it explicitly with --journal-dir PATH')
            sys.exit(1)
        print(f'Auto-detected journal folder: {journal_dir}')

    if not os.path.isdir(journal_dir):
        sys.exit(f'Not a directory: {journal_dir!r}')

    con = db.connect(args.db)
    try:
        files_touched, new_lines = parse_journal_directory(con, journal_dir)
    finally:
        con.close()

    if files_touched == 0:
        print(f'Up to date -- no new journal data since the last run ({args.db})')
    else:
        print(f'Parsed {new_lines} new line(s) across {files_touched} file(s) into {args.db}')


if __name__ == '__main__':
    main()
