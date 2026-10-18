# OShell 共享配置协议 v1

> 历史兼容规范。当前两端统一写入规则见 [v2 规范](configuration-protocol-v2.md) 和共享字段合同；本文件保留用于读取旧数据。

规范标识：`OShell.shared-vault/v1`
文档修订：`1.0`（2026-10-17）
维护源：`oshell-desktop/oshell-desktop` 仓库的 `docs/configuration-protocol-v1.md`
适用：macOS OShell 与 OShell Desktop 的共享配置交换。

本规范描述已实现的 v1 线格式及双方约束。文档修订号不是应用版本，也不增加任何磁盘字段。文中的“必须”是互通要求；第 9 节明确列出的扩展机制尚未实现，不可假定旧客户端已支持。

## 1. 范围与数据所有权

共享文件是各节点交换的加密快照。每个应用保持独立本机工作副本、凭据库、同步基准和冲突草稿；不是多个进程直接打开同一本机数据库。

共享内容：会话、目录、共享代理、代理链、保存的密码、快捷引用、快速命令、突出显示规则、默认会话属性和下文列出的偏好。

禁止共享：本机随机解密密钥、WebDAV 登录凭据、本机同步基准、日志、主机指纹库、Touch ID / 系统钥匙串绑定、应用签名私钥及私钥文件内容。`identityFile` 是路径文本，不是私钥附件，跨系统需保证路径可用。

配置同步与 `OShell.sessions` 会话导入导出是不同协议，不得把后者的版本号、导出文件或本机 `config.json` 直接当成共享文件。

## 2. 文件与外层加密信封

| 项目 | 值 |
| --- | --- |
| WebDAV | 配置目录下 `oshell-vault.json` |
| 同步目录 | `configuration.json` |
| 编码 | UTF-8 JSON，无 BOM；JSON 字段区分大小写 |
| `format` | 必填字符串，恰为 `OShell.shared-vault` |
| `version` | 必填整数，恰为 `1` |
| `iterations` | 必填整数；新写入 600000，读取允许 600000–2000000 |
| `salt` | 必填标准 Base64 字符串，解码后 16 字节 |
| `ciphertext` | 必填标准 Base64 字符串，解码后为 nonce、密文、tag 的串接 |

外层最大 48 MiB，解密前 JSON 内容最大 32 MiB（1 MiB = 1048576 字节）。Base64 使用标准字母表及必要的 `=` 填充，不是 Base64URL。

密钥派生：PBKDF2-HMAC-SHA256，密码为主密码原始 UTF-8 字节，盐为 `salt`，轮数为 `iterations`，结果 32 字节。不得自动 trim、大小写转换或 Unicode 归一化主密码。共享主密码非空，最长 4096 UTF-8 字节；新建主密码还需满足各应用的密码设置策略。

加密：AES-256-GCM，12 字节随机 nonce，16 字节认证 tag；`ciphertext` 解码后的排列为：

```text
nonce[12] || encryptedJSON[N] || tag[16]
AAD = UTF8("OShell.shared-vault.v1")
```

每次真正生成新信封必须使用新的安全随机盐和 nonce。解密失败、未知 `format` / `version`、参数超限时不得应用或覆盖数据。随机密文不同不代表逻辑配置不同：比较及合并必须在认证解密后进行。

## 3. 解密后 Configuration

内层没有独立 `schemaVersion` 字段；外层 `version=1` 不能解释成支持任意未来字段。正常写入使用下列完整结构，数组为空时写 `[]`，不写 `null`。

