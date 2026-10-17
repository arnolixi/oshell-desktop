# OShell Desktop

Go 共用核心、mygo 原生 UI 和 Ghostty VT 终端实现，功能参考 macOS OShell 0.2.74。
当前构建、测试和打包目标仅为 Windows amd64，生成绿色便携包。
本项目不替换原 Swift OShell，数据目录相互独立。

## 本机使用与构建

Windows 使用 Go 1.27.1+ 和 Python 3：

```powershell
python scripts/build.py --target windows/amd64 --archive
```

完整解压 `dist/OShell-Desktop-windows-amd64.zip` 后启动 `OShell Desktop.exe`。
若输出目录已产生配置、凭据或其他额外文件，构建会拒绝覆盖；使用 `--out` 指定干净目录，防止数据丢失或被打入发布包。
本机和 CI 均只构建 Windows；以下 macOS 说明保留供历史维护参考。

要求 Apple Silicon、macOS 13+、Go 1.27.1 和 Python 3。开发构建显式使用 ad-hoc 签名：

```sh
OSHELL_ALLOW_ADHOC=1 python3 scripts/build.py
open "dist/OShell Desktop.app"
```

构建默认只生成 `dist/OShell Desktop.app`，成功后替换上一版，不生成 ZIP 或 DMG。
脚本校验固定终端库、编译主程序与外部启动助手，最后进行签名和签名验证。
正式 macOS 构建使用 `scripts/code-signing.json` 固定的自签名身份；本地通过 `OSHELL_CODESIGN_DIRECTORY` 指向含 `identity.p12` 和 `password` 的私有目录。私钥不得提交。
Actions 的普通构建使用 ad-hoc；手动选择 `fixed_signing` 时，必须配置 `OSHELL_CODESIGN_P12`（Base64）及 `OSHELL_CODESIGN_PASSWORD` 仓库 Secrets，缺失或证书不匹配会失败。固定自签名不等于 Developer ID 或 Apple 公证。
这不是 Developer ID 公证发布版。首次构建可能需要下载 Go 依赖和固定原生运行库。

开发运行：

```sh
go run .
```

默认数据位于 `~/Library/Application Support/oshell-desktop`；可在“设置 → 存储”更改。
更改保存位置在完全退出并重启后迁移，旧数据保留。
`OSHELL_DATA_DIR=/absolute/path` 可指定独立测试数据根目录。

## GitHub Actions

工作流 `.github/workflows/build.yml` 仅在 Windows 上运行全量回归（包含桌面与终端）、核心与 PTY 竞态检测、编译和静态检查。
测试通过后生成 Windows amd64 ZIP，
从 Actions 页面的 Artifacts 下载，无需在本机保留兼容包。

CI 使用统一入口：

```sh
python scripts/build.py --target windows/amd64 --archive
```

macOS 打包必须在 Mac 上进行。Windows 包携带 EXE、固定 DLL 和许可证；
解压后直接双击 EXE 即可运行，无需启动脚本或修改 PowerShell 执行策略。
Windows 包带有 `oshell-portable` 标记，配置、凭据等数据直接存放在 EXE 同级目录；
便携模式忽略环境变量和旧的目录定位记录，移动时应复制整个程序文件夹。
`--out` 指定输出目录，`--build` 指定构建号；默认使用 UTC 时间生成构建号。
当前流水线不会上传 Release、执行 Apple 公证或配置生产更新签名密钥。

## 功能与边界

