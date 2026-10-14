#!/usr/bin/env python3
"""Build the OShell native VT extension from its pinned upstream source.
Usage: build-native-macos.py --source /path/to/clean/ghostty --zig /path/to/zig
The supplied checkout is never modified; the build uses a temporary export.
"""
import argparse, hashlib, json, pathlib, platform, shutil, subprocess, tarfile, tempfile
p = argparse.ArgumentParser()
p.add_argument('--source', required=True, type=pathlib.Path)
p.add_argument('--zig', default='zig')
p.add_argument('--arch', choices=['arm64','amd64'], default='arm64' if platform.machine()=='arm64' else 'amd64')
a=p.parse_args()
root=pathlib.Path(__file__).resolve().parent.parent
commit='befcdfd2c3a1cb24d9ec886e93c95b2b5daa7028'
zig=shutil.which(a.zig) or str(pathlib.Path(a.zig).resolve())
version=subprocess.check_output([zig,'version'],text=True).strip()
if version!='0.16.0': raise SystemExit('Requires Zig 0.16.0')
patch=root/'native/ghostty/oshell-width.patch'
with tempfile.TemporaryDirectory(prefix='oshell-native-') as folder:
 work=pathlib.Path(folder);src=work/'source';src.mkdir()
 archive=work/'source.tar'
 with archive.open('wb') as out: subprocess.run(['git','archive',commit],cwd=a.source,stdout=out,check=True)
 with tarfile.open(archive) as tar: tar.extractall(src, filter='data')
 subprocess.run(['git','apply',str(patch)],cwd=src,check=True)
 prefix=work/'built'
 target={'arm64':'aarch64-macos','amd64':'x86_64-macos'}[a.arch]
 subprocess.run([zig,'build','-Demit-lib-vt','-Demit-xcframework=false','-Demit-macos-app=false','-Dtarget='+target,'-Doptimize=ReleaseFast','-Dstrip=true','--prefix',str(prefix)],cwd=src,check=True)
 output=root/'resources'/('darwin-'+a.arch);output.mkdir(parents=True,exist_ok=True)
 library=output/'oshell-ghostty-vt.dylib'
 shutil.copyfile(prefix/'lib/libghostty-vt.dylib',library)
 manifest=dict(upstream=commit,zig=version,patchSHA256=hashlib.sha256(patch.read_bytes()).hexdigest(),sha256=hashlib.sha256(library.read_bytes()).hexdigest(),target='darwin-'+a.arch)
 (output/'oshell-ghostty-vt.json').write_text(json.dumps(manifest,indent=2)+'\n')
 print(library)
