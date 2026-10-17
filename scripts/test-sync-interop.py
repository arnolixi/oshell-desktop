#!/usr/bin/env python3
"""Test the real Swift and Go configuration protocols using synthetic data only."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent.parent

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--native-source', type=Path, default=ROOT.parent / 'OShell')
    args = parser.parse_args()
    if platform.system() != 'Darwin':
        parser.error('The native Swift core requires macOS; Go fixture tests run on all platforms.')
    sources = sorted((args.native_source / 'Sources/OShellCore').glob('*.swift'))
    if not sources:
        parser.error('Provide the macOS OShell source checkout using --native-source.')
    with tempfile.TemporaryDirectory(prefix='oshell-sync-interop-') as folder:
        work = Path(folder)
        common = ['swiftc', '-swift-version', '5', '-module-cache-path', str(work / 'swift-cache')]
        subprocess.run(common + ['-emit-library', '-emit-module', '-module-name', 'OShellCore',
            *map(str, sources), '-o', str(work / 'libOShellCore.dylib'),
            '-emit-module-path', str(work / 'OShellCore.swiftmodule')], check=True)
        helper = work / 'interop'
        subprocess.run(common + ['-I', str(work), '-L', str(work), '-lOShellCore',
            '-Xlinker', '-rpath', '-Xlinker', str(work), str(ROOT / 'scripts/interop/main.swift'),
            '-o', str(helper)], check=True)
        env = dict(os.environ, OSHELL_SWIFT_INTEROP_HELPER=str(helper))
        env.setdefault('GOCACHE', str(work / 'go-cache'))
        subprocess.run(['go', 'test', './internal/core', '-run', 'TestSwift', '-count=1', '-v'],
                       cwd=ROOT, env=env, check=True)
        digest = hashlib.sha256()
        for source in sources:
            digest.update(source.name.encode()); digest.update(source.read_bytes())
        print(json.dumps({'swiftCoreSHA256': digest.hexdigest(), 'result': 'passed'}, indent=2))

if __name__ == '__main__':
    main()