| 字段 | 类型 | v1 写入与旧数据读取 |
| --- | --- | --- |
| `profiles` | SessionProfile[] | 必填；可以为空 |
| `proxies` | ProxyProfile[] | 写完整数组；旧数据缺失时为空 |
| `preferences` | Preferences 对象 | 必填；已知可选偏好缺失按第 5 节 |
| `directories` | string[] | 写数组；缺失为空，包含需要保留的空目录 |
| `quickCommands` | QuickCommand[] | 写数组；缺失为空 |
| `highlightSets` | HighlightSet[] | 写数组；旧数据缺失时使用标准集，正常写入不得依赖随机生成默认规则 ID |
| `sessionLinks` | SessionLinks 对象 | 写完整对象；旧数据缺失时空列表且可见 |
| `sessionDefaults` | SessionDefaults 对象 | 写完整对象；旧数据缺失使用第 4 节默认值 |
| `masterPasswordVerifier` | EncryptedPassword | 共享数据必需，不能是本机密钥保护 |
| `ftpProfiles` | 历史 FTPProfile[] | 仅为旧数据迁移；正常写入省略或 `[]`，FTP 归入 `profiles` |

UUID 使用带连字符的 36 字符形式，身份比较不区分十六进制大小写；推荐写大写。不得改变普通名称、主机文本、目录文本、密码、密文或用户数据的大小写。

会话、代理、隧道、快捷引用、命令及规则的 ID 必须保持稳定。不得因序列化、同步或读取而重新生成已有 ID。引用必须能解析，重复 ID、循环代理链、悬空代理引用必须拒绝，不能静默改成直连。

目录使用 `/` 分隔，根目录为 `""`，不使用 Windows 反斜杠。`Links` 为快捷引用保留根目录。会话目录和快捷引用目录要按各自规则规范化；不将平台绝对路径当作会话分组路径。

## 4. 会话与关联对象

### 4.1 SessionProfile

| 字段 | 类型 | 写入规则 / 兼容默认值 |
| --- | --- | --- |
| `id` | UUID string | 必填，稳定身份 |
| `name`, `group` | string | 必填；名称非空，目录根为 `""` |
| `kind` | string | 必填：`ssh`、`sftp`、`ftp`、`local` |
| `host`, `username` | string | 必填；SSH 用户可空以使用解析结果，FTP 用户非空 |
| `port` | integer | 必填；网络会话 1–65535；新 SSH/SFTP 默认 22，FTP 21 |
| `identityFile`, `jumpHost` | string | 必填；未设置为 `""` |
| `encryptedPassword` | EncryptedPassword | 无保存密码则省略；不得用占位密码代替 |
| `tunnels` | TunnelRule[] | 写数组；旧数据缺失为 `[]` |
| `proxy` | ProxySettings 或引用标记 | 写稳定对象；见下文 |
| `proxyID` | UUID string | 引用共享代理时写；否则省略 |
| `keepAlive` | KeepAliveSettings | 写完整对象；旧数据缺失使用默认值 |
| `legacySSH` | boolean | 默认 false |
| `titleMode` | string | `shellIntegration`（默认）、`activeProbe`、`passive` |
| `quickConnect` | boolean | 默认 true |
| `initialDirectory` | string | SSH/SFTP 默认 `.`，FTP 默认 `/`；非本地会话不能为空 |

历史 `activeHostProbe=true` 可映射为 `titleMode=activeProbe`，否则为 `shellIntegration`。已有 `titleMode` 优先。会话的密码身份绑定规则见第 6 节，不能将密码信封原样挪到新的身份。

共享代理：`proxyID` 指向 `proxies[].settings.id`；`proxy` 必须写成带稳定 ID 的 `{ "id": "UUID", "kind": "shared-reference" }`。这个鉴别值让不支持共享代理的旧客户端拒绝读取，防止忽略代理后直连。共享代理只用于 SSH/SFTP。

未引用共享代理时，`proxy` 使用 ProxySettings。即使未启用代理，其 ID 也必须稳定。旧内嵌代理迁移为共享代理时要保留凭据及引用语义。

### 4.2 ProxyProfile / ProxySettings

ProxyProfile：`name: string`、`settings: ProxySettings`、可省略的 `upstreamID: UUID string`。代理身份是 `settings.id`，不是额外的顶层 ID。代理链最多 8 层，不允许环路。

