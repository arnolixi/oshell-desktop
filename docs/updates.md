# Windows / Linux

Windows 单文件便携版和 Linux 运行包已改用 GitHub Release 更新，安装及回滚流程见 [updates-portable.md](updates-portable.md)。下文保留原 macOS 签名更新协议说明，不代表 Windows / Linux 的实现。


# OShell Desktop signed updates

Development status: metadata checks and verified streaming download are implemented.
Bounded extraction and macOS atomic replacement/rollback are implemented.
The offline release tool is implemented and tested; full native signed-update
installation acceptance remains pending. macOS settings/actions and verified post-relaunch backup cleanup are wired. No production signing key has been generated or assumed.

Behavior follows the 0.2.67 static Pages workflow: select a public GitHub repository,
fetch `updates/latest.json` from its Pages site, cache success for five minutes,
and back off failures for at least one minute (respecting Retry-After up to a day).
It does not call the GitHub API or require a token. Cancellation is not cached as
a server failure.

Desktop is a separate product. It cannot install the original OShell Sparkle DMG.
Its metadata envelope is `OShellDesktop.signed-updates.v1`:

- `payload`: base64 of the exact UTF-8 JSON manifest bytes, maximum 32 KiB.
- `signature`: base64 Ed25519 signature over the UTF-8 domain prefix
  `OShellDesktop update metadata v1` followed by a newline and those payload bytes.
- `format`: the envelope format identifier above.

The signed manifest binds repository, applicationID, version, build, notes and
platform-specific URL/size/SHA256/signature/minimumOS. macOS platform names are
`darwin-arm64` and `darwin-amd64`, minimumOS `13.0`. Version is a three-part stable
release and build is a positive decimal up to 18 digits. Older versions/builds are
never offered. The trusted public key is pinned in the release build, not supplied
by the feed. A missing build key disables acceptance.

Archives are named `OShell-Desktop-<version>-<target>.tar.gz` and downloaded from
that repository's `releases/download/v<version>/` path. Archive signatures sign
the raw 32-byte SHA-256 digest. Both metadata and archive signatures use Ed25519.
Downloads are bounded to the signed size (at most 1 GiB), streamed without loading
the archive in memory, and verified before being returned. Failed/cancelled files
are removed. Display fields cannot mutate the verified download authority.

Core tests exercise real ephemeral signing keys, tampered metadata/archives,
product/source binding, rollback checks, cache expiry/backoff and cleanup. These
checks do not constitute full updater installation acceptance.

## macOS staging and replacement

The downloaded archive is opened without following a final symlink and its size,
digest and signature are checked again before extraction. It contains exactly one
`.app`, with at most 100,000 entries, a 1 GiB per-file limit and a 4 GiB expanded
limit. Traversal, absolute names, case/Unicode-equivalent duplicate paths, hard
links and special files are rejected. Framework-style downward relative symlinks
are created only after ordinary files; parent traversal and symlink parents are
not allowed. Tar trailers and gzip integrity are also checked.

The bundle's identifier, version, numeric build, executable, architecture and
minimum OS must match the verified metadata. macOS additionally runs strict deep
code-signature verification. Staging occurs next to the installed application.
Both the original and staged trees are fingerprinted, then checked again before
commit, so intervening modifications abort the operation. Commit uses Darwin's
same-volume atomic exchange. The old application remains in the staging area for
rollback; explicit finalization removes it after successful relaunch. The native
UI and durable post-relaunch cleanup still need integration.

Tests use a temporary ad-hoc-signed copy of a real system Mach-O executable to
exercise native signature verification, atomic exchange and rollback. They also
verify cancellation, rename failure, traversal/link rejection, product mismatch,
mutated staging, safe framework symlinks and preservation of files outside the
application. These fixtures never replace an installed user application.

The installed bundle supplies `OShellDesktopUpdatePublicKey` (base64 Ed25519 public
key) and optional `OShellDesktopUpdateRepository` through Info.plist; release builds
also use a positive numeric CFBundleVersion. Link-time UpdatePublicKey/UpdateBuild
overrides are supported. Settings never accept a trust key from a repository.
The update settings and native menu are wired on macOS, with cancellable download,
installation confirmation, session-disposal waiting and relaunch. A private marker
inside the installer staging directory must exactly match the persistent receipt
before startup cleanup may remove it. Full signed native UI acceptance is pending.

## Offline release workflow

The `cmd/oshell-release` tool never uploads files or installs an application.
No production key is generated during normal builds or update checks. A release
maintainer may explicitly create a key in a private directory:

```sh
go run ./cmd/oshell-release -generate-key -key-file /secure/oshell-desktop.key
```

The command creates a 0600 file without overwriting an existing key and prints
only the public key. To read that public key again:

```sh
go run ./cmd/oshell-release -public-key -key-file /secure/oshell-desktop.key
```

Set these build-owned values under `macos.infoPlist` in `mygo.json`, using the
actual public key and monotonically increasing numeric build:

```json
{
  "CFBundleVersion": "41",
  "OShellDesktopUpdatePublicKey": "<base64 public key>",
  "OShellDesktopUpdateRepository": "owner/repo"
}
```

Keep the normal top-level version (for example `0.4.1`) in sync with the release.
Build/sign the app with MyGo, then generate offline release artifacts:

```sh
go run ./cmd/oshell-release \
  -app "dist/darwin-arm64/OShell Desktop.app" \
  -repository owner/repo -target darwin-arm64 \
  -key-file /secure/oshell-desktop.key -out release-output
```

The publisher verifies the application's pinned public key, identifier, version,
minimum OS and native code signature. It round-trips the archive through the same
bounded extractor and verifier the consumer uses before signing metadata. Existing
same-version archives are never overwritten. Matching platform entries for the
same version/build are preserved when publishing another architecture into the
same output directory. An existing feed for a different product/repository or a
version rollback is rejected. To remake an artifact intentionally, use a fresh
output directory and review it before publishing.

Outputs are the versioned tar.gz and `updates/latest.json`. Upload remains a
separate maintainer action. These commands were exercised only with temporary
keys and temporary signed applications during development.
