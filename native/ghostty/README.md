# OShell terminal extension

Upstream: Ghostty `befcdfd2c3a1cb24d9ec886e93c95b2b5daa7028`.
Compiler: Zig 0.16.0. Unicode ranges match OShell 0.2.67, Unicode 17.

`oshell-width.patch` adds a creation-time ambiguous-width policy, retaining
zero-width combining characters and wide emoji. The policy survives reset.
The optional `ghostty_oshell_abi_version` symbol gates the custom enum, so an
unmodified optimized upstream library never receives an unknown enum value.

Rebuild on macOS:

```
python3 scripts/build-native-macos.py --source /path/to/ghostty --zig /path/to/zig
```

This exports the pinned commit into a temporary directory, applies the patch,
and writes the library and provenance JSON into platform-specific resources.
MyGo bundles/signs these native resources under the distinct filename
`oshell-ghostty-vt.dylib`; the runtime prefers it to the upstream dependency.
Direct Go tests use `MYGO_GHOSTTY_VT` pointing to that generated library.
The generated ARM64 library has been tested on macOS; other targets remain
pending the unified cross-platform validation phase.

Tests: `TestOShellAmbiguousWidth` covers cursor movement, combining characters,
emoji, variation selectors, wrapping, reset and copied text without padding.
Unicode licensing is in `UNICODE-LICENSE.txt`. Ghostty's upstream license is
retained in `resources/licenses`.