ProxySettings：`id: UUID`、`kind: none|socks4|socks4a|socks5|http|jump`、`host: string`、`port: integer`、`username: string`、`identityFile: string`、`sshAuthentication: automatic|password|privateKey`，以及可省略的 `encryptedPassword`。默认 `kind=none`、`port=1080`、空主机/用户/路径、`sshAuthentication=automatic`。写出认证方式时不得用空字符串代替 `automatic`。共享代理目录内的条目不得使用 `none`。

代理密码的身份载体使用代理 `settings.id/host/port/username`，不是引用它的会话身份。更改代理身份须按第 6 节重新绑定。

### 4.3 TunnelRule / KeepAliveSettings

TunnelRule 完整字段：`id: UUID`、`enabled: bool=true`、`kind: local|remote|dynamic`、`bindHost: string="127.0.0.1"`、`listenPort: int=8080`、`destinationHost: string="localhost"`、`destinationPort: int=80`、`note: string=""`。非 dynamic 的目标端口必须有效，同一会话不能配置重复启用的监听地址/端口。

KeepAliveSettings 完整字段：`enabled=true`、`interval=30`、`maxMissed=3`、`tcp=true`、`idleEnabled=false`、`idleInterval=3600`、`idleText` 默认为反斜杠与字母 n 两个字符。时间单位为秒；idleText 的 JSON 字符串写法为 `"\\n"`，不是实际换行。

SessionDefaults 完整字段：`sshPort=22`、`sshUsername=""`、`identityFile=""`、`legacySSH=false`、`sshDirectory="."`、`ftpPort=21`、`ftpUsername="anonymous"`、`ftpDirectory="/"`、`quickConnect=true`、`keepAlive` 为上述完整对象。它只决定新建会话初值，不覆盖已有会话。

### 4.4 快捷引用、命令、突出显示

- SessionLinks：`entries: SessionLink[]=[]`、`folders: string[]=[]`、`visible: bool=true`、`rootOrder: string[]=[]`。SessionLink 字段为 `id`、`profileID`（均 UUID）、`name`、`folder`（相对 Links 的路径，根为 `""`）。引用不复制会话密码。`rootOrder` 使用 `link:UUID`、`session:UUID`、`folder:路径`，保持有效项顺序。
- QuickCommand：`id: UUID`、`name: string`、`group: string`、`text: string`、`appendReturn: bool`。命令文本非空，不含 NUL，最长 1 MiB；不得修改命令大小写或换行内容。
- HighlightSet：`id: UUID`、`name: string`、`rules: HighlightRule[]`。标准集 ID 为 `F6C9AE44-0C05-450B-AB54-28F8AC88B314`。
- HighlightRule：`id: UUID`、`pattern: string`、`regex: bool`、`caseSensitive: bool`、`color: #RRGGBB string`、`enabled: bool`。pattern 非空且最多 512 UTF-8 字节。保存规则身份，不能每次同步重新生成。

## 5. Preferences 字段登记

写入必须使用下列类型；不要把 UI 展示值直接写成协议值。兼容默认值用于旧文档缺失字段，正常生产者应写出已实现的完整设置。

