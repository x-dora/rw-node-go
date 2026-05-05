# 架构说明

本项目把 Remnawave Panel-facing API 兼容层和运行时实现层分开，便于后续跟随官方 Node contract 变化。

## 分层

- `cmd/rw-node-go`：进程入口，负责加载配置、初始化运行状态并启动 HTTP 服务。
- `internal/httpapi`：Gin HTTP 服务、路由注册、response envelope，以及后续 TLS/JWT/zstd 中间件。
- `internal/contracts`：Panel-facing API 的请求和响应类型。
- `internal/controller`：路由处理器。当前大部分处理器是兼容 stub。
- `internal/state`：内存运行状态，包括 Xray 状态、hash、inbound 用户和插件状态。
- `internal/xray`：后续外部 Xray 进程和 gRPC 控制抽象。
- `internal/system`：后续系统统计、网络能力检测、conntrack 和 nftables 集成。
- `internal/plugin`：后续 torrent blocker 和 nftables 插件逻辑。

## 运行时方向

M1 先采用外部 Xray 进程模式。`xray.Core` interface 是 HTTP contract 层和 Xray 实现层的边界，后续如需实验内嵌 Xray，不应影响公开 API。

## 响应格式

外部 Remnawave API 统一返回：

```json
{
  "response": {}
}
```

内部调试接口可以直接返回 JSON 对象，因为它们不是 Panel contract 的一部分。
