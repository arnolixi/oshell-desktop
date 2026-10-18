# 开发约定

## 开发、提交和 Actions

- 日常开发只修改需求相关代码并本地验证；用户明确要求时才提交或推送，不为验证而擅自推送。
- 代码提交到 GitHub 后，只有用户另行要求才触发 Actions。工作流保持手动触发；提交、推送、PR 和标签推送不自动运行。
- 触发后提供链接即可，默认不跟踪、不轮询、不自动重跑。用户另行要求检查或修复时再处理。
- 缺少验证环境时如实说明，不以提交或运行 Actions 代替授权。

## 本地便携包

- 每次完善功能或修复后，本机编译并测试 Windows 单文件便携包，再更新固定 `dist` 中的 EXE 和 ZIP。用户已持续授权本机工具链，无需逐次询问；此前本地编译必须用容器的限制不再适用，CI 编译仍使用容器。
- 产物统一放入固定目录，不按日期、提交或构建次数新增生产输出目录。验证 ZIP 完整性和独立目录启动后，只替换程序文件，校验原配置未改变。
- EXE 被占用时先完成构建和测试，再请用户保存并退出；不得强制结束会话。

## 跨端配置兼容

- OShell 与 Desktop 的共享配置须持续双向兼容。修改会话、代理、目录、引用、密码、默认值、共享偏好、归档或同步时，核对两端字段、类型、默认值及身份绑定，不能只验证一端。
- 协议入口：[配置协议 v2](docs/configuration-protocol-v2.md)；字段唯一维护源：`internal/core/shared_configuration_schema.json`。Swift 合同由脚本生成，不能各自增加共享字段后回写。
- 共享配置读 v1/v2、写 v2；归档版本独立。未知字段、版本或无法理解的数据拒绝应用和覆盖，不能绕过校验或静默丢字段。
- 两端各自保留本机数据目录，通过 `OShell.shared-vault` 交换。WebDAV 使用 `oshell-vault.json`，目录同步使用 `configuration.json`；平台专属状态留在本机。
- 保留密码解密与身份重绑定、引用完整性、首次同步确认、备份、强 ETag / 条件写入、多节点合并和冲突保护。密码错误、数据损坏、过期或冲突未解决时不能覆盖有效数据。
- 无逻辑修改不重复上传；空数组、UUID 大小写和默认值规范化不应制造冲突。使用合成数据测试，不将真实配置、密码、私钥、WebDAV 凭据或本机密钥放入代码和日志。
- 不擅自提交、推送或发布另一个项目中的无关修改。

## 协议验证

```sh
go test ./internal/core -run 'Test(NativeSwiftSharedFixture|SharedQuickSendScope|LegacySharedProtocol|SharedEncodingNormalization)'
python3 scripts/test-sync-interop.py --native-source /path/to/OShell
```

第二条需要 macOS 和真实 OShell 源码，验证 Swift ↔ Go 往返、密码与代理、多节点同步、冲突及无变化不重写。合同新增或变更字段时同步扩充两端测试。

Windows / Linux 至少执行 Go 兼容样本及相关回归；缺少 Swift 环境时明确报告，跳过不能视为真实互通通过。协议发布前完成真实双端验证，详细步骤见 v2 规范。
