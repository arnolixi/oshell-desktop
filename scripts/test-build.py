"""Check portable publication without touching real application data."""
import pathlib
import tempfile
import unittest
from unittest.mock import patch

import build


class PortablePublishTests(unittest.TestCase):
    def fixture(self, root):
        source, destination = root / 'incoming', root / 'published'
        source.mkdir()
        (source / 'oshell-portable').write_text('portable')
        (source / 'OShell Desktop.exe').write_bytes(b'new-program')
        destination.mkdir()
        (destination / 'oshell-portable').write_text('portable')
        (destination / 'OShell Desktop.exe').write_bytes(b'old-program')
        return source, destination

    def test_rebuild_does_not_erase_portable_user_data(self):
        with tempfile.TemporaryDirectory() as folder:
            source, destination = self.fixture(pathlib.Path(folder))
            (destination / 'config.json').write_bytes(b'user-configuration')
            with self.assertRaisesRegex(RuntimeError, '--out'):
                build.publish(source, destination)
            self.assertEqual((destination / 'config.json').read_bytes(), b'user-configuration')
            self.assertEqual((destination / 'OShell Desktop.exe').read_bytes(), b'old-program')
            self.assertFalse((source / 'config.json').exists())

    def test_clean_build_can_be_replaced(self):
        with tempfile.TemporaryDirectory() as folder:
            source, destination = self.fixture(pathlib.Path(folder))
            build.publish(source, destination)
            self.assertEqual((destination / 'OShell Desktop.exe').read_bytes(), b'new-program')

    def test_failed_publication_restores_previous_build(self):
        with tempfile.TemporaryDirectory() as folder:
            source, destination = self.fixture(pathlib.Path(folder))
            rename = pathlib.Path.rename

            def fail_incoming(path, target):
                if path.name == destination.name and path.parent.name.startswith('.oshell-publish-'):
                    raise OSError('simulated publication failure')
                return rename(path, target)

            with patch.object(pathlib.Path, 'rename', fail_incoming):
                with self.assertRaisesRegex(OSError, 'simulated'):
                    build.publish(source, destination)
            self.assertEqual((destination / 'OShell Desktop.exe').read_bytes(), b'old-program')


if __name__ == '__main__':
    unittest.main()
