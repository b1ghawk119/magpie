# Windows 和 macOS 下游发行版

在仓库 Releases 中选择带 `+hawk.N` 的版本。Windows 使用 `magpie-windows-amd64.exe` 或 `magpie-windows-arm64.exe`。退出托盘中的 Magpie，再从「下载」打开对应程序。已经装有相同或更新官方版本时，退出后将下载件复制到原安装目录并命名为 `magpie.exe`。Mac 选 `magpie-darwin-arm64.dmg`（Apple Silicon）或 `magpie-darwin-amd64.dmg`（Intel），退出 Magpie，将应用拖到「应用程序」。本 fork 首次发行的 Mac 应用采用 ad-hoc 代码签名。首次打开如果遇到 Gatekeeper 提示，可在系统设置的「隐私与安全性」核对并允许这份应用。

在设置中使用默认更新来源，选择「每个版本」并启用更新检查，即可接收已发布的预发行版和正式版。稳定、每日按本仓库成功发布的时间每周、每日选版，更新文件来自本仓库。首次迁移建议保留现有可执行文件或 `.app` 的副本，方便换回。

## 自动压缩接续

从 `v0.1.4738+hawk.2` 起，在同一个主对话里切换 Responses provider 会先把原压缩项交给新线路。能直接读取时只调用新线路。新线路明确拒绝密文时，Magpie 会先排除独立的推理项，再尝试以下恢复方式。

1. 新版本在原生压缩成功时从压缩前可读消息和工具结果截取最多 24 KiB 文字，和写入者记录一起加密保存到本机。每项摘录最多 4 KiB，总体保留开头和最近部分。最近 128 个检查点保留摘录，最长 30 天。接续优先使用匹配的摘录，原账号欠费、停用或离线均可。
2. 对同机旧对话，Magpie 还可从当前 Codex 主目录的活动或归档 JSONL 记录中读取摘录，要求本机直连、线程与压缩密文同时匹配，只读取截至该次压缩的可见内容。
3. 找不到本地材料时，已知的原写入者可提供可读摘要。写入者也失败时，普通对话仍会把当前请求中可见的消息交给目标，并要求它说明早期上下文缺失、在需要时向你补问。只有密文、连写入者身份也未知且无匹配本地记录时，保留目标端错误。恢复单份压缩项，旁路任务及 Codex 隐式登录路由沿用上游规则。

`/responses/compact`、`/responses` 加 `compaction_trigger`、`/responses` 加摘要提示是不同的生成方式。前两种可以产生原生密文，下一轮按实际收到的压缩项处理；第三种如果返回可读摘要，下一轮直接沿用它。转发 `/responses/compact` 时保留上游返回的 `output`，在账号适配层用 trigger 模拟该端点时，Magpie 会保留输入中的 user 消息，再追加原生压缩项。普通 trigger 的流式输出由上游决定。

### 参考实现中的取舍

| 项目与审阅版本 | 与此 fork 有关的实现 |
|---|---|
| [CC-Switch](https://github.com/farion1231/cc-switch/blob/f19a9d4/src-tauri/src/proxy/opaque_state_rectifier.rs) | 转发原生 Responses 时有按拒绝结果整流的路径。它自己封装的摘要可以还原成文字；第三方密文在整流时可能换成占位说明。本 fork 借用先试再处理的顺序，通过写入者取得有内容的摘要。 |
| [gpt-load](https://github.com/tbphp/gpt-load/blob/cfc6a6d/internal/codexrecovery/sanitize.go) | 按错误类型与被拒密文清理 reasoning 等可拆状态，保留无法重建的 compaction。这里同样区分推理项和唯一的历史摘要。 |
| [sub2api](https://github.com/Wei-Shaw/sub2api/blob/00c51f0/backend/internal/service/account.go) | 在账号层记录 compact 能力，支持自动判定、强制开关和独立模型映射。账号能生成压缩，与它能读取某份外来密文是两件事。 |
| [new-api](https://github.com/QuantumNous/new-api/blob/cb074d3/relay/responses_handler.go) | 有专用 compact 分支，按端点允许的字段构造请求，再交给渠道适配器。 |
| [FanAPI](https://github.com/Fansky123/FanAPI/blob/9f6d649/internal/handler/llm_upstream.go) | 在 Responses 渠道上把压缩请求送往独立的 `/responses/compact` 地址。 |

## 同步与恢复

`main` 维护 fork 改动。每个上游版本对应一个独立合并后的源码提交，发布 tag 指向该提交。`fork-sync.yml` 每 15 分钟检查上游已经发布的 Releases。构建任务在指定上游 tag 上通过 `git apply --3way` 合入 fork 相对基线的改动。冲突或验证失败将停止该版本的发布，上一份已发布安装包继续可用。维护者在 `main` 解决冲突后，可以手动运行 `fork-release.yml` 并传入对应的上游 tag。

Windows 自更新需要 `.exe`，Mac 自更新需要 `.zip`，首次安装用 `.dmg`。CI 生成 `SHA256SUMS`，用本仓库的 Ed25519 私钥签名，并生成 GitHub 构建来源证明。更新通道的指针在 `update-feed` 分支。仓库设置中的 `FORK_UPDATE_SIGNING_KEY` 是后续版本的签名凭证，客户端内置公钥在 `internal/update/signed.go`。
