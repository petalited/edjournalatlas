#!/usr/bin/env python3
"""Generates standalone_viewer.html from standalone.db (see parse_journals.py to build that
DB first). Same self-contained, open-in-a-browser, no-server approach as edexotracker.py.

Usage:
    python3 build_viewer.py [--db PATH] [--out PATH] [--no-open]
"""

from __future__ import annotations

import argparse
import os
import sys
import webbrowser

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from standalone import db, viewer  # noqa: E402

DEFAULT_DB_PATH = 'standalone.db'
DEFAULT_OUT_PATH = 'standalone_viewer.html'


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('--db', default=DEFAULT_DB_PATH)
    parser.add_argument('--out', default=DEFAULT_OUT_PATH)
    parser.add_argument('--no-open', action='store_true')
    args = parser.parse_args()

    if not os.path.exists(args.db):
        sys.exit(f'{args.db!r} not found -- run parse_journals.py first.')

    con = db.connect(args.db)
    try:
        html = viewer.render(con)
        system_count = con.execute('SELECT COUNT(*) FROM systems').fetchone()[0]
    finally:
        con.close()

    with open(args.out, 'w', encoding='utf-8') as f:
        f.write(html)
    print(f'Wrote {args.out} ({system_count} systems)')

    if not args.no_open:
        webbrowser.open('file://' + os.path.realpath(args.out))


if __name__ == '__main__':
    main()
