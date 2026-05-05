# Contract 说明

兼容性来源以官方 `remnawave/node` 2.7.x 面向 Panel 的 contract 和实际实现为准。Go 侧公开类型放在 `internal/contracts`，HTTP route 注册放在 `internal/httpapi/router.go`。

`tmp/remnawave-node` 是官方 2.7.0 仓库，必要时应参考其 contract、controller、service、plugin、Xray 配置生成和错误处理实现。如果 `REMNAWAVE_NODE_GO_PLAN.md`、`docs/roadmap.md` 或当前实现假设与官方仓库冲突，以官方仓库为准。

当前已从官方 2.7.0 contract 手工整理小型 golden manifest：

```text
testdata/contracts/official-2.7.0/panel-api.json
```

该 manifest 覆盖官方 Panel-facing route、代表性请求和响应 envelope，用于 Go contract struct 的 strict decode、响应 JSON 形状和路由注册测试。它不包含内部 `/internal/...` 调试接口，也不复制官方 TypeScript contract 包。

## 覆盖原则

- 保持公开路由路径、HTTP method、JSON 字段名和 response envelope 稳定。
- 已知官方 contract 中的 Panel-facing route 至少要注册，避免 Panel 调用时得到 404。
- contract 类型无法解释的行为，要继续查看官方仓库中对应 controller/service 的实现。
- 未实现的能力必须返回明确的兼容占位数据，不能伪装成真实 Xray、stats、plugin、nftables 或 conntrack 行为。
- 业务失败优先保持官方风格：Xray start/handler mutation 多数通过 `response.error`、`response.success` 或对应业务字段表达失败；官方 stats 查询失败使用 `{timestamp,path,message,errorCode}` 和对应 HTTP status。

## 当前路由状态

状态说明：`done` 已有真实或基础实现，`partial` 部分接入，`stub` 仅兼容占位。

| 分组 | 路由 | 状态 | 说明 |
| --- | --- | --- | --- |
| Xray | `POST /node/xray/start` | partial | 已接入 config 注入和外部进程启动；gRPC 验收仍未完成。 |
| Xray | `GET /node/xray/stop` | partial | 已接入外部进程停止。 |
| Xray | `GET /node/xray/healthcheck` | partial | 当前按官方缓存在线状态和缓存版本返回。 |
| Handler | `/node/handler/add-user`, `/node/handler/add-users`, `/node/handler/remove-user`, `/node/handler/remove-users`, `/node/handler/get-inbound-users`, `/node/handler/get-inbound-users-count` | partial | 已接入 Xray HandlerService 和内存 inbound/user hash 状态；真实 Panel + Xray 验收仍未完成。 |
| Handler | `/node/handler/drop-users-connections`, `/node/handler/drop-ips` | stub | 返回成功；不操作 conntrack 或 nftables。 |
| Stats | `/node/stats/*` | partial | system stats 已按官方 2.7.0 形状返回宿主机 CPU、memory、uptime、load、network interface 和 Xray sys stats；users、inbound、outbound、combined、user online status、user IP list 和 users IP list 已接入 Xray StatsService；主要 stats 查询失败会返回官方错误 envelope；真实 Panel + Xray 验收仍未完成。 |
| Vision | `/vision/block-ip`, `/vision/unblock-ip` | stub | 返回成功；不操作 conntrack 或 nftables。 |
| Plugin | `/node/plugin/sync`, `/node/plugin/torrent-blocker/collect` | partial | 已接入 torrent blocker 配置内存态、官方 Unix socket webhook URL 注入、Xray webhook report 收集和 collect flush；plugin 配置变化会停止 Xray 等待 Panel 重新 start；真实 nftables 封禁未实现，报告 `blocked=false`。 |
| Plugin | `/node/plugin/nftables/*` | stub | 返回 accepted；不执行真实 nftables 操作。 |
| Internal | `/internal/get-config`, `/internal/webhook` | partial | `get-config` 用于调试；`webhook` 通过内部 Unix socket 接收 torrent blocker report，可用 `INTERNAL_REST_TOKEN` 保护。主公网 API 仅保留 `get-config` 的 JWT 豁免。 |

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

`index.ts`、`package.json`、`tsconfig.json` 和非 `.ts` 文件不会进入比较。若官方目录结构变化导致上述路径不存在，检查会失败并要求人工确认上游 contract 结构。

本地检查当前基线：

```sh
mise run contract-diff
```

检查其他官方 tag：

```sh
CONTRACT_TAG=2.7.1 mise run contract-diff
```

也可以在 GitHub Actions 的 `Contract Diff` 手动 workflow 中输入 tag 触发检查。该 workflow 不在普通 `push` 或 `pull_request` 中运行，避免外部网络和 GitHub 限流影响常规 CI。

如果检查失败，先查看新增、删除或 hash 变化的文件列表，再对照官方仓库更新 Go contract、route 注册和 golden fixture。确认兼容调整后，再重新生成对应版本的 baseline：

```sh
go run ./cmd/contract-diff -tag X.Y.Z -source-dir tmp/remnawave-node -baseline testdata/contracts/official-X.Y.Z/upstream-contract.sha256.json -write-baseline
```

生成 baseline 后运行：

```sh
mise run test
```

后续仍需补充：

- 更完整的 golden fixture 变体，特别是非空 stats、torrent blocker report 和真实 HandlerService 返回。
- 自动监听官方 release 的提醒机制。
- 真实 Panel + Xray integration test 的端到端响应对照。

不要把官方 TypeScript contract 包整体复制进仓库。只固化必要的小型请求/响应 JSON fixture，并在 contract 变化时对照官方实现更新。
