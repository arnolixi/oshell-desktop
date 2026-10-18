# OShell terminal adapter

Derived from [egoist/mygo](https://github.com/egoist/mygo) plugins/terminal at
`bad73d119dc8d00a10daba45f19ba484e754d7a8`. The original MIT license is retained
in [LICENSE](LICENSE).

The local package is `oshell-desktop/internal/terminal`. It combines the native
mygo terminal view, platform PTY support and libghostty-vt, loaded at runtime
without cgo. [mygo-plugin.json](mygo-plugin.json) pins native library downloads
and SHA-256 hashes; the application build collects the Windows DLL or Linux
shared library. Windows single-file builds embed the DLL and its licenses.
See the [project README](../../README.md) for supported builds and runtime setup.

OShell adds semantic input observation/replay, paste policy, copy trimming,
output observation/logging, viewport search, highlight rendering and configurable
word selection. The optional ambiguous-width native API is detected at runtime;
the pinned Windows DLL does not implement it.

Upstream parser, PTY and renderer tests remain alongside OShell regressions.
CI executes these on Windows and Linux, including PTY race detection.
