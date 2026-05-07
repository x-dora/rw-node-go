# rw-node-go

`rw-node-go` 是 Remnawave Node 兼容服务的 Go 实现，目标是对齐官方 `remnawave/node` 2.7.x 面向 Panel 的 API contract。当前主线使用内嵌 `xray-core`：Go 进程直接加载 Panel 下发的 Xray JSON config，并在同一进程内启动、停止和管理 Xray instance。

本项目不再管理外部 `xray` 进程，不把 Xray 配置作为主路径落盘，不注入内部 gRPC API inbound，也不实现 plugin 运行时能力。Plugin 相关路由只做 Panel-facing contract adapter，避免 Panel 调用时返回 404，但不会产生官方 plugin side effects。

## 当前能力

详细进度矩阵见 [docs/roadmap.md](docs/roadmap.md)。入口文档只保留关键状态：

- 已建立 Gin HTTP 层、公开路由注册、contract struct、response envelope、CI、Docker 构建和 release 流程。
- 已接入 `SECRET_KEY` 解析、PEM normalize、mTLS、JWT RS256 和 zstd request body。
- `/node/xray/start`、`/node/xray/stop`、`/node/xray/healthcheck` 已接入内嵌 Xray instance 生命周期。
- Handler、stats、Vision 和连接清理已部分接入内嵌 Xray feature 或系统能力；真实 Panel + Xray 的完整验收仍在推进。
- Stats online status/IP 已通过内嵌 Xray stats `OnlineMap` 接入；不可用或读取失败时稳定降级为 `false` 或空列表。
- Plugin routes 只做 contract adapter，不保存状态、不重启 Xray、不执行 nftables。

## 版本语义

本项目有两个互相独立的版本：

- `VERSION`：`rw-node-go` 自己的语义化发布版本，构建和 Docker 镜像会把它注入为 `ProjectVersion`。
- `nodeVersion`：上报给 Remnawave Panel 的兼容性版本，默认对齐官方 `remnawave/node` 2.7.x 的 `2.7.0`。它只用于 Panel 兼容性检查，不代表本项目发布版本。
- `mise run build`、CI、Docker 和 release 都走同一个构建入口读取 `VERSION` 并注入构建元信息；不要直接用裸 `go build` 代替发布构建路径。

正式 release 发布后，GitHub Actions 会推送：

- `ghcr.io/x-dora/rw-node-go:latest`
- `ghcr.io/x-dora/rw-node-go:v<VERSION>`

发布流程、GHCR 权限和恢复入口见 [docs/development.md](docs/development.md)。

## 快速开始

安装工具链并运行基础验证：

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

发布前本地验证：

```sh
mise run preflight
```

如需把真实 Panel live harness 纳入验证，设置 `RUN_PANEL_INTEGRATION=true` 后再运行 `mise run preflight`。该流程会启用并禁用真实 Panel 测试节点，只能指向专用测试节点。

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
| `XRAY_LOCATION_ASSET` | 空 | Xray geodata 目录。Docker 镜像固定设置为 `/usr/local/share/xray`。 |

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

当前镜像包含 `rw-node-go` 二进制和 Xray geodata。Xray 运行时来自内嵌 `xray-core`，不需要额外提供外部 `xray` 二进制。镜像内默认设置 `XRAY_LOCATION_ASSET=/usr/local/share/xray` 和 `REQUIRE_SECRET_KEY=true`；本地容器调试如需 HTTP contract 模式，需要显式覆盖为 `REQUIRE_SECRET_KEY=false`。

## 真实 Panel 联调

真实 Panel 联调唯一入口是 `scripts/panel-integration.sh`，详细配置见 [docs/development.md](docs/development.md)。该 harness 会修改真实 Panel 节点状态，`run`、`enable` 和 `disable` 必须使用完整节点 UUID 且只应指向专门的测试节点；普通 `go test ./...` 不会连接真实 Panel。

```sh
bash scripts/panel-integration.sh summary
bash scripts/panel-integration.sh run
```

## 文档

- [docs/architecture.md](docs/architecture.md)：当前架构和运行时边界。
- [docs/contracts.md](docs/contracts.md)：Panel-facing contract 对齐、路由覆盖和 stub 策略。
- [docs/development.md](docs/development.md)：本地开发、测试、真实 Panel harness 和发布流程。
- [docs/roadmap.md](docs/roadmap.md)：功能路线图和详细进度矩阵。
