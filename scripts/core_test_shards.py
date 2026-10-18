"""Time, balance and audit complete core race suites; never cache test results."""
import argparse
import collections
import json
import math
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent
PACKAGE = 'oshell-desktop/internal/core'
TIMINGS = ROOT / 'scripts/core-race-timings.json'


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')


def plan_tests(names, weights, count):
    if count < 1 or len(names) < count or len(names) != len(set(names)):
        raise ValueError('Invalid shard count or duplicate/insufficient discovered tests')
    def weight(name):
        value = float(weights.get(name, 1.0))
        if not math.isfinite(value) or value < 0:
            raise ValueError('Invalid timing: ' + name)
        return max(value, 0.01)
    shards, loads = [[] for _ in range(count)], [0.0] * count
    for name in sorted(names, key=lambda name: (-weight(name), name)):
        index = min(range(count), key=lambda i: (loads[i], i))
        shards[index].append(name)
        loads[index] += weight(name)
    return [sorted(shard) for shard in shards], loads


def read_events(path):
    # utf-8-sig also accepts logs redirected by Windows shells.
    with path.open(encoding='utf-8-sig') as source:
        return [json.loads(line) for line in source if line.strip()]


def outcomes(events):
    result = {}
    package_passed = False
    for event in events:
        if event.get('Package') != PACKAGE:
            continue
        name, action = event.get('Test', ''), event.get('Action')
        if action == 'fail':
            raise ValueError('Go reported failure: ' + (name or PACKAGE))
        if not name and action == 'pass':
            package_passed = True
        if name and '/' not in name and action in ('pass', 'skip'):
            if name in result:
                raise ValueError('Test completed more than once: ' + name)
            result[name] = {'status': action, 'seconds': event.get('Elapsed', 0)}
    if not package_passed:
        raise ValueError('Missing successful package completion (crash, timeout or truncated log)')
    return result


def check_selected(selected, result):
    if len(selected) != len(set(selected)) or set(selected) != set(result):
        raise ValueError('Executed tests differ from selection; missing=%s extra=%s' % (
            sorted(set(selected) - set(result)), sorted(set(result) - set(selected))))
    if any(item['status'] not in ('pass', 'skip') for item in result.values()):
        raise ValueError('Unsuccessful test in shard')


def run_shard(args):
    if not 0 <= args.shard < args.count:
        raise ValueError('Shard index must be within [0, count)')
    # Discover with the same race/platform build constraints as the actual run.
    binary = getattr(args, 'binary', None)
    listing_command = [str(binary.resolve()), '-test.list', '.'] if binary else [
        'go', 'test', '-race', '-list', '.', './internal/core']
    listing = subprocess.check_output(listing_command, cwd=ROOT / 'internal/core' if binary else ROOT, text=True)
    names = sorted(line for line in listing.splitlines()
                   if re.fullmatch(r'(Test|Example|Fuzz)\w*', line))
    weights = json.loads(TIMINGS.read_text(encoding='utf-8'))['platforms'][args.platform]
    plan, loads = plan_tests(names, weights, args.count)
    selected = plan[args.shard]
    prefix = args.output / f'{args.platform}-{args.shard}'
    prefix.parent.mkdir(parents=True, exist_ok=True)
    report = {'platform': args.platform, 'shard': args.shard, 'count': args.count,
              'commit': os.environ.get('GITHUB_SHA', 'local'), 'inventory': names,
              'plan': plan, 'selected': selected, 'estimatedSeconds': loads[args.shard],
              'success': False, 'results': {}}
    report_path = prefix.with_suffix('.manifest.json')
    write_json(report_path, report)
    print(f'{args.platform} shard {args.shard + 1}/{args.count}: {len(selected)}/{len(names)} tests, '
          f'scheduling weight {loads[args.shard]:.1f}s', flush=True)
    pattern = '^(' + '|'.join(re.escape(name) for name in selected) + ')$'
    command = ['go', 'test', '-race', '-json', '-count=1', '-timeout=30m',
               '-run', pattern, './internal/core']
    cwd = ROOT
    if binary:
        command = [str(args.test2json.resolve()), '-p', PACKAGE, '-t', str(binary.resolve()),
                   '-test.v=test2json', '-test.count=1', '-test.timeout=30m', '-test.run', pattern]
        cwd = ROOT / 'internal/core'
    events = []
    with prefix.with_suffix('.jsonl').open('w', encoding='utf-8') as log:
        with subprocess.Popen(command, cwd=cwd, stdout=subprocess.PIPE,
                              text=True, encoding='utf-8') as process:
            for line in process.stdout:
                log.write(line)
                log.flush()
                event = json.loads(line)
                events.append(event)
                if event.get('Action') == 'fail':
                    print(line.rstrip(), flush=True)
            code = process.wait()
    try:
        if code:
            raise ValueError(f'go test exited with status {code}; see {prefix}.jsonl')
        report['results'] = outcomes(events)
        check_selected(selected, report['results'])
        report['success'] = True
    finally:
        write_json(report_path, report)
    print(f'Verified {len(selected)} completed top-level tests, including all their subtests.')
    for name, item in sorted(report['results'].items(), key=lambda pair: -pair[1]['seconds'])[:10]:
        print(f"{item['seconds']:8.2f}s {item['status']:4s} {name}")


