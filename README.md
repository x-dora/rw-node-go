# rw-node-go

`rw-node-go` 是 Remnawave Node 兼容服务的 Go 实现，目标是对齐官方 `remnawave/node` 2.7.x 面向 Panel 的 API contract。当前主线使用内嵌 `xray-core`：Go 进程直接加载 Panel 下发的 Xray JSON config，启动并管理内存中的 Xray instance。

本项目不再管理外部 `xray` 进程，不写入 Xray 配置文件，不注入内部 gRPC API inbound，也不实现 plugin 运行时能力。Plugin 相关路由只做 Panel-facing contract adapter，避免 Panel 调用时返回 404，但不会产生官方 plugin side effects。

## 功能进度

状态说明：`[x]` 已完成，`[~]` 部分完成，`[ ]` 未完成。

- [x] Go 项目骨架、Gin HTTP 层、公开路由注册、response envelope、contract struct。
- [x] CI、Dockerfile、GitHub Actions 多架构 Docker 构建流程。
- [x] `SECRET_KEY` 解析、PEM normalize、mTLS、JWT RS256、zstd request body。
- [x] 内嵌 `xray-core` 启动、停止、重复 start 替换旧 instance。
- [x] Xray config 最小 stats/policy 注入；不注入 Remnawave API inbound/service。
- [x] 本机 internal REST API：`127.0.0.1:INTERNAL_REST_PORT` 上的 `/internal/get-config`。
- [~] `/node/xray/start`、`/node/xray/stop`、`/node/xray/healthcheck` 已按官方缓存状态语义接入；真实 Panel + Xray 验收仍未完成。
- [~] Handler 用户动态管理通过内嵌 Xray inbound feature 接入；真实 Panel + Xray 验收仍未完成。
- [~] Stats users、inbound、outbound、combined 通过内嵌 Xray stats feature 接入；online status/IP 当前按系统能力降级。
- [~] Vision `/vision/block-ip`、`/vision/unblock-ip` 已走内嵌 routing feature；真实 Panel + Xray 验收仍未完成。
- [ ] drop users connections、drop IPs 的真实实现。
- [x] Plugin routes 只做 contract adapter：sync accepted、torrent blocker collect 空数组、nftables accepted；不保存状态、不重启 Xray、不执行 nftables。
- [~] contract golden tests 和 contract drift 检查已接入；真实 Panel + Xray integration tests 未完成。

## 快速开始

安装工具链并运行测试：

```sh
mise install
mise run test
mise run build
```

启动本地服务：

```sh
NODE_PORT=2222 INTERNAL_REST_PORT=61001 mise exec -- go run ./cmd/rw-node-go
```

不设置 `SECRET_KEY` 时，服务会以本地 HTTP 模式启动，只用于本地开发和 contract 测试。生产部署必须设置 `SECRET_KEY`，或使用默认启用 `REQUIRE_SECRET_KEY=true` 的 Docker 镜像让缺少密钥的容器直接启动失败。设置 `SECRET_KEY` 后会启用 HTTPS、mTLS 和 JWT 校验；官方 `/vision/*` 路由保留 mTLS，但按官方 2.7.0 行为豁免 Bearer JWT。

## 运行配置

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `NODE_PORT` | `2222` | Panel 访问节点 API 的端口。 |
| `INTERNAL_REST_PORT` | `61001` | 本机 internal REST API 端口，只监听 `127.0.0.1`。 |
| `SECRET_KEY` | 空 | 官方 Node 使用的 base64 JSON 密钥包；设置后启用 mTLS/JWT。 |
| `REQUIRE_SECRET_KEY` | `false` | 裸进程默认允许本地开发 HTTP；Docker 镜像默认设为 `true`。 |
| `RW_NODE_DIR` | `/opt/rw-node-go` | 节点运行目录预留入口。 |
| `LOG_LEVEL` | `info` | 日志级别配置入口。 |
| `REQUEST_BODY_LIMIT_BYTES` | `1073741824` | request body 上限，默认 1 GiB。 |

`INTERNAL_REST_PORT` 必须保持本机访问，不要通过 Docker publish、防火墙、FRP 或 PaaS 入站暴露到公网。

## Internal API

Internal API 不走 Panel mTLS/JWT，只供本机调试或内部控制面使用：

- `GET /internal/get-config`：返回当前内存中的 Xray config；没有 config 时返回 `{}`。

Vision API 是官方主服务上的 unprefixed Panel-facing route，不属于 internal API：

- `POST /vision/block-ip`：添加内嵌 Xray dynamic source IP routing rule，目标 outbound 为 `BLOCK`。
- `POST /vision/unblock-ip`：删除对应 dynamic routing rule。

## Docker

本地构建镜像：

```sh
mise run docker-build
```

手动构建：

```sh
docker build -t ghcr.io/x-dora/rw-node-go:local .
```

当前镜像包含 `rw-node-go` 二进制。Xray 运行时来自内嵌 `xray-core`，不需要额外提供外部 `xray` 二进制。镜像默认 `REQUIRE_SECRET_KEY=true`；本地容器调试如需 HTTP contract 模式，需要显式覆盖为 `REQUIRE_SECRET_KEY=false`。

## 文档

- [docs/architecture.md](docs/architecture.md)：当前架构和运行时边界。
- [docs/contracts.md](docs/contracts.md)：Panel-facing contract 对齐、路由覆盖和 stub 策略。
- [docs/development.md](docs/development.md)：本地开发、测试和文档维护规则。
- [docs/roadmap.md](docs/roadmap.md)：功能路线图和当前进度。
