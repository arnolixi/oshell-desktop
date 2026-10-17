"""Check contract tooling with an explicit non-UTF-8 Windows-style locale."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class ContractToolTests(unittest.TestCase):
    def test_utf8_generate_check_and_drift_detection(self):
        script = Path(__file__).with_name('sync-configuration-contract.py').resolve()
        # Force accidental Path.read_text()/write_text() calls to use GBK,
        # regardless of the CI host's default encoding or Python UTF-8 mode.
        wrapper = "import locale,runpy,sys; locale.getencoding=lambda: 'gbk'; p=sys.argv.pop(1); runpy.run_path(p,run_name='__main__')"
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            target = root / 'Sources/OShellCore/SharedConfigurationContract.swift'
            target.parent.mkdir(parents=True)
            command = [sys.executable, '-X', 'utf8=0', '-c', wrapper, str(script), '--native-source', str(root)]
            env = dict(os.environ, PYTHONUTF8='0')
            for args in ([], ['--check']):
                result = subprocess.run(command + args, env=env, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr.decode('utf-8', errors='replace'))
            data = target.read_bytes()
            data.decode('utf-8')
            self.assertNotIn(b'\r\n', data)
            target.write_bytes(data + b'// drift\n')
            result = subprocess.run(command + ['--check'], env=env, capture_output=True)
            self.assertNotEqual(result.returncode, 0)


if __name__ == '__main__':
    unittest.main()
