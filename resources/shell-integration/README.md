# Shell 集成

可选的 Bash / Zsh 脚本，在提示符出现时向 OShell 上报主机名和 IP，改善直接及嵌套 SSH 的标题识别。不注入探测命令，不修改命令历史；未安装时默认使用被动识别。

## 安装

1. 在远端以登录用户执行 `mkdir -p "$HOME/.config/oshell"`。
2. 用 SFTP 将本目录的 `oshell-integration.sh` 上传到 `~/.config/oshell/oshell-integration.sh`。文件只需可读，不需要 root 或执行权限。
3. 在 Bash 的 `~/.bashrc` 或 Zsh 的 `~/.zshrc` 末尾添加：

   ```sh
   [ -r "$HOME/.config/oshell/oshell-integration.sh" ] && . "$HOME/.config/oshell/oshell-integration.sh"
   ```

4. 会话属性选择“接受 Shell 集成上报”，然后重新连接。每个嵌套服务器及账号都需安装。

Bash 登录配置应加载 `.bashrc`；不生效时检查现有 `.bash_profile` / `.bash_login` / `.profile`，不要覆盖原文件。Zsh 使用自定义 `ZDOTDIR` 时修改其下的 `.zshrc`。

## 行为与限制

- 优先上报 `SSH_CONNECTION` 中的服务端 IP，否则读取接口地址；网络信息缓存最多 30 秒，连接信息变化时刷新。
- 保留 Bash `PROMPT_COMMAND`、退出状态和 Zsh `precmd` 钩子；重复加载不重复注册，不覆盖只读钩子。
- 不更改 `PS1`、历史选项或历史文件。元数据只用于标题，不决定认证、连接和文件传输目标。
- tmux / screen 可能过滤 OSC 上报，需自行验证透传；脚本不修改复用器配置。主动探测只有在会话属性中明确启用后才运行。

## 停用

删除启动文件中的加载行并重新登录。当前 Shell 可设置 `OSHELL_INTEGRATION=0` 暂停上报；完整恢复原钩子状态需重新登录。
