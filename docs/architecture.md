# 架构说明

`rw-node-go` 将 Remnawave Panel-facing API 兼容层和内嵌 `xray-core` 运行时分开。公开 API 的路由、method、JSON 字段和 response envelope 必须稳定；内部实现可以逐步把明确标注的 stub 替换为真实运行时能力。

详细功能进度由 [docs/roadmap.md](roadmap.md) 维护。本文只描述当前架构边界和运行路径。

## 分层

- `cmd/rw-node-go`：进程入口，负责加载配置、初始化运行状态、注册 controller 并启动 HTTP 服务。
- `internal/config`：环境变量、`SECRET_KEY` 解码、PEM normalize 和运行配置。
- `internal/httpapi`：Gin main router、internal router、response envelope、body limit、panic recovery、zstd request body、mTLS 和 JWT RS256 middleware。
- `internal/contracts`：Panel-facing API 的请求和响应类型。
- `internal/controller`：路由处理器。Xray controller 管理内嵌 instance；handler、stats 和 vision 通过内嵌 Xray feature 访问运行时；plugin 当前是接口适配 stub。
- `internal/state`：内存运行状态，包括 Xray 状态、当前 config、hash 和 inbound 用户集合。
- `internal/xray`：内嵌 `xray-core` core、config builder、用户构建、stats 读取和 feature client 抽象。
- `internal/system`：系统统计、网络能力检测、conntrack 连接清理和 nftables 未来集成入口。
- `internal/testkit`：证书、JWT、golden 和 Panel client 测试辅助。

## 运行路径

```text
Remnawave Panel
    |
    | HTTPS + mTLS + Bearer JWT
    v
Main Gin API on 0.0.0.0:NODE_PORT
    |
    | controller + runtime state
    v
Embedded xray-core instance
    |
    | inbound/stats/routing features
    v
Xray runtime in the same Go process

Local tooling
    |
    | HTTP on 127.0.0.1:INTERNAL_REST_PORT
    v
Internal Gin API
```

设置 `SECRET_KEY` 后，主 API 通过 TLS server config、mTLS 和 JWT public key 校验 Panel 请求。官方 `/vision/*` route 仍走主 API 的 HTTPS/mTLS，但按官方 2.7.0 行为豁免 Bearer JWT。

不设置 `SECRET_KEY` 时，主 API 以本地 HTTP 模式启动，只用于开发和 contract 测试。Docker 镜像默认要求 `SECRET_KEY`。

## Xray 运行时边界

当前唯一 Xray runtime 是内嵌 `xray-core`。不要重新引入外部 `xray` 进程、Xray 配置落盘主路径、内部 gRPC API inbound 或 internal mTLS：

- `/node/xray/start` 从 Panel 下发的 JSON config 构建内嵌可加载的 Xray config，并启动新的 `xray-core` instance。
- 重复 start 会关闭旧 instance，再替换为新 instance。
- `/node/xray/stop` 会关闭当前内嵌 instance。
- `/node/xray/healthcheck` 按官方 Node 行为返回缓存状态：节点 API 可响应时 `isAlive=true`，`xrayInternalStatusCached` 来自 start/stop 或内部健康检查结果。
- Config builder 只补齐 stats/policy 和 Vision 所需 `BLOCK` outbound，不注入 Remnawave API inbound、API service、internal mTLS 或 plugin webhook。

用户动态管理、stats 和 Vision 优先通过内嵌 Xray feature 访问运行时。Stats online status/IP 通过 Xray stats `OnlineMap` 读取；读取失败或 feature 不可用时按 contract 稳定降级为 `false` 或空列表。

Xray start/restart/stop 会输出官方风格的脱敏表格摘要，便于在 Panel live harness 和容器日志中判断运行状态。配置日志只包含 inbound/outbound/routing rule 数量、inbound tag、用户数量和缩短 hash；不输出完整 Xray config、clients、password、privateKey、shortId、证书、JWT、bearer token 或 `SECRET_KEY`。

## Internal API 边界

`INTERNAL_REST_PORT` 只监听 `127.0.0.1`，不属于 Panel-facing contract，也不走 Panel mTLS/JWT。不要通过 Docker publish、防火墙、FRP 或 PaaS 入站暴露到公网。

- `GET /internal/get-config`：返回当前内存 Xray config；没有 config 时返回 `{}`。

`/vision/block-ip` 和 `/vision/unblock-ip` 是官方主 API 上的 unprefixed Panel-facing route，不属于 internal API。它们通过内嵌 routing feature 添加或删除 source IP dynamic rule。

## 降级和不支持能力

- Handler 和 stats 读取运行时 feature 失败时返回兼容的业务降级响应，不把内部错误暴露为不稳定 JSON 形状。
- Drop users connections 和 drop IPs 通过 Linux conntrack best-effort 清理连接；非 Linux、无 `CAP_NET_ADMIN` 或 conntrack netlink 不可用时稳定降级为 no-op。
- Plugin routes 只做 contract adapter，不保存状态、不注入配置、不接收 webhook、不触发 Xray restart、不执行 nftables、不产生 torrent reports。

## 响应格式

Panel-facing API 统一返回：

```json
{
  "response": {}
}
```

Internal API 可以直接返回 JSON 对象，因为它不是 Panel contract 的一部分。
