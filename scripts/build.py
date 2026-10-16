#!/usr/bin/env python3
"""Build a native app locally; add --archive for GitHub Actions artifacts."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import platform
import plistlib
import shutil
import subprocess
import tempfile
import zipfile
import code_signing

ROOT = Path(__file__).resolve().parent.parent


def run(*args, env=None):
    subprocess.run(args, cwd=ROOT, env=env, check=True)


def publish(source, destination):
    """Stage the complete result before replacing the previous build."""
    if (source / 'oshell-portable').is_file() and destination.exists():
        # Portable apps store credentials beside the executable. Rebuilding
        # must neither delete those files nor include them in a release ZIP.
        incoming_files = {p.relative_to(source) for p in source.rglob('*') if p.is_file()}
        for existing in destination.rglob('*'):
            if existing.is_file() and existing.relative_to(destination) not in incoming_files:
                raise RuntimeError('Portable output contains local data or extra files; choose a clean --out directory.')
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='.oshell-publish-', dir=destination.parent) as folder:
        incoming = Path(folder) / destination.name
        shutil.copytree(source, incoming)
        backup = Path(folder) / 'previous'
        had_previous = destination.exists()
        if had_previous:
            destination.rename(backup)
        try:
            incoming.rename(destination)
        except OSError:
            if had_previous:
                backup.rename(destination)
            raise


def build(target, output, archive=False, build_number=None):
    goos, arch = target.split('/')
    if goos == 'darwin' and platform.system() != 'Darwin':
        raise SystemExit('macOS signing/packaging must run on a Mac.')
    if goos == 'darwin':
        native = ROOT / 'resources' / f'darwin-{arch}'
        provenance = json.loads((native / 'oshell-ghostty-vt.json').read_text())
        actual = hashlib.sha256((native / 'oshell-ghostty-vt.dylib').read_bytes()).hexdigest()
        if actual != provenance['sha256']:
            raise SystemExit('Pinned OShell terminal library checksum mismatch.')
    generated = ROOT / 'mygo.ts'
    previous = generated.read_bytes() if generated.exists() else None
    with tempfile.TemporaryDirectory(prefix='oshell-build-') as folder:
        stage = Path(folder)
        try:
            run('go', 'tool', 'mygo', 'build', '-platform', target,
                '-skip-build-command', '-skip-dmg', '-o', str(stage))
        finally:
            # This native app has no TS client. Do not leave generated stubs.
            if previous is None:
                generated.unlink(missing_ok=True)
            else:
                generated.write_bytes(previous)
        platform_dir = stage / f'{goos}-{arch}'
        if goos == 'darwin':
            product = platform_dir / 'OShell Desktop.app'
            resources = product / 'Contents/Resources'
            helper = resources / 'OShell-ZOC'
            run('go', 'build', '-trimpath', '-ldflags=-s -w', '-o', str(helper),
                './cmd/oshell-launcher', env=dict(os.environ, GOOS=goos, GOARCH=arch, CGO_ENABLED='0'))
            shutil.copy2(helper, resources / 'OShell-FileZilla')
            if build_number:
                info_path = product / 'Contents/Info.plist'
                info = plistlib.loads(info_path.read_bytes())
                info['CFBundleVersion'] = build_number
                info_path.write_bytes(plistlib.dumps(info))
            code_signing.sign_app(product)
            run('codesign', '--verify', '--deep', '--strict', str(product))
            destination = output / product.name
        else:
            product = platform_dir
            destination = output / f'{goos}-{arch}'
            for installer in product.glob('OShell Desktop Setup *.exe'):
                installer.unlink()
            (product / '使用说明.txt').write_text('Windows amd64 绿色便携版\n完整解压后直接双击 OShell Desktop.exe。无需安装、启动脚本或管理员权限。\n配置、凭据、主机指纹等数据文件保存在 EXE 同级目录。移动时复制整个文件夹。\n请保留 oshell-portable 标记和 ghostty-vt.dll，并放在可写目录。\n旧版配置请先关闭旧版，再将原数据目录的完整数据文件复制过来；不要覆盖程序文件。\n更新时请先完全退出旧版，只替换程序文件，保留原配置、凭据、密钥及 state-transaction.json；新版启动会自动恢复未完成的保存。\n当前 Windows 终端库暂不支持歧义字符双宽扩展，请保持该选项关闭。\n', encoding='utf-8-sig')
            (product / 'oshell-portable').write_text('OShell portable mode: data stays beside the executable.\n', encoding='utf-8')

        publish(product, destination)
        if archive:
            zip_path = output / f'OShell-Desktop-{goos}-{arch}.zip'
            # Keep a previous archive until its replacement is complete.
            with tempfile.TemporaryDirectory(prefix='.oshell-archive-', dir=output) as temp:
                zipped = Path(temp) / zip_path.name
                if goos == 'darwin':
                    run('ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', str(destination), str(zipped))
                else:
                    with zipfile.ZipFile(zipped, 'w', zipfile.ZIP_DEFLATED) as out:
                        for file in sorted(destination.rglob('*')):
                            if file.is_file():
                                out.write(file, str(file.relative_to(destination.parent)))
                with zipfile.ZipFile(zipped) as check:
                    if check.testzip() is not None:
                        raise RuntimeError('Archive verification failed')
                os.replace(zipped, zip_path)
            print(zip_path)
        print(destination)


if __name__ == '__main__':
    default_os = {'Darwin': 'darwin', 'Windows': 'windows'}.get(platform.system())
    default_arch = {'arm64': 'arm64', 'aarch64': 'arm64', 'x86_64': 'amd64', 'AMD64': 'amd64'}.get(platform.machine())
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target', choices=['darwin/arm64', 'windows/amd64'], default=f'{default_os}/{default_arch}')
    parser.add_argument('--out', type=Path, default=ROOT / 'dist')
    parser.add_argument('--archive', action='store_true', help='also package a ZIP for CI')
    parser.add_argument('--build', default=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%d%H%M%S'))
    args = parser.parse_args()
    if args.target not in ('darwin/arm64', 'windows/amd64'):
        parser.error('Specify a supported --target (darwin/arm64 or windows/amd64).')
    build(args.target, args.out.resolve(), args.archive, args.build)
