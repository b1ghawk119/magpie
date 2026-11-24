# Windows 和 macOS 下游发行版

在仓库 Releases 中选择带 `+hawk.N` 的版本。Windows 使用 `magpie-windows-amd64.exe` 或 `magpie-windows-arm64.exe`。退出托盘中的 Magpie，再从「下载」打开对应程序。已经装有相同或更新官方版本时，退出后将下载件复制到原安装目录并命名为 `magpie.exe`。Mac 选 `magpie-darwin-arm64.dmg`（Apple Silicon）或 `magpie-darwin-amd64.dmg`（Intel），退出 Magpie，将应用拖到「应用程序」。本 fork 首次发行的 Mac 应用采用 ad-hoc 代码签名。首次打开如果遇到 Gatekeeper 提示，可在系统设置的「隐私与安全性」核对并允许这份应用。

在设置中使用默认更新来源，选择「每个版本」并启用更新检查，即可接收已发布的预发行版和正式版。稳定、每日按本仓库成功发布的时间每周、每日选版，更新文件来自本仓库。首次迁移建议保留现有可执行文件或 `.app` 的副本，方便换回。

## 自动压缩接续

切换到一个使用 Responses 的 provider 后，Magpie 可以在同一主对话里转发旧的原生压缩项。目标端明确拒绝密文时，如果原写入者还可访问，网关可以向它请求已有压缩项的文字摘要，将这段摘要和后续消息一起交给目标端。当两端兼容，正常路径只需一次调用。导出会产生额外的模型调用。对含多份原生密文、无法访问原写入者或摘要不完整的请求，导出失败会保留上游错误。旁路任务和 Codex 自身的登录路由沿用上游规则。

对从别处导入、找不到写入者记录的旧主对话，也先让目标 Responses 直接读取；若目标拒绝压缩项，原错误会回到客户端。这样用户可以换到能读它的线路继续。一次成功接续不能代表两家 provider 的所有账号都互通。

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
