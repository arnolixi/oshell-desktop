# macOS 外部启动入口

构建 macOS 应用时会自动编译以下连接助手，并放入应用的
`Contents/Resources` 目录：

- `OShell-ZOC`：支持 `/CONNECT` 或 `/SSH`、`/SSHUSER`、`/SSHPASSWORD`、
  `/SSHKEY`、`/TITLE`、`/EMU` 等连接参数，仅支持 SSH 和 UTF-8。
- `OShell-FileZilla`：支持 FTP/SFTP URL、`--site=0/目录/站点`、
  `--site 0/目录/站点` 或 `-c 0/目录/站点`。

也支持把堡垒机启动路径直接指向 `Contents/MacOS/OShell Desktop`：主程序识别原版 ZOC 参数、FTP/SFTP URL 或 FileZilla 站点参数，并转交同一应用包中的助手；普通启动参数不会触发转交。

入口只打开一次性会话，不把启动密码或临时会话写入配置。原有 SSH
主机指纹校验仍会执行。主密码锁定时，请在应用窗口解锁后继续。

FileZilla 站点默认从用户的 `.config/filezilla/sitemanager.xml` 或
`.filezilla/sitemanager.xml` 读取；可用 `OSHELL_FILEZILLA_CONFIG` 指定文件。
只选择请求的站点，不导入或改写整个数据库。不支持 FTPS、脚本参数、
FileZilla 专有加密密码及外部 XML 实体。

助手必须留在完整的 OShell Desktop.app 中。它定位同一应用包中的主程序，
通过当前用户专用的本机通道转交请求；不同启动数据根目录使用不同通道；设置中迁移数据目录后，启动通道仍使用稳定的根目录定位。
`OSHELL_DATA_DIR` 可指定隔离的测试配置，助手与应用需要使用同一个值。

开发状态：参数、本机通道及两个实际助手可执行文件到运行中通道的转交测试已通过。测试使用隔离应用包、隔离配置和合成凭据，配置目录未写入会话或凭据。SSH／SFTP 的原生冷启动、锁定等待、解锁后终端／文件窗口路由也已验证，配置及凭据文件保持不变。测试目标使用 .invalid 域名，不代表真实服务器认证验收；整体 0.2.72 复刻仍在核查。
