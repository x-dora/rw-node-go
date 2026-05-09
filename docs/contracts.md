# Contract 说明

兼容性来源以官方 `remnawave/node` 2.7.x 面向 Panel 的 contract 和实际实现为准。Go 侧公开类型放在 `internal/contracts`，HTTP route 注册放在 `internal/httpapi/router.go`。

`tmp/remnawave-node` 是官方 2.7.0 仓库，必要时应参考其 contract、controller、service、Xray 配置生成和错误处理实现。`tmp/remnawave-node-go` 只作为内嵌 `xray-core` 结构参考。

本仓库的 `nodeVersion` 仍固定为 `2.7.0`，与 `VERSION` 无关；`VERSION` 只表示 `rw-node-go` 自己的发布版本。

当前已从官方 2.7.0 contract 手工整理小型 golden manifest：

```text
testdata/contracts/official-2.7.0/panel-api.json
```

该 manifest 覆盖官方 Panel-facing route、代表性请求和响应 envelope，用于 Go contract struct 的 strict decode、响应 JSON 形状和路由注册测试。它不包含 internal REST API，也不复制官方 TypeScript contract 包。

## 状态总览

| 分组 | 状态 | 说明 |
| --- | --- | --- |
| Xray | `partial` | 已接入内嵌 `xray-core` instance 生命周期。 |
| Handler | `partial` | 已接入内嵌 inbound feature 和 best-effort 清理。 |
| Stats | `partial` | 已接入基础 stats、流量统计和 OnlineMap 降级。 |
| Vision | `partial` | 已接入内嵌 routing feature。 |
| Plugin | `adapter stub` | 只保留 Panel-facing contract adapter。 |

## 覆盖原则

- 保持公开路由路径、HTTP method、JSON 字段名和 response envelope 稳定。
- 已知官方 contract 中的 Panel-facing route 至少要注册，避免 Panel 调用时得到 404。
- contract 类型无法解释的行为，要继续查看官方仓库中对应 controller/service 的实现。
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
| Handler | `/node/handler/add-user`, `/node/handler/add-users`, `/node/handler/remove-user`, `/node/handler/remove-users`, `/node/handler/get-inbound-users`, `/node/handler/get-inbound-users-count` | partial | 已接入内嵌 Xray inbound feature 和内存 inbound/user hash 状态；真实 Panel + Xray 验收仍未完成。 |
| Handler | `/node/handler/drop-users-connections`, `/node/handler/drop-ips` | partial | 已通过 conntrack best-effort 清理匹配 IP 的连接；无权限或无系统能力时返回成功 no-op，不操作 nftables。 |
| Stats | `/node/stats/*` | partial | system stats 已按官方 2.7.0 响应形状返回宿主机 CPU、memory、uptime、load、network interface 和 Xray sys stats；users、inbound、outbound、combined 和 online status/IP 已接入内嵌 stats feature；OnlineMap 不可用或读取失败时降级为 `false` 或空列表；真实 Panel + Xray 验收仍未完成。 |
| Vision | `/vision/block-ip`, `/vision/unblock-ip` | partial | 官方主 API unprefixed route；设置 `SECRET_KEY` 后保留 mTLS、豁免 JWT；已通过内嵌 routing feature 操作 source IP dynamic rule，真实验收仍未完成。 |
| Plugin | `/node/plugin/sync`, `/node/plugin/torrent-blocker/collect`, `/node/plugin/nftables/*` | adapter stub | routes 保持 Panel-facing contract adapter；feature intentionally unsupported，不保存插件状态、不注入 Xray 配置、不接收 webhook、不触发 Xray restart、不执行 nftables、不产生 torrent reports。 |

Internal REST API 不是 Panel-facing contract，边界说明见 [docs/architecture.md](architecture.md)。

## Golden 测试

Golden fixture 目录：

```text
testdata/contracts/official-2.7.0
```

当前 contract 测试已覆盖：

- HTTP method 和 path。
- JSON 字段名和可选字段。
- response envelope。
- `null`、空数组、空对象行为。
- 代表性错误或降级响应形状。

## 官方 Contract Drift 检查

本仓库保存官方 `remnawave/node 2.7.0` 的 contract hash baseline：

```text
testdata/contracts/official-2.7.0/upstream-contract.sha256.json
```

该 baseline 只保存 `libs/contract` 中 Panel-facing contract 相关 TypeScript 文件的路径和 SHA-256，不保存官方源码正文。检查范围包括：

- `libs/contract/api`
- `libs/contract/commands`
- `libs/contract/constants/errors`
- `libs/contract/constants/xray`
- `libs/contract/models`

本地检查当前基线：

```sh
mise run contract-diff
```

网络不可用但本地已有官方 checkout 时：

```sh
CONTRACT_SOURCE_DIR=tmp/remnawave-node mise run contract-diff
```

检查其他官方 tag：

```sh
CONTRACT_TAG=2.7.1 mise run contract-diff
```

如果检查失败，先查看新增、删除或 hash 变化的文件列表，再对照官方仓库更新 Go contract、route 注册和 golden fixture。

不要把官方 TypeScript contract 包整体复制进仓库。只固化必要的小型请求/响应 JSON fixture，并在 contract 变化时对照官方实现更新。
