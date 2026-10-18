"""Compile tests inside Docker, or execute the resulting Windows bundle without Go."""
import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent
BUNDLE = ROOT / 'validation/windows-tests'
CORE = 'oshell-desktop/internal/core'
RACE = [CORE, 'oshell-desktop/internal/terminal/internal/pty', 'oshell-desktop/internal/zmodem']
PARSERS = ['github.com/jlaffaye/ftp', 'github.com/pkg/sftp']


def run(command, **kwargs):
    print(' '.join(map(str, command)), flush=True)
    subprocess.run(list(map(str, command)), check=True, **kwargs)


def objects(text):
    decoder = json.JSONDecoder()
    while text.strip():
        value, end = decoder.raw_decode(text.lstrip())
        yield value
        text = text.lstrip()[end:]


def compile_bundle(embedded=False):
    if not Path('/.dockerenv').exists():
        raise RuntimeError('CI compilation requires Docker')
    BUNDLE.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, GOOS='windows', GOARCH='amd64', CGO_ENABLED='0')
    manifest_path = BUNDLE / 'manifest.json'
    if embedded:
        manifest = json.loads(manifest_path.read_text())
        specs = [(CORE, False, '^TestSingleFileStorageActivation$', 'embedded-core'),
                 ('oshell-desktop/internal/terminal', False, '^TestSingleFileResourcesLoad$', 'embedded-terminal')]
    else:
        manifest = {'commit': os.environ['GITHUB_SHA'], 'tests': []}
        listed = subprocess.check_output(['go', 'list', '-json', './...', *PARSERS], cwd=ROOT, env=env, text=True)
        packages = [p for p in objects(listed) if p.get('TestGoFiles') or p.get('XTestGoFiles')]
        specs = [(p['ImportPath'], False, 'Test(UNIXMetadata|OwnerHints|Parse)' if p['ImportPath'] in PARSERS else '.',
                  'normal-' + str(i)) for i, p in enumerate(packages)]
        specs += [(p, True, '.', 'race-' + p.rsplit('/', 1)[-1]) for p in RACE]
        run(['go', 'build', '-o', BUNDLE / 'test2json.exe', 'cmd/test2json'], cwd=ROOT, env=env)
        run(['go', 'build', '-o', BUNDLE / 'update-fixture.exe', './internal/core/testdata/update-fixture'], cwd=ROOT, env=env)
        run(['go', 'build', './...'], cwd=ROOT, env=env)
        run(['go', 'vet', './...'], cwd=ROOT, env=env)
    for package, race, pattern, name in specs:
        test_env = dict(env)
        if race:
            test_env.update(CGO_ENABLED='1', CC='x86_64-w64-mingw32-gcc', CXX='x86_64-w64-mingw32-g++')
        binary = name + '.exe'
        run(['go', 'test', '-c', *(['-race'] if race else []), '-o', BUNDLE / binary, package], cwd=ROOT, env=test_env)
        directory = ('third_party/ftp' if package == PARSERS[0] else 'third_party/sftp' if package == PARSERS[1]
                     else package.removeprefix('oshell-desktop/'))
        if package == 'oshell-desktop':
            directory = '.'
        manifest['tests'].append(dict(package=package, binary=binary, race=race, pattern=pattern, directory=directory))
    manifest_path.write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')


def execute_windows(shard):
    manifest = json.loads((BUNDLE / 'manifest.json').read_text())
    if manifest['commit'] != os.environ['GITHUB_SHA']:
        raise RuntimeError('Test bundle does not belong to this commit')
    os.environ['OSHELL_PRECOMPILED_TESTS'] = '1'
    os.environ['OSHELL_UPDATE_FIXTURE'] = str(BUNDLE / 'update-fixture.exe')
    if shard is not None:
        run([sys.executable, ROOT / 'scripts/core_test_shards.py', 'run', '--platform', 'Windows',
             '--shard', str(shard), '--binary', BUNDLE / 'race-core.exe', '--test2json', BUNDLE / 'test2json.exe'])
        return
    for item in manifest['tests']:
        if item['race'] and item['package'] == CORE:
            continue
        run([BUNDLE / item['binary'], '-test.v', '-test.count=1', '-test.timeout=5m', '-test.run', item['pattern']],
            cwd=ROOT / item['directory'])
    for script in ['test-build.py', 'test-configuration-contract.py', 'test-core-shards.py']:
        run([sys.executable, ROOT / 'scripts' / script], cwd=ROOT)


def linux():
    if not Path('/.dockerenv').exists():
        raise RuntimeError('Linux CI tests require Docker')
    os.environ['CGO_ENABLED'] = '1'
    os.environ['OSHELL_REQUIRE_LRZSZ'] = '1'
    fixture = ROOT / 'validation/linux-update-fixture'
    fixture.parent.mkdir(exist_ok=True)
    run(['go', 'build', '-o', fixture, './internal/core/testdata/update-fixture'], cwd=ROOT)
    os.environ['OSHELL_UPDATE_FIXTURE'] = str(fixture)
    core_binary = ROOT / 'validation/linux-core.test'
    adapter = ROOT / 'validation/linux-test2json'
    run(['go', 'test', '-race', '-c', '-o', core_binary, './internal/core'], cwd=ROOT)
    run(['go', 'build', '-o', adapter, 'cmd/test2json'], cwd=ROOT)
    # Parallel processes preserve isolation even when the organization has only one Runner.
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
        futures = [executor.submit(run, [sys.executable, ROOT / 'scripts/core_test_shards.py', 'run',
                                        '--platform', 'Linux', '--shard', str(i), '--binary', core_binary,
                                        '--test2json', adapter], cwd=ROOT) for i in range(4)]
        failures = []
        for future in futures:
            try:
                future.result()
            except subprocess.CalledProcessError as error:
                failures.append(error)
        if failures:
            raise failures[0]
    run(['go', 'test', '-race', '-count=1', '-timeout=5m', *RACE[1:]], cwd=ROOT)
    run(['xvfb-run', '-a', 'dbus-run-session', '--', 'go', 'test', '-count=1', '-timeout=180s', './...'], cwd=ROOT)
    run(['go', 'test', '-count=1', *PARSERS, '-run', 'Test(UNIXMetadata|OwnerHints|Parse)'], cwd=ROOT)
    for script in ['test-build.py', 'test-core-shards.py', 'test-configuration-contract.py']:
        run([sys.executable, ROOT / 'scripts' / script], cwd=ROOT)
    run(['go', 'build', './...'], cwd=ROOT, env=dict(os.environ, CGO_ENABLED='0'))
    run(['go', 'vet', './...'], cwd=ROOT)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['compile', 'embedded', 'windows', 'linux'])
    parser.add_argument('--shard', type=int)
    args = parser.parse_args()
    if args.mode in ('compile', 'embedded'):
        compile_bundle(args.mode == 'embedded')
    elif args.mode == 'windows':
        execute_windows(args.shard)
    else:
        linux()
