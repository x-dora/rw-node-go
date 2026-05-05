# rw-node-go

`rw-node-go` 是 Remnawave Node 兼容服务的 Go 实现。项目目标是对齐官方 `remnawave/node` 2.7.x 面向 Panel 的 API contract，同时保持运行时轻量、结构清晰、便于测试和长期跟随上游。

当前项目处于框架完成并进入 M3 实现的阶段：公开路由、contract 类型、Gin HTTP 层、response envelope、CI、Docker 构建、mTLS/JWT/zstd、Xray 外部进程生命周期、内部 gRPC 健康检查、HandlerService 用户动态管理和基础 StatsService 流量统计已经落地；在线 IP、连接踢出、torrent blocker、nftables 等真实运行时能力仍在后续阶段实现。

stub 和占位响应只用于保持 Panel-facing API 不返回 404，不能视为真实能力。

## 功能进度

状态说明：`[x]` 已完成，`[~]` 部分完成，`[ ]` 未完成。

- [x] Go 项目骨架、Gin HTTP 层、公开路由注册、response envelope、contract struct。
- [x] CI、Dockerfile、GitHub Actions 多架构 Docker 构建流程。
- [x] `SECRET_KEY` 解析、PEM normalize、mTLS、JWT RS256、zstd request body。
- [x] Xray config 注入 Remnawave API inbound、API service、routing、policy stats 和内部 mTLS 证书。
- [x] 外部 Xray 进程启动、停止、配置写入和 StatsService gRPC ready 检查。
- [~] `/node/xray/start`、`/node/xray/stop`、`/node/xray/healthcheck` 已按官方缓存语义接入；Routing 业务方法仍未实现。
- [~] system stats 已按官方 2.7.0 响应形状返回基础快照和 Xray sys stats；完整 CPU、memory、disk、network、interface 字段仍未完成。
- [~] Xray HandlerService 用户动态管理：add/remove/bulk、inbound users、inbound users count 已接入；真实 Panel + Xray 验收仍未完成。
- [~] Xray StatsService 统计：users、inbound、outbound、combined 和 reset 语义已接入；真实 Panel + Xray 验收仍未完成。
- [ ] 在线 IP、drop users connections、drop IPs、Vision block/unblock 的真实实现。
- [ ] torrent blocker、nftables 插件真实实现。
- [ ] contract golden tests、真实 Panel + Xray integration tests。

完整阶段规划见 [docs/roadmap.md](docs/roadmap.md)，详细实现方案见 [REMNAWAVE_NODE_GO_PLAN.md](REMNAWAVE_NODE_GO_PLAN.md)。

## 快速开始

安装工具链并运行测试：

```sh
mise install
mise run test
mise run build
```

启动本地服务：

```sh
NODE_PORT=2222 mise exec -- go run ./cmd/rw-node-go
```

不设置 `SECRET_KEY` 时，服务会以本地 HTTP 模式启动，便于开发和 contract 测试。设置 `SECRET_KEY` 后会启用 HTTPS、mTLS 和 JWT 校验。

## 运行配置

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `NODE_PORT` | `2222` | Panel 访问节点 API 的端口。 |
| `SECRET_KEY` | 空 | 官方 Node 使用的 base64 JSON 密钥包；设置后启用 mTLS/JWT。 |
| `XTLS_API_PORT` | `61000` | 本机 Xray gRPC API 端口，只用于 Go 进程控制 Xray。 |
| `RW_NODE_DIR` | `/opt/rw-node-go` | 节点运行目录，当前用于派生 Xray 配置路径。 |
| `XRAY_BIN` | `/usr/local/bin/xray` | 外部 Xray 二进制路径。 |
| `XRAY_CONFIG_PATH` | `$RW_NODE_DIR/xray/config.json` | Xray 配置文件路径；当前由 `RW_NODE_DIR` 派生，后续会接入显式覆盖。 |
| `LOG_LEVEL` | `info` | 日志级别配置入口。 |
| `REQUEST_BODY_LIMIT_BYTES` | `1073741824` | request body 上限，默认 1 GiB。 |

`XTLS_API_PORT` 和内部控制接口必须只监听本机，不要通过 Docker publish、防火墙、FRP 或 PaaS 入站暴露到公网。

启动 Xray 时，程序会生成一套内存态 internal mTLS 证书，并把 API inbound 注入到 Xray 配置中。`/node/xray/start` 通过本机 TLS gRPC StatsService 确认内部 API 可用；`/node/xray/healthcheck` 按官方 Node 行为返回缓存的内部状态，不在每次请求时重新探测 Xray。

## Docker

本地构建镜像：

```sh
mise run docker-build
```

手动构建：

```sh
docker build -t ghcr.io/x-dora/rw-node-go:local .
```

当前镜像包含 `rw-node-go` 二进制和运行目录，不内置 Xray 二进制。运行真实 Xray 生命周期前，需要在镜像或宿主环境中提供 `XRAY_BIN` 指向的可执行文件。

## 文档

- [docs/architecture.md](docs/architecture.md)：当前架构和运行时边界。
- [docs/contracts.md](docs/contracts.md)：Panel-facing contract 对齐、路由覆盖和 stub 策略。
- [docs/development.md](docs/development.md)：本地开发、测试和文档维护规则。
- [docs/roadmap.md](docs/roadmap.md)：M0-M6 功能路线图和当前进度。
