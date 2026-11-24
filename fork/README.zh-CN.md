# Windows 和 macOS 下游发行版

在仓库 Releases 中选择 `+hawk.1` 版本。Windows 使用 `magpie-windows-amd64.exe` 或 `magpie-windows-arm64.exe`。退出托盘中的 Magpie，再从「下载」打开对应程序。已经装有相同或更新官方版本时，退出后将下载件复制到原安装目录并命名为 `magpie.exe`。Mac 选 `magpie-darwin-arm64.dmg`（Apple Silicon）或 `magpie-darwin-amd64.dmg`（Intel），退出 Magpie，将应用拖到「应用程序」。本 fork 首次发行的 Mac 应用采用 ad-hoc 代码签名。首次打开如果遇到 Gatekeeper 提示，可在系统设置的「隐私与安全性」核对并允许这份应用。

设置中选择「每个版本」并启用更新检查，即可接收已发布的预发行版和正式版。稳定、每日按本仓库成功发布的时间每周、每日选版，更新文件来自本仓库。首次迁移建议保留现有可执行文件或 `.app` 的副本，方便换回。

## 自动压缩接续

切换到一个使用 Responses 的 provider 后，Magpie 可以在同一主对话里转发旧的原生压缩项。目标端明确拒绝密文时，如果原写入者还可访问，网关可以向它请求已有压缩项的文字摘要，将这段摘要和后续消息一起交给目标端。当两端兼容，正常路径只需一次调用。导出会产生额外的模型调用。对含多份原生密文、无法访问原写入者或摘要不完整的请求，导出失败会保留上游错误。旁路任务和 Codex 自身的登录路由沿用上游规则。

## 同步与恢复

`main` 维护 fork 改动。每个上游版本对应一个独立合并后的源码提交，发布 tag 指向该提交。`fork-sync.yml` 每 15 分钟检查上游已经发布的 Releases。构建任务在指定上游 tag 上通过 `git apply --3way` 合入 fork 相对基线的改动。冲突或验证失败将停止该版本的发布，上一份已发布安装包继续可用。维护者在 `main` 解决冲突后，可以手动运行 `fork-release.yml` 并传入对应的上游 tag。

Windows 自更新需要 `.exe`，Mac 自更新需要 `.zip`，首次安装用 `.dmg`。CI 生成 `SHA256SUMS`，用本仓库的 Ed25519 私钥签名，并生成 GitHub 构建来源证明。更新通道的指针在 `update-feed` 分支。仓库设置中的 `FORK_UPDATE_SIGNING_KEY` 是后续版本的签名凭证，客户端内置公钥在 `internal/update/signed.go`。
