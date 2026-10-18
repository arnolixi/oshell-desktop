"""Ensure artifact splitting preserves all tests and rejects incomplete/stale bundles."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('ci_bundle', Path(__file__).with_name('ci-test-bundle.py'))
bundle = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bundle)


class BundleTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.source = Path(self.temp.name) / 'source'
        self.output = Path(self.temp.name) / 'output'
        self.source.mkdir()
        self.tests = [dict(package=package, race=race, binary=name + '.exe',
                           pattern=pattern, directory='internal/' + name)
                      for package, race, name, pattern in [
                          (bundle.CORE, False, 'normal-core', '.'),
                          (bundle.CORE, True, 'race-core', '.'),
                          (bundle.RACE[2], True, 'race-zmodem', '.'),
                          (bundle.CORE, False, 'embedded-core', '^TestSingleFileStorageActivation$'),
                          ('oshell-desktop/new-package', False, 'new-package', '.')]]
        self.manifest = dict(commit='current-commit', tests=self.tests)
        self.write_manifest()
        for name in [item['binary'] for item in self.tests] + ['update-fixture.exe', 'test2json.exe']:
            (self.source / name).write_bytes(('binary:' + name).encode())

    def write_manifest(self):
        (self.source / 'manifest.json').write_text(json.dumps(self.manifest), encoding='utf-8')

    def stage(self):
        bundle.stage_windows_bundles(self.source, self.output)

    def test_preserves_complete_inventory_metadata_and_binary_contents(self):
        self.stage()
        combined = []
        for group in ['windows-regression', 'windows-core-race']:
            target = self.output / group
            manifest = json.loads((target / 'manifest.json').read_text())
            self.assertEqual(manifest['commit'], self.manifest['commit'])
            combined.extend(manifest['tests'])
            for file in target.glob('*.exe'):
                self.assertEqual(file.read_bytes(), (self.source / file.name).read_bytes())
        self.assertCountEqual(combined, self.tests)
        self.assertEqual({p.name for p in (self.output / 'windows-core-race').iterdir()},
                         {'race-core.exe', 'test2json.exe', 'update-fixture.exe', 'manifest.json'})
        regression = self.output / 'windows-regression'
        self.assertFalse((regression / 'race-core.exe').exists())
        self.assertFalse((regression / 'test2json.exe').exists())
        self.assertTrue((regression / 'new-package.exe').exists())

    def test_missing_binary_fails_before_publishing_either_bundle(self):
        (self.source / 'test2json.exe').unlink()
        with self.assertRaisesRegex(ValueError, 'Missing or invalid'):
            self.stage()
        self.assertFalse(self.output.exists())

    def test_stale_bundle_is_not_merged_or_overwritten(self):
        stale = self.output / 'windows-core-race'
        stale.mkdir(parents=True)
        (stale / 'old.exe').write_bytes(b'keep')
        with self.assertRaisesRegex(ValueError, 'stale'):
            self.stage()
        self.assertEqual((stale / 'old.exe').read_bytes(), b'keep')
        self.assertFalse((self.output / 'windows-regression').exists())

    def test_missing_or_duplicate_core_race_binary_is_rejected(self):
        for tests in [self.tests[:1], self.tests + [self.tests[1]]]:
            self.manifest['tests'] = tests
            self.write_manifest()
            with self.assertRaisesRegex(ValueError, 'exactly one'):
                self.stage()
            self.assertFalse(self.output.exists())


if __name__ == '__main__':
    unittest.main()
