#!/usr/bin/env python3
"""Single entry point for the packaged standalone program: parses your journal, builds the
viewer, and opens it -- one double-click, no terminal, no commands. This is what gets bundled
into a real executable by build_executable.py/PyInstaller; running it directly with `python3
run.py` does the exact same thing for anyone who'd rather run it from source.
"""

from __future__ import annotations

import argparse
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


def main() -> None:
    # Double-clicking passes no arguments at all -- auto-detect handles that case. Anyone
    # launching this from a terminal (running from source, or the packaged program) can still
    # pass these explicitly, e.g. if auto-detection can't find a single unambiguous folder.
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--journal-dir', default=None)
    parser.add_argument('--db', default=DB_PATH)
    parser.add_argument('--out', default=VIEWER_PATH)
    args, _unknown = parser.parse_known_args()

    print('standalone -- parsing your Elite Dangerous journal...')

    journal_dir = args.journal_dir or journal_locate.find_journal_dir()
    if not journal_dir:
        candidates = journal_locate.find_journal_dir_candidates()
        if candidates:
            print("Found more than one possible journal folder, can't pick automatically:")
            for c in candidates:
                print(f'  {c}')
            print('Run from a terminal with --journal-dir PATH instead of double-clicking.')
        else:
            print('Could not find your Elite Dangerous journal folder automatically.')
            print('Run from a terminal with --journal-dir PATH instead of double-clicking.')
        _pause_on_windows()
        sys.exit(1)
    print(f'Journal folder: {journal_dir}')

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
