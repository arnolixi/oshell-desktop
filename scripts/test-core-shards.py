"""Prove shard scheduling cannot silently omit newly added tests or failed runs."""
import copy
import io
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import core_test_shards as shards


class ShardTests(unittest.TestCase):
    def reports(self):
        names = ['TestSlow', 'TestFast', 'TestNew', 'Example', 'FuzzSeeds']
        plan, _ = shards.plan_tests(names, {'TestSlow': 10, 'TestFast': 5}, 2)
        return [{'platform': platform, 'shard': i, 'count': 2, 'commit': 'abc',
                 'inventory': names, 'plan': plan, 'selected': selected, 'success': True,
                 'results': {name: {'status': 'pass', 'seconds': 1} for name in selected}}
                for platform in ['Windows', 'Linux'] for i, selected in enumerate(plan)]

    def audit(self, reports):
        return shards.audit_reports(reports, ['Windows', 'Linux'], 2, 'abc')

    def test_balances_by_time_and_discovers_new_tests(self):
        names = ['TestA', 'TestB', 'TestC', 'TestD', 'TestNew']
        weights = {'TestA': 10, 'TestB': 9, 'TestC': 2, 'TestD': 1}
        plan, loads = shards.plan_tests(names, weights, 2)
        self.assertEqual(sorted(sum(plan, [])), sorted(names))
        self.assertLessEqual(max(loads) - min(loads), 1)
        self.assertEqual((plan, loads), shards.plan_tests(list(reversed(names)), weights, 2))

    def test_audit_all_platforms_and_test_controlled_skips(self):
        reports = self.reports()
        reports[0]['results'][reports[0]['selected'][0]]['status'] = 'skip'
        self.assertEqual(len(self.audit(reports)), 2)

    def test_missing_duplicate_failed_and_stale_shards_fail(self):
        reports = self.reports()
        for broken in [reports[:-1], reports + [reports[0]], reports[:2] + reports[:2]]:
            with self.assertRaises(ValueError):
                self.audit(broken)
        for field, value in [('success', False), ('commit', 'old'), ('count', 3),
                             ('inventory', ['TestWrong']), ('selected', [])]:
            broken = copy.deepcopy(reports)
            broken[0][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.audit(broken)

    def test_omitted_and_extra_executions_fail(self):
        for mutation in ['missing', 'extra', 'failure']:
            reports = self.reports()
            results = reports[0]['results']
            if mutation == 'missing':
                results.pop(next(iter(results)))
            elif mutation == 'extra':
                results['TestUnexpected'] = {'status': 'pass'}
            else:
                results[next(iter(results))]['status'] = 'fail'
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                self.audit(reports)

    def test_subtests_not_double_counted_and_package_failure_cannot_pass(self):
        events = [{'Package': shards.PACKAGE, 'Test': name, 'Action': 'pass', 'Elapsed': duration}
                  for name, duration in [('TestA/child', 3), ('TestA', 4)]]
        terminal = {'Package': shards.PACKAGE, 'Action': 'pass'}
        self.assertEqual(shards.outcomes(events + [terminal]), {'TestA': {'status': 'pass', 'seconds': 4}})
        for broken in [events, events + [dict(terminal, Action='fail')],
                       events + events + [terminal]]:
            with self.assertRaises(ValueError):
                shards.outcomes(broken)

    def test_runner_records_actual_process_results_and_preserves_exit_failure(self):
        events = [{'Package': shards.PACKAGE, 'Test': 'TestAdded', 'Action': 'pass', 'Elapsed': 2},
                  {'Package': shards.PACKAGE, 'Action': 'pass'}]
        for exit_code in [0, 1]:
            class Process:
                stdout = io.StringIO(''.join(json.dumps(event) + '\n' for event in events))
                def __enter__(self): return self
                def __exit__(self, *args): pass
                def wait(self): return exit_code
            with self.subTest(exit_code=exit_code), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                weights = root / 'weights.json'
                # Missing timing must not omit the newly added test.
                weights.write_text('{"platforms":{"Windows":{}}}', encoding='utf-8')
                args = SimpleNamespace(platform='Windows', shard=0, count=1, output=root)
                with patch.object(shards, 'TIMINGS', weights), \
                     patch.object(shards.subprocess, 'check_output', return_value='TestAdded\nok core\n'), \
                     patch.object(shards.subprocess, 'Popen', return_value=Process()) as run:
                    if exit_code:
                        with self.assertRaises(ValueError):
                            shards.run_shard(args)
                    else:
                        shards.run_shard(args)
                    command = run.call_args.args[0]
                    self.assertIn('-race', command)
                    self.assertIn('-count=1', command)
                    self.assertEqual(command[command.index('-run') + 1], '^(TestAdded)$')
                report = json.loads((root / 'Windows-0.manifest.json').read_text(encoding='utf-8'))
                self.assertEqual(report['success'], exit_code == 0)
                self.assertEqual(report['selected'], ['TestAdded'])

    def test_precompiled_runner_never_invokes_go(self):
        events = [{'Package': shards.PACKAGE, 'Test': 'TestAdded', 'Action': 'pass'},
                  {'Package': shards.PACKAGE, 'Action': 'pass'}]
        class Process:
            stdout = io.StringIO(''.join(json.dumps(event) + '\n' for event in events))
            def __enter__(self): return self
            def __exit__(self, *args): pass
            def wait(self): return 0
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            weights = root / 'weights.json'
            weights.write_text('{"platforms":{"Windows":{}}}', encoding='utf-8')
            args = SimpleNamespace(platform='Windows', shard=0, count=1, output=root,
                                   binary=root / 'race.exe', test2json=root / 'adapter.exe')
            with patch.object(shards, 'TIMINGS', weights), \
                 patch.object(shards.subprocess, 'check_output', return_value='TestAdded\n') as listing, \
                 patch.object(shards.subprocess, 'Popen', return_value=Process()) as run:
                shards.run_shard(args)
                # Hosted Windows TEMP may contain an 8.3 username alias.
                self.assertEqual(listing.call_args.args[0], [str(args.binary.resolve()), '-test.list', '.'])
                command = run.call_args.args[0]
                self.assertEqual(command[0], str(args.test2json.resolve()))
                self.assertIn(str(args.binary.resolve()), command)
                self.assertIn('-test.count=1', command)
                self.assertIn('-test.paniconexit0', command)
                self.assertEqual(command[command.index('-test.run') + 1], '^(TestAdded)$')


if __name__ == '__main__':
    unittest.main()