def audit_reports(reports, platforms, count, commit):
    if len(reports) != len(platforms) * count:
        raise ValueError('Missing or extra shard reports')
    summaries = []
    for platform in platforms:
        group = sorted((report for report in reports if report['platform'] == platform),
                       key=lambda report: report['shard'])
        if [report['shard'] for report in group] != list(range(count)):
            raise ValueError('Missing/duplicate shards for ' + platform)
        inventory, plan = group[0]['inventory'], group[0]['plan']
        if len(plan) != count or len(inventory) != len(set(inventory)):
            raise ValueError('Invalid inventory/plan')
        all_selected = [name for shard in plan for name in shard]
        if collections.Counter(all_selected) != collections.Counter(inventory):
            raise ValueError('Plan does not cover every discovered test exactly once')
        skipped = []
        for report in group:
            if (not report['success'] or report['count'] != count or report['commit'] != commit
                    or report['inventory'] != inventory or report['plan'] != plan
                    or report['selected'] != plan[report['shard']]):
                raise ValueError('Inconsistent, stale or failed shard report')
            check_selected(report['selected'], report['results'])
            skipped += [name for name, item in report['results'].items() if item['status'] == 'skip']
        summaries.append(f'{platform}: all {len(inventory)} top-level tests accounted for exactly once; '
                         f'{len(skipped)} test-controlled skips: {", ".join(sorted(skipped)) or "none"}')
    return summaries


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    run = commands.add_parser('run')
    run.add_argument('--platform', choices=['Windows', 'Linux'], required=True)
    run.add_argument('--shard', type=int, required=True)
    run.add_argument('--count', type=int, default=4)
    run.add_argument('--output', type=Path, default=ROOT / 'validation/core-race')
    run.add_argument('--binary', type=Path, help='Container-built race test executable; no compiler invoked')
    run.add_argument('--test2json', type=Path, help='Container-built Go JSON test adapter')
    audit = commands.add_parser('audit')
    audit.add_argument('directory', type=Path)
    audit.add_argument('--count', type=int, default=4)
    audit.add_argument('--commit', default=os.environ.get('GITHUB_SHA', 'local'))
    record = commands.add_parser('record')
    record.add_argument('--platform', choices=['Windows', 'Linux'], required=True)
    record.add_argument('--input', type=Path, required=True)
    record.add_argument('--source-run', required=True)
    args = parser.parse_args()
    if args.command == 'run':
        run_shard(args)
    elif args.command == 'audit':
        reports = []
        for path in args.directory.rglob('*.manifest.json'):
            report = json.loads(path.read_text(encoding='utf-8'))
            raw = path.with_name(path.name.removesuffix('.manifest.json') + '.jsonl')
            if outcomes(read_events(raw)) != report['results']:
                raise ValueError('Manifest differs from original Go execution log: ' + str(path))
            reports.append(report)
        for summary in audit_reports(reports, ['Windows', 'Linux'], args.count, args.commit):
            print(summary)
            if os.environ.get('GITHUB_STEP_SUMMARY'):
                with open(os.environ['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as output:
                    output.write(summary + '\n\n')
    else:
        results = outcomes(read_events(args.input))
        data = json.loads(TIMINGS.read_text(encoding='utf-8')) if TIMINGS.exists() else {
            'version': 1, 'sources': {}, 'platforms': {}}
        data['sources'][args.platform] = args.source_run
        data['platforms'][args.platform] = dict(sorted((name, item['seconds']) for name, item in results.items()))
        write_json(TIMINGS, data)
        for name, item in sorted(results.items(), key=lambda pair: -pair[1]['seconds'])[:15]:
            print(f"{item['seconds']:8.2f}s {name}")


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        sys.exit(str(error))
