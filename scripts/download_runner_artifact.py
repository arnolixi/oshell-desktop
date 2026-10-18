"""Download one immutable CI artifact with bounded retries and SHA256 verification."""
import argparse
import hashlib
from http.client import HTTPException
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tempfile
import time
from urllib.error import HTTPError
from urllib.parse import urlsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener
import zipfile


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def artifact_location(opener, repository, artifact_id, token):
    request = Request(f'https://api.github.com/repos/{repository}/actions/artifacts/{artifact_id}/zip',
                      headers={'Authorization': f'Bearer {token}', 'Accept': 'application/vnd.github+json',
                               'X-GitHub-Api-Version': '2022-11-28'})
    try:
        with opener.open(request, timeout=20):
            raise ValueError('Expected an artifact redirect')
    except HTTPError as error:
        try:
            if error.code != 302:
                raise
            location = error.headers.get('Location', '')
        finally:
            error.close()
    parsed = urlsplit(location)
    if parsed.scheme != 'https' or not (parsed.hostname or '').endswith(('.blob.core.windows.net', '.githubusercontent.com')):
        raise ValueError('Unexpected artifact storage destination')
    return location


def download(repository, artifact_id, token, digest, target, *, attempts=3,
             opener=None, clock=time.monotonic, sleep=time.sleep):
    opener = opener or build_opener(NoRedirect())
    for attempt in range(attempts):
        started = clock()
        try:
            location = artifact_location(opener, repository, artifact_id, token)
            sha = hashlib.sha256()
            size = 0
            # A new request deliberately omits GitHub authorization on the storage host.
            with opener.open(Request(location), timeout=20) as response, target.open('wb') as output:
                while True:
                    if clock() - started > 120:
                        raise TimeoutError('Artifact attempt exceeded two minutes')
                    chunk = response.read1(1024 * 1024)
                    if not chunk:
                        break
                    sha.update(chunk)
                    size += len(chunk)
                    output.write(chunk)
            if sha.hexdigest() != digest:
                raise ValueError('Artifact SHA256 mismatch')
            print(f'Artifact verified: {size} bytes, attempt {attempt + 1}, {clock() - started:.1f}s', flush=True)
            return
        except (OSError, ValueError, HTTPException) as error:
            target.unlink(missing_ok=True)
            # Never print exceptions containing signed storage URLs or request headers.
            print(f'Artifact attempt {attempt + 1} failed ({type(error).__name__})', flush=True)
            if attempt + 1 == attempts or isinstance(error, HTTPError) and error.code in (401, 403, 404):
                raise RuntimeError('Artifact download failed; no unverified files were installed') from None
            sleep(2 * (attempt + 1))


def extract_verified(archive, destination, commit):
    with zipfile.ZipFile(archive) as bundle:
        seen = set()
        for entry in bundle.infolist():
            original = entry.orig_filename  # ZipInfo normalizes backslashes on Windows.
            path = PurePosixPath(original)
            if (not path.parts or path.is_absolute() or '..' in path.parts or '\\' in original
                    or ':' in original or '\0' in original or path.parts[0] not in ('windows-tests', 'container-package')
                    or stat.S_ISLNK(entry.external_attr >> 16) or path.as_posix().casefold() in seen):
                raise ValueError('Unsafe or duplicate artifact entry')
            seen.add(path.as_posix().casefold())
        manifest = json.loads(bundle.read('windows-tests/manifest.json'))
        if manifest['commit'] != commit:
            raise ValueError('Artifact does not belong to this commit')
        if 'container-package/oshell-desktop-windows-amd64-single-file.zip' not in seen:
            raise ValueError('Portable package missing')
        destination.mkdir(parents=True, exist_ok=True)
        for name in ('windows-tests', 'container-package'):
            if (destination / name).exists():
                raise ValueError('Artifact destination already exists; refusing stale files')
        with tempfile.TemporaryDirectory(prefix='artifact-stage-', dir=destination) as stage:
            bundle.extractall(stage)
            for name in ('windows-tests', 'container-package'):
                (Path(stage) / name).rename(destination / name)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--id', required=True, type=int)
    parser.add_argument('--digest', required=True)
    args = parser.parse_args()
    repository = os.environ['GITHUB_REPOSITORY']
    digest = args.digest.removeprefix('sha256:').lower()
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repository) or not re.fullmatch(r'[0-9a-f]{64}', digest):
        raise ValueError('Invalid artifact identity')
    with tempfile.TemporaryDirectory(prefix='oshell-artifact-', dir=os.environ['RUNNER_TEMP']) as scratch:
        archive = Path(scratch) / 'bundle.zip'
        download(repository, args.id, os.environ['GH_TOKEN'], digest, archive)
        extract_verified(archive, Path('validation'), os.environ['GITHUB_SHA'])


if __name__ == '__main__':
    main()
