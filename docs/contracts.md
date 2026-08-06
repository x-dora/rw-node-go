# Contract 说明

兼容性来源以官方 [`remnawave/node`](https://github.com/remnawave/node) [`3.0.0`](https://github.com/remnawave/node/tree/3.0.0) 面向 Panel 的 contract 和实际实现为准。Go 侧公开类型放在 [`internal/contracts`](../internal/contracts)，HTTP route 注册放在 [`internal/httpapi/router.go`](../internal/httpapi/router.go)。

`tmp/remnawave-node` 用作官方仓库参考，当前对齐目标是 tag [`3.0.0`](https://github.com/remnawave/node/tree/3.0.0)（commit `46fc5d2d736ff60f6c6a9a56e2661acb95d3f559`）。必要时应参考其 contract、controller、service、Xray 配置生成和错误处理实现。

本仓库的 `nodeVersion` 固定为 `3.0.0`，与 [`VERSION`](../VERSION) 无关；`VERSION` 只表示 `rw-node-go` 自己的发布版本。

当前已从官方 3.0.0 contract 手工整理小型 golden manifest：

```text
testdata/contracts/official-3.0.0/panel-api.json
```

该 manifest 覆盖官方 Panel-facing route、代表性请求和响应 envelope，用于 Go contract struct 的 strict decode、响应 JSON 形状和路由注册测试。它不包含 internal REST API，也不复制官方 TypeScript contract 包。

## 状态总览

| 分组 | 状态 | 说明 |
| --- | --- | --- |
| Xray | `partial` | 已接入内嵌 `xray-core` instance 生命周期。 |
| Handler | `partial` | 已接入内嵌 inbound feature 和 best-effort 清理。 |
| Stats | `partial` | 已接入基础 stats、流量统计和 OnlineMap 降级。 |
| Plugin | `adapter stub` | 只保留 Panel-facing contract adapter。 |

## 边界和覆盖原则

- 保持公开路由路径、HTTP method、JSON 字段名和 response envelope 稳定。
- 已知官方 contract 中的 Panel-facing route 至少要注册，避免 Panel 调用时得到 404。
- Panel-facing contract 只覆盖主 API 路由；Internal REST API 不属于官方 Panel contract。
- Adapter stub 只表示为了兼容 Panel 调用而保留路由和响应形状，不表示有真实 plugin、torrent blocker 或 nftables side effects。
- contract 类型无法解释的行为，要继续查看官方仓库中对应 controller/service 的实现。
- 当官方 `libs/contract` schema 与 runtime model 不一致时，Panel-facing 响应优先跟随官方 runtime model；例如 `get-inbound-users` 返回用户的 `username`、`level`、`protocol`，不输出 schema 中 optional 的 `email`。
- 未实现的能力必须返回明确的兼容占位数据，不能伪装成真实 Xray、stats、plugin、nftables 或 conntrack 行为。
- 业务失败优先保持官方风格：Xray start/handler mutation 多数通过 `response.error`、`response.success` 或对应业务字段表达失败；官方 stats 查询失败使用 `{timestamp,path,message,errorCode}` 和对应 HTTP status。

## 当前路由状态

本表只描述 Panel-facing contract 层的注册、响应形状和真实/partial/stub 边界，不作为完整功能路线图。详细进度见 [docs/roadmap.md](roadmap.md)。

状态说明：`done` 已有真实或基础实现，`partial` 部分接入，`adapter stub` 只做接口适配。

| 分组 | 路由 | 状态 | 说明 |
| --- | --- | --- | --- |
| Xray | `POST /node/xray/start` | partial | 已接入内嵌 `xray-core` instance 启动和 config stats/policy 注入；真实 Panel + Xray 验收仍未完成。 |
| Xray | `GET /node/xray/stop` | partial | 已接入内嵌 instance 关闭。 |
| Xray | `GET /node/xray/healthcheck` | partial | 当前按官方缓存在线状态和缓存版本返回。 |
| Handler | `/node/handler/add-user`, `/node/handler/add-users`, `/node/handler/remove-user`, `/node/handler/remove-users`, `/node/handler/get-inbound-users`, `/node/handler/get-inbound-users-count` | partial | 已接入内嵌 Xray inbound feature 和内存 inbound/user hash/protocol 状态；`get-inbound-users` 按官方 runtime model 返回 `username`、`level`、`protocol`；真实 Panel + Xray 验收仍未完成。 |
| Handler | `/node/handler/drop-users-connections`, `/node/handler/drop-ips` | partial | 已通过 conntrack best-effort 清理匹配 IP 的连接；无权限或无系统能力时返回成功 no-op，不操作 nftables。 |
| Stats | `/node/stats/*` | partial | system stats 已按官方 3.0.0 响应形状返回宿主机 CPU、memory、uptime、load、network interface 和 Xray sys stats；默认网卡按 `/proc/net/route` 中 metric 最小的默认路由选择；users、inbound、outbound、combined 和 online status/IP 已接入内嵌 stats feature；`get-users-stats` 按官方行为过滤上下行均为 0 的用户；OnlineMap 依赖 Linux `CAP_NET_ADMIN` 启用 `statsUserOnline`，不可用或读取失败时降级为 `false` 或空列表；真实 Panel + Xray 验收仍未完成。 |
| Vision | `/vision/block-ip`, `/vision/unblock-ip` | removed | 官方 3.0.0 已移除 Vision public contract；Go 侧同步不注册这些 route，访问返回 404。 |
| Plugin | `/node/plugin/sync`, `/node/plugin/torrent-blocker/collect`, `/node/plugin/nftables/*` | adapter stub | routes 保持 Panel-facing contract adapter；feature intentionally unsupported，不保存插件状态、不注入 Xray 配置、不接收 webhook、不触发 Xray restart、不执行 nftables、不产生 torrent reports；官方 3.0.0 新增的 pre-start plugin 同样不实现。 |

Internal REST API 不是 Panel-facing contract，边界说明见 [docs/architecture.md](architecture.md)。

## Golden 测试

Golden fixture 目录：

```text
testdata/contracts/official-3.0.0
```

对应仓库路径见 [`testdata/contracts/official-3.0.0`](../testdata/contracts/official-3.0.0)。

当前 contract 测试已覆盖：

- HTTP method 和 path。
- JSON 字段名和可选字段。
- response envelope。
- `null`、空数组、空对象行为，包括 `plugin.sync` 的 `plugin: null` adapter-only 响应。
- 代表性错误或降级响应形状，包括 stats 和 handler 查询错误的官方 error envelope。
- 非空响应字段回归，包括 `get-inbound-users` 的 `username`、`level`、`protocol` runtime 字段。

Go 侧请求解析保持兼容解析，不复刻官方 zod 全量强校验；只保留当前业务安全需要的最小校验，例如 drop connections/drop IPs 的空数组返回 `success:false`。Plugin nftables route 仍是 adapter stub，不因 IP 格式执行真实校验或系统操作。

已知 deliberate divergence：

- 官方 2.7.0 历史实现中 `getAllOutboundsStats` 的 catch 分支实际复用了 inbounds 错误常量；Go 侧按 contract 语义返回 outbounds 错误码 `A016`。
- 官方 `DISABLE_HASHED_SET_CHECK` env 在 Go 侧不存在。官方用它跳过 start 的 hash 短路强制重启 Xray；Go 侧只能由 Panel 请求里的 `internals.forceRestart` 绕过，`.env.example` 也不提供这个键，避免留下不生效的假开关。
- Internal REST API 固定监听 `127.0.0.1` TCP，不跟随官方 3.0.0 的 abstract unix socket（`INTERNAL_SOCKET_PATH` 以 `\0` 前缀监听、s6 `init-env.sh` 随机生成 socket path 和 token）。Go 侧唯一运行模式是内嵌 core，不需要把 config 通过 socket 交给外部 Xray 进程。边界见 [docs/architecture.md](architecture.md)。
- 官方 3.0.0 新增的 pre-start plugin（`PreStartService` 按 glob 清理 stale unix socket）不实现：既受 plugin adapter-only 约束，内嵌 core 也不产生需要清理的 socket 文件。

已核对的 non-divergence：官方 contract 仍声明 `XRAY_ROUTES.STATUS='status'`，但 3.0.0 的 `xray.controller.ts` 依旧只实现 start/stop/healthcheck；Go 侧同样不注册 `/node/xray/status`，返回 404 与官方一致。

## 官方 Contract Drift 检查

本仓库保存官方 [`remnawave/node`](https://github.com/remnawave/node) 3.0.0 的 contract hash baseline：

```text
testdata/contracts/official-3.0.0/upstream-contract.sha256.json
```

对应仓库文件见 [`testdata/contracts/official-3.0.0/upstream-contract.sha256.json`](../testdata/contracts/official-3.0.0/upstream-contract.sha256.json)。

该 baseline 只保存官方 [`libs/contract`](https://github.com/remnawave/node/tree/3.0.0/libs/contract) 中 Panel-facing contract 相关 TypeScript 文件的路径和 SHA-256，不保存官方源码正文。检查范围包括：

- `libs/contract/api`
- `libs/contract/commands`
- `libs/contract/constants/errors`
- `libs/contract/constants/xray`
- `libs/contract/models`

本地检查当前基线。默认检查官方 `dev` 分支：

```sh
mise run contract-diff
```

网络不可用但本地已有官方 checkout 时，显式指定本地源码目录：

```sh
CONTRACT_SOURCE_DIR=tmp/remnawave-node mise run contract-diff
```

临时检查其他官方 tag 或 branch ref 时指定 `CONTRACT_TAG`；这只改变本次检查目标，不会自动更新 baseline：

```sh
CONTRACT_TAG=main mise run contract-diff
```

如果检查失败，先查看新增、删除或 hash 变化的文件列表，再对照 [官方仓库](https://github.com/remnawave/node) 更新 Go contract、route 注册和 golden fixture。

3.0.0 相对 2.8.0 的 contract 变化全部是 zod 3→4 语法迁移（`z.uuid()`、`z.enum()`、`z.ipv4()`/`z.ipv6()`、`z.record(z.string(), …)`、`z.iso.datetime()`、`z.int()`），11 个文件的 hash 变化、0 新增 0 删除，`libs/contract/api` 与 `constants/*` 零变更，wire format 和 route 表未变，所以 Go contract struct 无需改动。

跟随官方 `nodeVersion` 时，除了 contract hash 还要复查 Panel 侧的 semver 判断点（当前是 `start-node.processor.ts` 和 `start-all-nodes-by-profile.processor.ts` 两处 `semver.lt(nodeVersion, '2.7.0')`），确认没有引入按版本区间启用的新行为。

不要把官方 TypeScript contract 包整体复制进仓库。只固化必要的小型请求/响应 JSON fixture，并在 contract 变化时对照官方实现更新。