- SSH、本地终端、FTP / SFTP、SCP 与 ZMODEM 传输。每个文件标签独立限速，支持预设 / 自定义；SFTP / FTP 即时调整，SCP 下次传输生效。
- 文件权限、用户与组：FTP 保留 UNIX LIST / MLSD 属性；SFTP 优先显示经属性核对的服务器名称，否则显示 UID/GID。
- 会话目录、快捷引用、搜索、批量管理、导入导出和代理 / 跳板机。F2 / 右键列表内重命名，回车保存，Esc / 失焦取消；行尾按钮打开属性。
- 分屏、标签组、多窗口、快速命令、输入同步和终端日志。标签支持前后插入、跨组排序及边缘自动滚动，文件标签采用同样规则。
- 空闲终端原位复用、连接状态圆点、已连接标签双击复制、标签栏全区域右键菜单。
- 主密码与本机凭据保护、共享目录 / WebDAV 同步及冲突处理。会话属性 / 登录密码可切换本次输入显隐，已存密码不回显；修改目标后留空沿用并重新加密，取消保存则删除凭据。
- 配色与颜色选择器、键盘快捷键、自定义数据目录。
- 内置主题默认启用光标自动对比：按当前背景选择清晰的黑色或白色光标，光标内文字使用相反颜色。在“设置 → 主题与配色”关闭“光标颜色自动对比背景”后，可自行设置光标与光标文字颜色；已有自定义方案保留原设置。
- 终端聚焦时，普通 `Ctrl+字母` 优先交给终端程序处理，保留 `Ctrl+A` 行首、`Ctrl+E` 行尾、`Ctrl+W` 向前删词等常见 Shell 编辑操作；具体行为遵循当前 Shell / 程序的键位设置。Windows 默认全选为 `Ctrl+Shift+A`、关闭分屏为 `Ctrl+Alt+W`、关闭标签为 `Ctrl+Shift+W`。
- Windows 终端额外支持 `Ctrl+Backspace`（同 `Ctrl+W`）和 `Home`（同 `Ctrl+A`）；在“设置 → 键盘快捷键”可修改或清除这两项终端替代键，保存后立即生效。替代键不会接管撰写框等普通输入框；已有自定义快捷键优先。
- 终端左键双击选词、连续三击选行，支持继续拖动扩选；远端程序启用鼠标跟踪时，按住 `Shift` 使用本地选择。
- 独立时钟设置：字体、8–96 点字号、颜色、0–100% 透明度与九宫格位置；无背景浮层不占终端行列。

跨平台编译不等于实机验收；仍需核对 Windows 原生交互、实际更新安装/重启和真实服务兼容性。
Windows 当前固定终端 DLL 不含 OShell 歧义字符双宽扩展，该设置应保持关闭。
Touch ID 不在本项目范围。不要把移植功能已接入理解为全量原生验收完成。

## 验证

```sh
export MYGO_GHOSTTY_VT="$PWD/resources/darwin-arm64/oshell-ghostty-vt.dylib"
go test ./...
go test -race ./internal/core
go vet ./...
```

测试使用临时数据目录和本机模拟服务，不需要真实服务器凭据。
Windows CI 另用固定版本及 SHA-256 的 [lrzsz Windows 便携工具](https://github.com/trzsz/lrzsz-win32/releases/tag/v0.12.21rc)
验证真实 `rz` / `sz` 互通，覆盖终端自动识别、分片、CRC16 / CRC32、转义、中文文件名、批量传输、取消和下载落盘，以及同一会话连续上传后下载（含立即切换）。
本机可将 `rz.exe` / `sz.exe` 及其 DLL 放入 PATH，再设置 `OSHELL_REQUIRE_LRZSZ=1` 运行测试；测试工具不会打入产品包。
可设置 `OSHELL_TEST_ARTIFACTS=/absolute/path` 输出 UI 回归截图。
`validation/report-20261009-221430.zip` 是本机保留的用户 Windows 实测报告，不随源码上传，非安装包。

## 维护结构

- `internal/core`：配置、凭据、协议、同步、导入导出和更新验证。
- `internal/desktop`：窗口、原生 UI、交互与桌面测试。
- `internal/terminal`、`internal/zmodem`：终端实现、平台 PTY 与传输。
- `cmd/oshell-launcher`：macOS 外部连接助手；用法见 [外部启动说明](docs/external-launch.md)。
- `cmd/oshell-release`：离线签名工具；协议与使用方式见 [更新协议](docs/updates.md)。
- `scripts/build.py`：本机 / CI 共用打包入口。
- `scripts/build-native-macos.py`、`native/ghostty`：固定 Ghostty 源码的补丁与原生库重建工具。
- `resources/darwin-arm64`：固定原生库和校验清单，是构建输入，不能当作历史产物删除。
- `third_party/mygo`：固定框架源码，含运行时渲染后端切换补丁；保留上游结构、测试和许可证。

重建原生扩展需要固定 Ghostty commit（脚本内指定）和 Zig 0.16.0：

```sh
python3 scripts/build-native-macos.py --source /path/to/ghostty --zig /path/to/zig --arch arm64
```

许可见 [LICENSE](LICENSE) 和 [第三方声明](THIRD_PARTY_NOTICES.md)。
