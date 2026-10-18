"""Exercise retry, token isolation, integrity and commit checks without network access."""
import hashlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from urllib.error import HTTPError
import zipfile

import download_runner_artifact as artifact


class FakeOpener:
    def __init__(self, responses):
        self.responses = iter(responses)
        self.api_calls = 0

    def open(self, request, timeout):
        assert timeout == 20
        if request.full_url.startswith('https://api.github.com/'):
            assert request.get_header('Authorization') == 'Bearer test-token'
            self.api_calls += 1
            raise HTTPError(request.full_url, 302, 'Found',
                            {'Location': 'https://fixture.blob.core.windows.net/artifact?signed=secret'}, None)
        assert request.get_header('Authorization') is None
        value = next(self.responses)
        if isinstance(value, Exception):
            raise value
        return value


class ArtifactTests(unittest.TestCase):
    def download(self, root, opener, data=b'complete artifact', **kwargs):
        target = Path(root) / 'download.zip'
        artifact.download('oshell-desktop/oshell-desktop', 123, 'test-token', hashlib.sha256(data).hexdigest(),
                          target, opener=opener, sleep=lambda _: None, **kwargs)
        return target

    def test_timeout_retries_and_never_sends_token_to_storage(self):
        opener = FakeOpener([TimeoutError(), io.BytesIO(b'complete artifact')])
        with tempfile.TemporaryDirectory() as root:
            self.assertEqual(self.download(root, opener).read_bytes(), b'complete artifact')
        self.assertEqual(opener.api_calls, 2)

    def test_partial_transfer_is_replaced_instead_of_appended(self):
        class Interrupted(io.BytesIO):
            def read1(self, size):
                if self.tell():
                    raise TimeoutError()
                return super().read1(size)
        opener = FakeOpener([Interrupted(b'partial'), io.BytesIO(b'complete artifact')])
        with tempfile.TemporaryDirectory() as root:
            self.assertEqual(self.download(root, opener).read_bytes(), b'complete artifact')

    def test_checksum_failure_never_leaves_a_download(self):
        opener = FakeOpener([io.BytesIO(b'corrupt')] * 1)
        with tempfile.TemporaryDirectory() as root:
            with self.assertRaisesRegex(RuntimeError, 'no unverified files'):
                self.download(root, opener, attempts=1)
            self.assertFalse((Path(root) / 'download.zip').exists())

    def test_slow_stream_has_an_overall_attempt_deadline(self):
        class Dribble(io.BytesIO):
            def read1(self, size):
                return b'x'
        ticks = iter(range(0, 1000, 30))
        with tempfile.TemporaryDirectory() as root:
            with self.assertRaises(RuntimeError):
                self.download(root, FakeOpener([Dribble()]), attempts=1, clock=lambda: next(ticks))
            self.assertFalse((Path(root) / 'download.zip').exists())

    def make_zip(self, root, *, commit='current', extra=None):
        path = Path(root) / 'artifact.zip'
        with zipfile.ZipFile(path, 'w') as output:
            output.writestr('windows-tests/manifest.json', json.dumps({'commit': commit}))
            output.writestr('windows-tests/example.exe', b'synthetic')
            output.writestr('container-package/OShell-Desktop-windows-amd64-single-file.zip', b'package')
            if extra:
                if isinstance(extra, str):
                    entry = zipfile.ZipInfo('fixture')
                    entry.filename = extra  # Preserve hostile Windows separators in the test archive.
                else:
                    entry = extra
                output.writestr(entry, b'unexpected')
        return path

    def test_commit_binding_and_refusal_to_merge_stale_files(self):
        with tempfile.TemporaryDirectory() as root:
            archive = self.make_zip(root)
            destination = Path(root) / 'validation'
            with self.assertRaisesRegex(ValueError, 'commit'):
                artifact.extract_verified(archive, destination, 'other')
            self.assertFalse(destination.exists())
            artifact.extract_verified(archive, destination, 'current')
            self.assertEqual((destination / 'windows-tests/example.exe').read_bytes(), b'synthetic')
            with self.assertRaisesRegex(ValueError, 'stale'):
                artifact.extract_verified(archive, destination, 'current')

    def test_path_traversal_symlinks_and_case_collisions_are_rejected(self):
        link = zipfile.ZipInfo('windows-tests/link')
        link.external_attr = 0o120777 << 16
        for entry in ['../outside', 'windows-tests/../../outside', '/absolute', 'windows-tests/C:stream',
                      'windows-tests\\escape', 'windows-tests/EXAMPLE.exe', link]:
            with self.subTest(entry=entry), tempfile.TemporaryDirectory() as root:
                archive = self.make_zip(root, extra=entry)
                destination = Path(root) / 'validation'
                with self.assertRaises(ValueError):
                    artifact.extract_verified(archive, destination, 'current')
                self.assertFalse(destination.exists())


if __name__ == '__main__':
    unittest.main()
