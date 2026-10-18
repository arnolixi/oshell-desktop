# CI 构建与测试

工作流：[build.yml](../.github/workflows/build.yml)。仅手动 `workflow_dispatch` 触发，push / PR / 标签推送均不自动运行。使用 GitHub 官方 Runner，不依赖自托管服务器。

## 执行结构

| 任务 | 执行内容 |
| --- | --- |
| package | Ubuntu Docker 内顺序构建 Windows / Linux 包，并预编译 Windows 测试 |
| linux | 容器内完整回归、虚拟显示桌面测试、PTY / ZMODEM 竞态检查 |
| core-race-linux | 4 个 Ubuntu Runner，各自在容器内执行一个 core 竞态分片 |
| windows | Windows 原生执行预编译测试及便携包启动、升级、配置保留验证 |
| core-race-windows | 4 个 Windows Runner 执行预编译 core 竞态分片 |
| core-race-audit | 汇总两平台分片，检查遗漏、重复、失败和不完整日志 |
| release | 在版本标签上手动运行且所有必需任务通过后发布 |

所有 Go 编译和静态检查都在容器内完成。Windows Runner 只执行二进制，执行前校验其提交号；产物提前上传不代表全部测试通过。

## 缓存与分片

- 依赖镜像按 Dockerfile、架构和月份缓存，命中后直接装载；Go 模块、编译缓存及原生资源分别复用，测试结果不缓存。
- Linux core 分片共享专用缓存，仅分片 0 保存；Windows 普通回归与 core 竞态使用独立测试包，减少下载量。
- 容器挂载工作区、只读工具链和缓存，不挂载 Docker socket 或宿主凭据；结束或取消时恢复目录所有权。
- `core_test_shards.py` 从当前二进制发现全部测试，根据 `core-race-timings.json` 均衡分片；新增测试使用默认权重，不会漏测。
- 每片保留原始日志和清单。审计要求同一提交、两平台各 4 片、每个测试恰好执行一次并正常结束；跳过项单独报告。

维护耗时权重时使用完整竞态日志，不能用普通测试耗时替代。比较性能应分别记录冷缓存、热缓存和完整流程耗时；历史单次计时不代表后续运行速度。

操作规则见 [AGENTS.md](../AGENTS.md)，发布步骤见 [更新与发布](updates-portable.md)。
