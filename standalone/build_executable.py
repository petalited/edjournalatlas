#!/usr/bin/env python3
"""Builds a real, self-contained double-clickable program from run.py -- no Python install
needed on the machine that eventually runs it, via PyInstaller.

PyInstaller itself is only a BUILD-time tool -- it belongs in a throwaway virtualenv on the
machine doing the building (this project's other stdlib-only rule is about what the *shipped*
program needs, not what builds it). It is NOT vendored/installed into the project itself.

PyInstaller cannot cross-compile: running this script produces an executable for whatever OS
you run it ON (Linux produces a Linux binary, Windows produces a .exe, Mac produces a Mac
binary) -- there is no way to build a Windows .exe from Linux or vice versa. To ship all three,
run this script once on each OS.

Usage:
    python3 -m venv /tmp/build-venv
    /tmp/build-venv/bin/pip install pyinstaller
    /tmp/build-venv/bin/python build_executable.py
"""

from __future__ import annotations

import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.dirname(HERE)


def main() -> None:
    try:
        import PyInstaller  # noqa: F401
    except ImportError:
        sys.exit(
            'PyInstaller not installed in this Python environment.\n'
            'Run this from a dedicated build venv, e.g.:\n'
            '  python3 -m venv /tmp/build-venv && /tmp/build-venv/bin/pip install pyinstaller\n'
            '  /tmp/build-venv/bin/python build_executable.py'
        )

    name = 'edjournalatlas' + ('.exe' if sys.platform == 'win32' else '')
    vendor_src = os.path.join(HERE, 'vendor')
    # PyInstaller's --add-data separator is ';' on Windows, ':' everywhere else.
    sep = ';' if sys.platform == 'win32' else ':'

    cmd = [
        sys.executable, '-m', 'PyInstaller',
        '--onefile',
        '--name', 'edjournalatlas',
        '--distpath', os.path.join(PROJECT_ROOT, 'dist'),
        '--workpath', os.path.join(PROJECT_ROOT, 'build', 'pyinstaller-work'),
        '--specpath', os.path.join(PROJECT_ROOT, 'build', 'pyinstaller-work'),
        '--add-data', f'{vendor_src}{sep}standalone/vendor',
        os.path.join(HERE, 'run.py'),
    ]
    print('Running:', ' '.join(cmd))
    subprocess.run(cmd, check=True)
    print(f"\nBuilt: {os.path.join(PROJECT_ROOT, 'dist', name)}")
    print('That single file is the whole distributable program -- copy it anywhere and run it.')


if __name__ == '__main__':
    main()
