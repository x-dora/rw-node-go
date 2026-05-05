# Contract 说明

兼容性来源以官方 `remnawave/node` 2.7.x 面向 Panel 的 contract 和实际实现为准。Go 侧公开类型放在 `internal/contracts`，HTTP route 注册放在 `internal/httpapi/router.go`。

`tmp/remnawave-node` 是官方 2.7.0 仓库，必要时应参考其 contract、controller、service、plugin、Xray 配置生成和错误处理实现。如果 `REMNAWAVE_NODE_GO_PLAN.md`、`docs/roadmap.md` 或当前实现假设与官方仓库冲突，以官方仓库为准。

## 覆盖原则

- 保持公开路由路径、HTTP method、JSON 字段名和 response envelope 稳定。
- 已知官方 contract 中的 Panel-facing route 至少要注册，避免 Panel 调用时得到 404。
- contract 类型无法解释的行为，要继续查看官方仓库中对应 controller/service 的实现。
- 未实现的能力必须返回明确的兼容占位数据，不能伪装成真实 Xray、stats、plugin、nftables 或 conntrack 行为。
- 业务失败优先保持官方风格：HTTP status 尽量稳定，通过 `response.error`、`response.success` 或对应业务字段表达失败。

## 当前路由状态

状态说明：`done` 已有真实或基础实现，`partial` 部分接入，`stub` 仅兼容占位。

| 分组 | 路由 | 状态 | 说明 |
| --- | --- | --- | --- |
| Xray | `POST /node/xray/start` | partial | 已接入 config 注入和外部进程启动；gRPC 验收仍未完成。 |
| Xray | `GET /node/xray/stop` | partial | 已接入外部进程停止。 |
| Xray | `GET /node/xray/healthcheck` | partial | 当前基于进程状态和缓存版本返回。 |
| Handler | `/node/handler/*` | stub | 返回成功、空用户或 count 0；不调用 HandlerService。 |
| Stats | `/node/stats/*` | stub/partial | system stats 有基础快照，其余多为空数据；不调用 StatsService。 |
| Vision | `/vision/block-ip`, `/vision/unblock-ip` | stub | 返回成功；不操作 conntrack 或 nftables。 |
| Plugin | `/node/plugin/*` | stub | 返回 accepted 或空 reports；不执行真实插件逻辑。 |
| Internal | `/internal/get-config`, `/internal/webhook` | partial | 调试和 webhook 占位，不属于 Panel-facing contract。 |

## Golden 测试

Golden fixture 预留目录：

```text
testdata/contracts/official-2.7.0
```

后续 contract 测试应覆盖：

- HTTP method 和 path。
- JSON 字段名和可选字段。
- response envelope。
- `null`、空数组、空对象行为。
- 时间字符串格式。
- 错误响应形状。

不要把官方 TypeScript contract 包整体复制进仓库。只固化必要的小型请求/响应 JSON fixture，并在 contract 变化时对照官方实现更新。
