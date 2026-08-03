"""Best-effort auto-detection of the Elite Dangerous journal folder, so a random commander on
their own machine can run this with zero configuration in the common case. No single standard
path exists on Linux (unlike Windows/Mac's fixed locations) -- this is the same problem EDMC
itself has to solve, with the same answer: best-effort guess, explicit override otherwise.
"""

from __future__ import annotations

import glob
import os
import sys

JOURNAL_GLOB = 'Journal.*.log'


def _candidate_dirs() -> list[str]:
    home = os.path.expanduser('~')

    if sys.platform == 'win32':
        # Fixed location on Windows -- confirmed by ED's own documented journal path.
        userprofile = os.environ.get('USERPROFILE', home)
        return [os.path.join(userprofile, 'Saved Games', 'Frontier Developments', 'Elite Dangerous')]

    if sys.platform == 'darwin':
        # Not verified against a real Mac install (none available in this project's
        # development environment) -- this is ED's documented Mac save-data convention, flagged
        # here so a real Mac user hitting a wrong guess knows to just pass --journal-dir.
        return [os.path.join(home, 'Library', 'Application Support', 'Frontier Developments', 'Elite Dangerous')]

    # Linux has no single standard location -- glob across the common Proton/Wine/Lutris
    # layouts. The Steam/Proton pattern is confirmed against a real install (this project's own
    # dev machine uses exactly this layout); the others are reasonable, unverified guesses at
    # equally common setups.
    patterns = [
        os.path.join(home, '.local', 'share', 'Steam', 'steamapps', 'compatdata', '*', 'pfx',
                     'drive_c', 'users', '*', 'Saved Games', 'Frontier Developments', 'Elite Dangerous'),
        os.path.join(home, '.steam', 'steam', 'steamapps', 'compatdata', '*', 'pfx',
                     'drive_c', 'users', '*', 'Saved Games', 'Frontier Developments', 'Elite Dangerous'),
        os.path.join(home, '.wine', 'drive_c', 'users', '*', 'Saved Games', 'Frontier Developments', 'Elite Dangerous'),
        os.path.join(home, 'Games', '*', 'drive_c', 'users', '*', 'Saved Games', 'Frontier Developments', 'Elite Dangerous'),
    ]
    found: list[str] = []
    for pattern in patterns:
        found.extend(glob.glob(pattern))
    return found


def find_journal_dir() -> str | None:
    """Returns the journal directory if exactly one candidate with actual journal files in it
    is found; None otherwise (ambiguous or nothing found -- caller should fall back to an
    explicit --journal-dir)."""
    candidates = find_journal_dir_candidates()
    return candidates[0] if len(candidates) == 1 else None


def find_journal_dir_candidates() -> list[str]:
    """All distinct directories that look like they contain real journal files, for a helpful
    error message when auto-detection can't pick a single one. Same realpath dedup as
    find_journal_dir(), so this never shows the same real directory twice under different
    symlinked paths (e.g. ~/.steam/steam is commonly a symlink to ~/.local/share/Steam)."""
    candidates = [
        d for d in _candidate_dirs()
        if os.path.isdir(d) and glob.glob(os.path.join(d, JOURNAL_GLOB))
    ]
    return sorted(set(os.path.realpath(c) for c in candidates))