| 字段 | JSON 类型 | 共同值域 / 兼容默认 |
| --- | --- | --- |
| `fontName` | string | 空表示默认字体；字体缺失时本机回退 |
| `fontSize` | number | 默认 13；共同配置范围 10–26 |
| `scrollback` | integer | 默认 3000；500–100000 行 |
| `interfaceTheme` | string | `system`（默认）、`light`、`dark` |
| `colorSchemeID` | string | 默认 `oshell-dark`，指向有效内置/自定义配色 |
| `customColorSchemes` | array | 默认 `[]`；见下文 |
| `darkTheme` | boolean | 旧兼容标记，由终端背景推导，不能用 UI 主题代替 |
| `metal`, `autoZmodem`, `preferColorEmoji` | boolean | 默认 true；渲染能力按平台使用 |
| `ambiguousCharactersAreWide` | boolean | 默认 false；依赖终端库支持 |
| `copyOnSelect`, `rightClickPaste`, `confirmMultilinePaste` | boolean | 默认 true |
| `copyTrimLeadingWhitespace`, `copyTrimTrailingWhitespace` | boolean | 默认 false |
| `updateRepository` | string | 默认 `""`；安装仍须校验应用身份，不能因共享设置而跨产品安装 |
| `automaticUpdateChecks`, `masterWarningAcknowledged` | boolean | 默认 false |
| `quickSendBarVisible` | boolean | 默认 true |
| `quickSendScope` | integer | 0 当前、1 标签、2 全部、3 可见、4 所选、5 当前组、6 所选组；默认 0 |
| `keyboardShortcuts` | object | `{ "overrides": {} }` |
| `highlightSetID` | UUID string 或 null | 缺失使用标准集，显式 null 表示不选择 |
| `terminalClockEnabled` | boolean | 默认 false |
| `terminalClockPosition` | string | 九宫格值，默认 `topRight` |
| `terminalClockFontName`, `terminalClockColor` | string | 默认空，分别使用默认字体/终端文字颜色；显式颜色为 `#RRGGBB` |
| `terminalClockFontSize` | number | 默认 12，8–96 |
| `terminalClockOpacity` | number | 默认 0.5，0–1 |

九宫格值：`topLeft`、`topCenter`、`topRight`、`centerLeft`、`center`、`centerRight`、`bottomLeft`、`bottomCenter`、`bottomRight`。

配色字段：`id`、`name`、`background`、`foreground`、`cursor`、`cursorText`、`selection`、`selectionText` 均字符串，颜色为 `#RRGGBB`；`ansi` 为恰好 16 个颜色的数组。`darkTheme` 计算为 `R*0.2126 + G*0.7152 + B*0.0722 < 128`，RGB 分量为 0–255。Desktop 的 `autoCursor` 等未登记扩展不能作为跨端无损保留的保证，见第 9 节。

快捷键：`overrides` 以动作标识为键，每项为 `{ "shortcut": { "keyCode": integer, "modifiers": integer } }`；无 shortcut / null 表示禁用该覆盖项对应动作。keyCode 使用 OShell 登记的 macOS 键位编号，不能直接写 Windows VK 或 Linux keycode；修饰掩码 command=1、option=2、control=4、shift=8，组合为按位或。各平台进行本地按键映射。动作及键位完整登记见原版 `KeyboardShortcuts.swift` 与 desktop `shortcuts-presets.json`，新增共同动作须同时登记和测试。

旧 Desktop 的 `quickSendScope` 字符串只作为迁移输入，正常共享输出必须为上述整数；旧字符串与等价整数不应产生用户冲突。

## 6. 保存密码及主密码校验

EncryptedPassword 字段：`version:1`、`algorithm:"AES-256-GCM"`、`kdf:"PBKDF2-HMAC-SHA256"`、`iterations`、`salt`、`ciphertext`、`identity:{host:string,user:string,port:integer}`。共享数据不得包含非空 `localKeyID`。

盐、派生算法、轮数、密钥长度及密文排列同第 2 节；每条密码独立派生和加密。密码明文为 UTF-8，非空，最多 4096 字节，不含 NUL、CR、LF；密码信封密文解码后长度范围为 28–8192 字节。AAD 不同：

```text
UTF8("OShell-password-v1" + NUL
  + UPPERCASE_UUID(profile.id) + NUL
  + profile.host + NUL
  + DECIMAL(profile.port) + NUL
  + profile.username + NUL
  + envelope.identity.host + NUL
  + envelope.identity.user + NUL
  + DECIMAL(envelope.identity.port))
```

NUL 是单字节 `0x00`，最后无分隔符。端口是十进制整数文本，无前导零。原始主机/用户与解析后的连接身份分别绑定，不可互换；AAD 不包含 name、group 或 kind。实现可以在本机凭据库额外绑定 kind，但不能改变共享 AAD。

