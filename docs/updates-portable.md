# Windows / Linux 自更新

Windows amd64 单文件便携版和 Linux amd64 运行包使用 GitHub Release。
默认仓库是 `oshell-desktop/oshell-desktop`，设置 → 更新中可修改仓库，手动检查，或启用每日自动检查。
自动检查只提示新版本；下载及安装均由用户确认。安装会关闭所有窗口中的连接和文件传输。

私有仓库可在同一页面填写 GitHub 访问令牌，授予目标仓库 Contents 读取权限，点击“保存令牌”。先应用仓库地址再保存令牌。令牌按仓库隔离，在数据目录的 `update-auth/` 独立本机密码库中加密保存；不写入会话配置或共享同步文件，不回显。公开仓库无需令牌。下载私有附件走 GitHub asset API，跳转到 CDN 时移除 Authorization，令牌失效可替换或清除。

## 发布约定

1. 修改 `mygo.json` 中的三段正式版本号，例如 `0.3.2`。
2. 将通过检查的代码推送后，创建并推送相同版本的 `v0.3.2` 标签。
3. Build and test 工作流在 Windows 和 Linux 测试、构建均成功后发布 Release。
   标签与程序内版本不匹配会拒绝发布；不会覆盖已有 Release。

附件名称必须为：

- `OShell-Desktop-windows-amd64-single-file.zip`：仅含 `windows-amd64-single-file/OShell Desktop.exe`。
- `OShell-Desktop-linux-amd64.tar.gz`：程序 `oshell-desktop`、`libghostty-vt.so` 以及构建脚本生成的资源。

更新器读取 `https://api.github.com/repos/<owner>/<repo>/releases/latest`，只接受比当前版本新的正式三段版本。
不会安装草稿、预发布版、旧版本或相同版本。附件必须来自该 Release、该仓库、正确平台，并具有 GitHub 提供的 `sha256:` digest。
缺少 digest 的旧附件需重新上传，不能绕过校验。
API 结果缓存五分钟，失败退避一分钟。下载限制大小，校验完整长度和 SHA-256；取消或损坏下载不会进入安装阶段。

这里的信任来源是 GitHub HTTPS API、仓库发布权限和 GitHub 附件摘要，**不宣称独立发布者数字签名**。
原 macOS 的 Ed25519/GitHub Pages 协议保持独立，不参与这两个平台的发布。
参考：[GitHub Release API](https://docs.github.com/en/rest/releases/releases)、[Release asset digest](https://docs.github.com/en/rest/releases/assets)。

## 安装、配置与回滚

下载及解压在程序所在目录的私有 `.oshell-update-*` 临时目录完成；该目录必须可写。
压缩包不允许路径穿越、符号链接、特殊文件、重复路径或配置文件。Windows 只替换 EXE，Linux 只替换明确允许的程序资源。
`config.json`、`credentials.json`、`credential-key`、`known_hosts`、同步状态和配置事务文件均不属于更新目标。

确认安装后，当前 EXE 的副本作为后台更新进程运行。Windows 不启动 cmd/PowerShell，也不弹控制台窗口。
更新进程等待旧进程退出，先备份全部待替换文件，再逐个原子替换。新版启动并完成窗口、配置及密码库初始化，且内置版本号与 Release 一致后才确认升级。
新版无法启动、提前退出或 90 秒内未确认时，更新进程终止其启动的进程、恢复旧文件并重新启动旧版。
备份恢复是幂等操作；未完成的更新保留事务日志，下次手动启动时自动尝试恢复。
如果文件权限、杀毒软件或另一个实例占用文件导致恢复也失败，备份和事务继续保留，错误会提示备份目录；解决占用/权限后再次启动即可重试。
成功完成的备份目录在后续启动时清理。更新结果显示在设置 → 更新页面。

Linux 自更新适用于解压到用户可写目录的 tar.gz 运行包。系统包管理器安装、只读目录或系统目录不会自动提权，请使用包管理器或移到可写目录。
本机 Windows 测试不等同于 Linux 原生桌面验收；Linux 原生测试在 Ubuntu CI 中执行。

## 回归验证

`go test ./internal/core -run 'Test(Release|Portable)'` 覆盖正式版本选择、拒绝降级、附件来源、摘要校验、取消下载、危险压缩包、配置保留和重复回滚。
`TestPortableHelperLifecycle` 使用真实子进程验证 Windows/Linux 可执行文件占用、成功重启、新版崩溃回滚和中断后恢复，不访问真实用户配置。

实际 Windows 单文件包验证可运行 `./scripts/test-update-package.ps1 -Executable <构建后的EXE> -FixtureExecutable <容器预编译的update-fixture.exe>`，使用独立合成会话和加密密码，检查重启、配置哈希及密码解密。
