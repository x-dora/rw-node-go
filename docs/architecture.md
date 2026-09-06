# 架构说明

`rw-node-go` 将 Remnawave Panel-facing API 兼容层和内嵌 [`xray-core`](https://github.com/XTLS/Xray-core) 运行时分开。公开 API 的路由、method、JSON 字段和 response envelope 必须稳定；内部实现可以逐步把明确标注的 stub 替换为真实运行时能力。

详细功能进度由 [docs/roadmap.md](roadmap.md) 维护。本文只描述当前架构边界和运行路径。

## 分层

- [`cmd/rw-node-go`](../cmd/rw-node-go)：进程入口，负责加载配置、初始化运行状态、注册 controller 并启动 HTTP 服务。
- [`internal/config`](../internal/config)：环境变量、`SECRET_KEY` 解码、PEM normalize、`SECRET_KEY` 载荷完整性校验和运行配置。
- [`internal/httpapi`](../internal/httpapi)：Gin main router、internal router、response envelope、body limit、panic recovery、zstd request body、TLS client auth、SNI 派生门控和 JWT RS256 middleware。
- [`internal/contracts`](../internal/contracts)：Panel-facing API 的请求和响应类型。
- [`internal/controller`](../internal/controller)：路由处理器。Xray controller 管理内嵌 instance 和 geodata 资产准备；handler 和 stats 通过内嵌 Xray feature 访问运行时；plugin 当前是接口适配 stub。
- [`internal/state`](../internal/state)：内存运行状态，包括 Xray 状态、当前 config、hash 和 inbound 用户集合。
- [`internal/xray`](../internal/xray)：内嵌 `xray-core` core、config builder、geodata assets 下载、用户构建、stats 读取和 feature client 抽象。
- [`internal/geocheck`](../internal/geocheck)：`get-geocheck` 路由的 geocheck 二进制 runner（exec 镜像内置的官方 `geocheck`）。
- [`internal/system`](../internal/system)：系统统计、网络能力检测、conntrack 连接清理和 nftables 未来集成入口。
- [`internal/testkit`](../internal/testkit)：证书、JWT、golden 和 Panel client 测试辅助。

## 运行路径

```text
Remnawave Panel
    |
    | HTTPS + TLS client auth + Bearer JWT
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

设置 `SECRET_KEY` 后，主 API 通过 TLS server config、TLS client auth 和 JWT public key 校验 Panel 请求。启动时先解码 `SECRET_KEY` 载荷并做完整性校验（CA 可解析/未过期/自签名、node cert 由该 CA 签发、node key 与 cert 匹配、JWT public key 可解析，对齐官方 3.3.1 `assertPayloadIntegrity`），校验失败输出逐项报告并中止启动。默认 `NODE_TLS_CLIENT_AUTH=mtls` 会要求并验证客户端证书，保持官方 mTLS 行为。`NODE_TLS_CLIENT_AUTH=optional` 会在客户端提交证书时校验，`NODE_TLS_CLIENT_AUTH=none` 只保留 HTTPS 和 JWT，适用于前置可信代理已经完成客户端证书校验的部署。

TLS 最低版本是 TLS 1.3，跟随官方的 `httpsOptions.minVersion = 'TLSv1.3'`。这对只支持 TLS 1.2 的前置代理、反向代理或探活工具是破坏性变更，它们会在握手阶段失败。官方 3.3.0 的 `handshakeTimeout: 10s` 由 `http.Server.ReadHeaderTimeout`（10s，从 Accept 起计时并覆盖 TLS 握手阶段）等价覆盖。

`SNI_VERIFICATION=true`（默认 `false`，对齐官方 3.4.0 默认关闭）时，主 API 握手由 HKDF-SHA256 从 `SECRET_KEY` 载荷（jwtPublicKey ‖ caCertPem，info `rw-v1`）派生的 SNI 门控：只有 ClientHello 的 ServerName 与派生值匹配才完成握手，常量时间比较，其余一律拒绝。派生算法与官方 `decode-servername.util.ts` 字节级一致（有 golden 向量测试）。

官方已移除 `/vision/*` Panel-facing route 和 `get-inbound-users`/`get-inbound-users-count` 两条 route（3.4.1），Go 侧同步不注册这些入口。使用 `NODE_TLS_CLIENT_AUTH=none` 时，前置代理仍必须限制源站访问并完成客户端证书校验，但 Node 层会继续用 JWT 保护所有已注册的 Panel-facing route。

不设置 `SECRET_KEY` 时，主 API 以本地 HTTP 模式启动，只用于开发和 contract 测试。Docker 镜像默认要求 `SECRET_KEY`。

## Xray 运行时边界

当前唯一 Xray runtime 是内嵌 [`xray-core`](https://github.com/XTLS/Xray-core)。不要重新引入外部 `xray` 进程、Xray 配置落盘主路径、内部 gRPC API inbound 或 internal mTLS：

- `/node/xray/start` 从 Panel 下发的 JSON config 构建内嵌可加载的 Xray config，并启动新的 `xray-core` instance。
- 重复 start 会先验证并构造新 instance，再关闭旧 instance 释放监听端口并启动新 instance；如果配置解析或 instance 构造失败，旧 instance 会保留运行。
- `/node/xray/stop` 会关闭当前内嵌 instance。
- start 受 15s 超时约束：内嵌 instance 的启动调用被超时 context 包住，超时后返回 `xray core did not become ready in time`，并在启动调用返回后关闭这个被放弃的 instance。
- `/node/xray/healthcheck` 按官方 Node 行为返回缓存状态：节点 API 可响应时 `isAlive=true`，`xrayInternalStatusCached` 来自 start/stop 或内部健康检查结果。
- Config builder 只补齐 stats/policy，不注入 Remnawave API inbound、API service、internal mTLS、Vision `BLOCK` outbound 或 plugin webhook。
- start 时按 Panel 下发的 `xrayConfig.geodata.assets`（官方 3.1.0 起）下载 geodata 资产到 Xray asset 目录（`XRAY_LOCATION_ASSET`，次选 `XRAY_ASSET_DIR`，默认 `/usr/local/share/xray`）：URL 必须 https、文件名禁止路径段，已存在非空文件跳过，下载失败落空 stub 并告警，失败不阻断启动。`geodata.core` 换 xray 二进制不实现（deliberate divergence）：内嵌 core 编译进二进制，运行时不可替换，下发时记录告警并忽略。
- start 请求的 `internals.metadata` 和 `internals.integrations`（官方 3.2/3.3.0 新增）接受但忽略：metadata 暂无消费者；integrations 等价于官方无已启用 integration 时 sync 直接成功（deliberate divergence，plugin 保持 adapter-only）。
- 官方的 `DISABLE_HASHED_SET_CHECK` 在 Go 侧不存在（deliberate divergence）：hash 短路只能由 Panel 请求里的 `internals.forceRestart` 绕过，没有 env 开关。

用户动态管理和 stats 优先通过内嵌 Xray feature 访问运行时。Stats online status/IP 通过 Xray stats `OnlineMap` 读取；该能力依赖 Linux `CAP_NET_ADMIN` 让 Xray policy 启用 `statsUserOnline`，读取失败、feature 不可用或 capability 不足时按 contract 稳定降级为 `false` 或空列表。

官方 [`3.0.0`](https://github.com/remnawave/node/tree/3.0.0) 在外部进程模式下把 Xray internal API 和 internal webserver 都改成 abstract Unix socket：`INTERNAL_SOCKET_PATH` 以 `\0` 前缀监听在 abstract namespace，不再落盘也不再 unlink，`INTERNAL_SOCKET_PATH`、`XTLS_API_SOCKET_PATH` 和 `INTERNAL_REST_TOKEN` 由容器内 s6 `init-env.sh` 随机生成。Go 版当前唯一运行模式是内嵌 `xray-core`，因此不新增 `XTLS_API_SOCKET_PATH`，也不恢复外部进程、internal gRPC inbound 或 internal mTLS；internal REST 固定监听 `127.0.0.1` TCP（deliberate divergence，边界见下节）。

官方 3.3.0 的 instance lock（abstract unix socket `\0rwnode-lock` 检测同 network namespace 重复实例并告警）不实现（deliberate divergence）：Windows 主机开发场景无 abstract unix socket；live harness 已有自己的 pid file 防重入。

Xray start/restart/stop 会输出官方风格的脱敏表格摘要，便于在 Panel live harness 和容器日志中判断运行状态。配置日志只包含 inbound/outbound/routing rule 数量、stats/policy 是否存在、`statsUserOnline` 是否启用、inbound tag、用户数量和缩短 hash；不输出完整 Xray config、clients、password、privateKey、shortId、证书、JWT、bearer token 或 `SECRET_KEY`。

## Internal API 边界

`INTERNAL_REST_PORT` 只监听 `127.0.0.1`，不属于 Panel-facing contract，也不走 Panel mTLS/JWT。不要通过 Docker publish、防火墙、FRP 或 PaaS 入站暴露到公网。

- `GET /internal/get-config`：返回当前内存 Xray config；没有 config 时返回 `{}`。当 `SECRET_KEY` 可解码时会额外在响应顶层注入 `panelSni`（HKDF 派生的 Panel SNI），供本机 front proxy（如 Caddy inbound watcher）在 `SNI_VERIFICATION` 开启时使用；该字段是注入的工具信息，不是 Panel 下发 Xray config 的一部分，派生失败时不注入。

官方 [`3.0.0`](https://github.com/remnawave/node/tree/3.0.0) 已移除 `/vision/block-ip` 和 `/vision/unblock-ip` public contract；官方 3.4.1 已移除 `/node/handler/get-inbound-users` 和 `/node/handler/get-inbound-users-count`。Go 侧访问这些路径会按未注册 route 返回 404。

## 降级和不支持能力

- Handler 和 stats 读取运行时 feature 失败时返回兼容的业务降级响应，不把内部错误暴露为不稳定 JSON 形状。
- Panel `fetch-users-ips` / `get-users-ip-list` 依赖 Xray OnlineMap；非 Linux 或无 `CAP_NET_ADMIN` 时 `statsUserOnline` 不启用，在线 IP 稳定降级为空。
- Drop users connections 和 drop IPs 通过 Linux conntrack best-effort 清理连接；非 Linux、无 `CAP_NET_ADMIN` 或 conntrack netlink 不可用时稳定降级为 no-op。`add-user` 带 `prevVlessUuid` 重新注册时，同样 best-effort 地对用户当前 IP 执行连接清理（对齐官方 3.4.1）。
- `get-geocheck` 依赖镜像内置的官方 `geocheck` 二进制（Dockerfile 下载 v0.3.0 到 `/usr/local/bin/geocheck`）；裸机部署可用 `GEOCHECK_BINARY_PATH` 覆盖二进制路径（`ExecRunner.BinaryPath` 字段 > env > 默认路径）。二进制缺失、执行失败、输出超限或已有运行中请求时返回官方错误码 `A018`，单飞互斥、45s 超时。
- Plugin routes 只做 contract adapter，不保存状态、不注入配置、不接收 webhook、不触发 Xray restart、不执行 nftables、不产生 torrent reports。官方 3.0.0 新增的 pre-start plugin（按 glob 清理 stale unix socket）同样不实现：内嵌 core 不产生需要清理的 socket 文件。

## 响应格式

Panel-facing API 统一返回：

```json
{
  "response": {}
}
```

Internal API 可以直接返回 JSON 对象，因为它不是 Panel contract 的一部分。
