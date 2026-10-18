# Third-party notices

OShell Desktop original code and OShell artwork: GPL-3.0-only, see LICENSE.

Dependencies retain their own licenses. Exact module versions are in go.mod / go.sum.

| Component | License | Source |
| --- | --- | --- |
| mygo (local runtime renderer-selection patch) | MIT | https://github.com/egoist/mygo |
| libghostty-vt | MIT | https://github.com/ghostty-org/ghostty |
| purego | Apache-2.0 | https://github.com/ebitengine/purego |
| go-text/typesetting | BSD-3-Clause / Unlicense | https://github.com/go-text/typesetting |
| golang.org/x/crypto, x/sys, x/image | BSD-3-Clause | https://go.googlesource.com/ |
| pkg/sftp | BSD-2-Clause | https://github.com/pkg/sftp |
| jlaffaye/ftp | ISC | https://github.com/jlaffaye/ftp |
| Microsoft/go-winio | MIT | https://github.com/microsoft/go-winio |
| xfer ZMODEM implementation | WTFPL (upstream README) | https://github.com/arttu76/xfer |
| DejaVu Sans Mono | Bitstream Vera / public domain additions | https://dejavu-fonts.github.io/ |
| Symbols Nerd Font | MIT and component licenses | https://github.com/ryanoasis/nerd-fonts |
| kr/fs | BSD-3-Clause | https://github.com/kr/fs |

Bundled library builds and SHA-256 hashes are pinned by internal/terminal/mygo-plugin.json. Ghostty theme data and fonts may carry separate upstream notices. License texts for the shipped components are included under resources/licenses.

## OShell 0.2.67 additions

- regexp2 v1.12.0 (MIT): resources/licenses/regexp2-LICENSE.
- golang.org/x/text v0.42.0 and golang.org/x/term v0.46.0 (BSD): corresponding resources/licenses files.
- Unicode 17.0.0 East Asian Width ranges (Unicode-3.0): resources/licenses/Unicode-LICENSE.
- Windows / Linux Ghostty binaries use the pinned builds in internal/terminal/mygo-plugin.json. The optional OShell ambiguous-width API is detected at runtime; the Windows binary does not provide it. Original Ghostty licensing remains applicable.

## FTP listing metadata patch

`third_party/ftp` contains github.com/jlaffaye/ftp v0.2.4 (MIT; see its LICENSE). Local changes retain UNIX LIST and MLSD permissions and ownership fields for file display.

`third_party/sftp` contains github.com/pkg/sftp v1.13.11 (BSD-2-Clause; see LICENSE), patched to retain verified UNIX long-listing owner/group display hints. Numeric SFTP attributes remain authoritative.
