#!/usr/bin/env python3
"""Single entry point for the packaged standalone program: parses your journal, builds the
viewer, and opens it -- one double-click, no terminal, no commands. This is what gets bundled
into a real executable by build_executable.py/PyInstaller; running it directly with `python3
run.py` does the exact same thing for anyone who'd rather run it from source.
"""

from __future__ import annotations

import argparse
import glob
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from standalone import db, journal_locate, viewer  # noqa: E402
from standalone.parse_journals import parse_journal_directory  # noqa: E402

DB_PATH = 'standalone.db'
VIEWER_PATH = 'standalone_viewer.html'


def _pause_on_windows() -> None:
    """Keeps a double-clicked console window open on Windows long enough to read the output
    (Windows closes the console the instant a launched .exe exits) -- a no-op everywhere else,
    where the program was presumably launched from a terminal that stays open on its own."""
    if sys.platform == 'win32':
        input('\nPress Enter to close this window...')


def _print_autodetect_failure() -> None:
    candidates = journal_locate.find_journal_dir_candidates()
    if candidates:
        print("Found more than one possible journal folder, can't pick automatically:")
        for c in candidates:
            print(f'  {c}')
    else:
        print('Could not find your Elite Dangerous journal folder automatically.')


def _resolve_journal_dir(explicit: str | None) -> str | None:
    """Asks before it acts, rather than silently guessing and only presenting a choice once
    auto-detection has already failed. Falls back to the old one-shot auto-detect-or-fail
    behavior when stdin isn't an interactive terminal (piped/scripted run), since there's nobody
    there to answer a prompt."""
    if explicit:
        return explicit

    if not sys.stdin.isatty():
        journal_dir = journal_locate.find_journal_dir()
        if not journal_dir:
            _print_autodetect_failure()
            print('Pass it explicitly with --journal-dir PATH')
            return None
        return journal_dir

    while True:
        print("Where's your Elite Dangerous journal folder?")
        print('  [Enter]  auto-detect it for me')
        print('  or type the full folder path and press Enter')
        line = input('> ').strip()

        if not line:
            journal_dir = journal_locate.find_journal_dir()
            if journal_dir:
                return journal_dir
            _print_autodetect_failure()
            print('Try typing the path instead.')
            print()
            continue

        if not os.path.isdir(line):
            print("That doesn't look like a folder -- try again.")
            print()
            continue
        if not glob.glob(os.path.join(line, journal_locate.JOURNAL_GLOB)):
            print('No Journal.*.log files found in that folder -- try again.')
            print()
            continue
        return line


def main() -> None:
    # Double-clicking passes no arguments at all -- auto-detect/prompt handles that case. Anyone
    # launching this from a terminal (running from source, or the packaged program) can still
    # pass --journal-dir explicitly to skip the prompt entirely.
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--journal-dir', default=None)
    parser.add_argument('--db', default=DB_PATH)
    parser.add_argument('--out', default=VIEWER_PATH)
    args, _unknown = parser.parse_known_args()

    print('standalone -- Elite Dangerous journal viewer')
    print()

    journal_dir = _resolve_journal_dir(args.journal_dir)
    if not journal_dir:
        _pause_on_windows()
        sys.exit(1)
    print(f'Using journal folder: {journal_dir}')
    print()
    print('Parsing your Elite Dangerous journal...')

    con = db.connect(args.db)
    try:
        files_touched, new_lines = parse_journal_directory(con, journal_dir)
        if files_touched:
            print(f'Parsed {new_lines} new line(s) across {files_touched} file(s).')
        else:
            print('Up to date -- no new journal data since last time.')

        html = viewer.render(con)
        system_count = con.execute('SELECT COUNT(*) FROM systems').fetchone()[0]
    finally:
        con.close()

    with open(args.out, 'w', encoding='utf-8') as f:
        f.write(html)
    print(f'Wrote {args.out} ({system_count} systems) -- opening it now...')

    import webbrowser
    webbrowser.open('file://' + os.path.realpath(args.out))

    _pause_on_windows()


if __name__ == '__main__':
    main()