同一主密码、原始身份及已解析身份都未改变时，应复用有效旧密码信封，避免随机重加密产生假修改。显式修改目标或复制为新 UUID 时，必须先验证并解密旧密码，再按新身份重新加密；自动 SSH 配置解析变化不等同于用户授权修改目标。

`masterPasswordVerifier` 使用同一密码信封算法，明文恰为 `OShell master password verifier v1`，虚拟会话身份如下：

- id：`F7FCF275-EB85-488F-AF21-A0424101259A`
- host / identity.host：`oshell-vault`
- username / identity.user：`vault`
- port / identity.port：22

只有外层认证成功、校验器通过且内部定义有效后才可应用共享数据。主密码变更不是普通字段合并：其他节点需重新解锁、读取新版本并重新建立可用基准。

## 7. 传输及多节点合并

WebDAV 必须使用 HTTPS，账号密码独立传输；只允许测试工具对 loopback 使用 HTTP。客户端不跟随重定向携带凭据，要求最终目录地址。

1. GET 得到完整加密对象与强 ETag。404 表示不存在；已有基准后对象消失应报错，不自动重建覆盖。
2. 首次创建 PUT 携带 `If-None-Match: *`；更新 PUT 携带读取时的 `If-Match: ETag`。
3. 412 表示版本冲突：重新读取并合并，禁止退化为无条件 PUT。弱 ETag、缺失 ETag、认证失败和错误密码不得覆盖。
4. 首次接入已有共享数据，需明确选择采用共享数据或合并本机会话；替换前备份。不能以最新本机时间自动决定胜者。
5. 基于共同基准 B、本机 L、远端 R 三方比较：L=R 或 R=B 取 L；L=B 取 R；其余产生冲突。
6. `profiles` 按 UUID、整条会话合并，包括凭据；同一会话改不同字段仍需确认。删除也是修改。代理及其余顶层区块整体处理，避免拼接出失效代理链。
7. 冲突解决后提交仍须验证本机修订和远端 ETag；仅下载也需防止应用确认期间已过时的远端快照。
8. 无逻辑变更不 PUT、不随机重加密，不把 UUID 大小写或规范化默认值视为业务修改。

同步目录对完整加密文件计算 SHA-256，以带双引号的小写十六进制摘要作为比较版本；使用本机锁、原子替换，以及 macOS 文件协调。异步云盘和任意 SMB/NFS 不自动具备跨机器原子条件写入保证。多节点同时编辑优先使用正确支持条件写入的 WebDAV；云盘冲突副本须保留并处理。

## 8. 限制、规范化和验证边界

字段类型、引用完整性和凭据身份必须先验证。禁用未知代理/忽略失效引用后直连不可作为“兼容”。数值不得使用字符串替代，也不得使用 NaN 或 Infinity。

缺失/null 与空数组在旧解码器中可能都能读取，但正常写入必须按登记类型规范化。允许省略的对象/密码引用可省略；`highlightSetID=null` 有明确业务含义，不能改成默认集。排序和格式化 JSON 不改变语义，但不能任意重排数组顺序。

各应用可以有更严格的输入限制或平台支持范围。能解码不代表当前平台能运行对应功能，也不代表所有可能设置都已无损互通。共享私钥路径、字体、快捷键和渲染选项不得声称在所有系统有相同执行效果。

## 9. 历史边界与当前规则

旧 v1 客户端可能忽略未知字段，不能保证未登记扩展无损往返。本文中的字段表仅用于解释历史数据；当前字段、默认值、严格校验、版本迁移和验收均以 [v2 规范](configuration-protocol-v2.md) 及其可执行合同为准。

v2 已实现，不支持未知字段原样保留、能力协商或自动降级。旧客户端应升级后再参与同一同步目标。

实现入口：`shared_document.go`、`shared_protocol.go`、`mac_cipher.go`、`portable_archive.go`、`shared_merge.go`、`shared_sync.go` 和 `sync_transport.go`。跨端合成样本位于 `internal/core/testdata/`，Swift 生成器为 `scripts/interop/main.swift`。
