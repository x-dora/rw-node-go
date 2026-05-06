# 架构说明

`rw-node-go` 把 Remnawave Panel-facing API 兼容层和内嵌 `xray-core` 运行时分开。公开 API 的路由、method、JSON 字段和 response envelope 必须稳定；内部实现可以逐步替换 stub。

## 分层

- `cmd/rw-node-go`：进程入口，负责加载配置、初始化运行状态、注册 controller 并启动 HTTP 服务。
- `internal/config`：环境变量、`SECRET_KEY` 解码、PEM normalize 和运行配置。
- `internal/httpapi`：Gin main router、internal router、response envelope、body limit、panic recovery、zstd request body、mTLS 和 JWT RS256 middleware。
- `internal/contracts`：Panel-facing API 的请求和响应类型。
- `internal/controller`：路由处理器。Xray controller 管理内嵌 instance；handler、stats 和 vision 通过内嵌 Xray feature 访问运行时；plugin 当前是接口适配 stub。
- `internal/state`：内存运行状态，包括 Xray 状态、当前 config、hash 和 inbound 用户集合。
- `internal/xray`：内嵌 `xray-core` core、config builder、用户构建、stats 读取和 feature client 抽象。
- `internal/system`：系统统计、网络能力检测、conntrack 和 nftables 未来集成入口。
- `internal/testkit`：证书、JWT、golden 和 Panel client 测试辅助。

## 当前运行路径

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
    | inbound/stats features
    v
Xray runtime in the same Go process

Local tooling
    |
    | HTTP on 127.0.0.1:INTERNAL_REST_PORT
    v
Internal Gin API
```

当前已实现的运行路径包括：

- `SECRET_KEY` 解码后生成 TLS server config 和 JWT public key。
- request body 支持 zstd 解压和大小限制。
- `/node/xray/start` 会构建内嵌可加载的 Xray config，启动新的 `xray-core` instance；重复 start 会关闭旧 instance。
- config builder 只补齐 stats/policy 和 Vision 所需 `BLOCK` outbound，不注入 Remnawave API inbound、API service、internal mTLS 或 plugin webhook。
- `/node/xray/stop` 会关闭当前内嵌 Xray instance。
- `/node/xray/healthcheck` 按官方 Node 行为返回缓存状态：节点 API 可响应时 `isAlive=true`，`xrayInternalStatusCached` 来自 start/stop 或内部健康检查结果。
- `/vision/block-ip` 和 `/vision/unblock-ip` 是官方主 API 上的 unprefixed Panel-facing route；设置 `SECRET_KEY` 后仍走 HTTPS/mTLS，但按官方 2.7.0 行为豁免 Bearer JWT。
- `INTERNAL_REST_PORT` 上保留本机 internal API。`/internal/get-config` 返回当前内存 config。

## 未完成边界

- 用户动态管理接口已通过内嵌 Xray inbound feature 增删和查询 inbound 用户；真实 Panel + Xray 验收仍未完成。
- stats 接口已从内嵌 Xray stats feature 读取 users、inbound、outbound 和 combined 统计；online status/IP 当前按系统能力稳定降级。
- drop users connections、drop IPs 当前不操作 conntrack 或 nftables。
- Vision block/unblock 已通过内嵌 routing feature 添加或删除 source IP dynamic rule；真实 Panel + Xray 验收仍未完成。
- plugin routes 只做 contract adapter，不保存状态、不注入配置、不接收 webhook、不触发 Xray restart、不执行 nftables、不产生 torrent reports。

## 响应格式

Panel-facing API 统一返回：

```json
{
  "response": {}
}
```

Internal API 可以直接返回 JSON 对象，因为它不是 Panel contract 的一部分。
