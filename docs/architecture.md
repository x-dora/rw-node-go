# 架构说明

`rw-node-go` 把 Remnawave Panel-facing API 兼容层和 Xray 运行时控制层分开。公开 API 的路由、method、JSON 字段和 response envelope 必须稳定；内部实现可以按 M0-M6 逐步替换 stub。

## 分层

- `cmd/rw-node-go`：进程入口，负责加载配置、初始化运行状态、注册 controller 并启动 HTTP 服务。
- `internal/config`：环境变量、`SECRET_KEY` 解码、PEM normalize 和运行路径配置。
- `internal/httpapi`：Gin router、response envelope、body limit、panic recovery、zstd request body、mTLS 和 JWT RS256 middleware。
- `internal/contracts`：Panel-facing API 的请求和响应类型。
- `internal/controller`：路由处理器。Xray controller 已接入外部进程控制；handler、stats、plugin、vision 仍主要是兼容 stub。
- `internal/state`：内存运行状态，包括 Xray 状态、当前 config、hash、inbound 用户集合和 plugin 状态。
- `internal/xray`：Xray config builder、内部 mTLS 证书、外部进程 core 和 Xray gRPC client 抽象。
- `internal/system`：系统统计、网络能力检测、conntrack 和 nftables 集成入口。
- `internal/plugin`：torrent blocker、nftables 插件状态和报告处理入口。
- `internal/testkit`：证书、JWT、golden 和 Panel client 测试辅助。

## 当前运行路径

```text
Remnawave Panel
    |
    | HTTPS + mTLS + Bearer JWT
    v
Gin HTTP API
    |
    | controller + runtime state
    v
Xray Core abstraction
    |
    | external xray process + generated config
    v
Xray TLS gRPC API on 127.0.0.1:XTLS_API_PORT
```

当前已实现的运行路径包括：

- `SECRET_KEY` 解码后生成 TLS server config 和 JWT public key。
- request body 支持 zstd 解压和大小限制。
- `/node/xray/start` 会构建完整 Xray config，写入配置文件，停止旧进程并启动新 Xray 进程。
- config builder 会注入 Remnawave API inbound、API service、routing rule、policy stats 和内部 mTLS 证书。
- 内部 mTLS 证书在 Go 进程启动时生成并只保存在内存中。API inbound 使用 server certificate/key 和 `usage: "verify"` 的 CA certificate；Go gRPC client 使用 client certificate、同一 CA 和 `internal.remnawave.local` SNI 连接 `127.0.0.1:XTLS_API_PORT`。
- Xray start/restart 路径通过 StatsService `GetSysStats` 确认内部 API 可用；确认成功后才缓存 Xray 内部在线状态。
- `/node/xray/stop` 会停止当前外部 Xray 进程。
- `/node/xray/healthcheck` 按官方 Node 行为返回缓存状态：节点 API 可响应时 `isAlive=true`，`xrayInternalStatusCached` 来自上一次 start/stop 或内部健康检查结果，不在 healthcheck 请求中实时探测 Xray。

## 未完成边界

- Xray gRPC client 已具备基础连接和 StatsService health check；HandlerService、StatsService、RoutingService 的业务方法尚未接入 controller。
- 用户动态管理接口当前返回兼容成功或空集合，不会真实修改 Xray inbound 用户。
- stats 接口当前返回基础快照、空流量或 false，不会从 Xray StatsService 读取真实数据。
- plugin、nftables、conntrack、Vision block/unblock 当前是占位行为。
- 内部接口当前用于调试和 webhook 占位，后续需要补本机访问保护和插件逻辑。

## 响应格式

Panel-facing API 统一返回：

```json
{
  "response": {}
}
```

内部调试接口可以直接返回 JSON 对象，因为它们不是 Panel contract 的一部分。
