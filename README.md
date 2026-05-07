# rw-node-go

[![CI](https://github.com/x-dora/rw-node-go/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/x-dora/rw-node-go/actions/workflows/ci.yml)
[![Preflight](https://github.com/x-dora/rw-node-go/actions/workflows/preflight.yml/badge.svg?branch=main)](https://github.com/x-dora/rw-node-go/actions/workflows/preflight.yml)
[![Docker](https://github.com/x-dora/rw-node-go/actions/workflows/docker.yml/badge.svg?branch=main)](https://github.com/x-dora/rw-node-go/actions/workflows/docker.yml)
[![Release](https://img.shields.io/github/v/release/x-dora/rw-node-go?include_prereleases&label=release)](https://github.com/x-dora/rw-node-go/releases)
[![License](https://img.shields.io/github/license/x-dora/rw-node-go)](LICENSE)

`rw-node-go` 是 Remnawave Node 兼容服务的 Go 实现，目标是对齐官方 `remnawave/node` 2.7.x 面向 Panel 的 API contract。当前主线运行模式是内嵌 `xray-core`：Go 进程直接接收 Panel 下发的 Xray JSON config，并在同一进程内启动、停止和管理 Xray instance。

这不是外部 `xray` 进程包装器，也不把 Xray 配置作为主路径落盘。Plugin 相关路由只保留 Panel-facing contract adapter，避免 Panel 调用时返回 404，但不会产生官方 plugin side effects。

## 导航

- [一眼看懂](#一眼看懂)
- [能力快照](#能力快照)
- [快速开始](#快速开始)
- [运行配置](#运行配置)
- [运行结构](#运行结构)
- [目录导航](#目录导航)

## 一眼看懂

| 维度 | 当前状态 |
| --- | --- |
| 面向对象 | Remnawave Panel 和需要兼容官方 Node contract 的部署环境 |
| 当前主线 | 内嵌 `xray-core`、Gin HTTP 层、Panel-facing contract、真实 Panel live harness |
| 明确支持 | 主 API、internal API、Vision route、基础统计、用户管理、Xray 生命周期 |
| 明确降级 | conntrack、系统能力、Xray feature 读取失败时会稳定退化 |
| 明确不做 | 外部 `xray` 进程、内部 gRPC inbound、internal mTLS、plugin 运行时状态、nftables 真实现 |

详细进度矩阵见 [docs/roadmap.md](docs/roadmap.md)。

## 公开面

| 面向 | 入口 | 说明 |
| --- | --- | --- |
| 主 API | `NODE_PORT` | 面向 Panel 的主服务，走 HTTPS、mTLS 和可选 JWT 校验。 |
| Internal API | `INTERNAL_REST_PORT` | 仅本机可见的 internal REST API。 |
| Vision | `/vision/block-ip`、`/vision/unblock-ip` | 官方主 API 上的 unprefixed Panel-facing route。 |
| Live Harness | `scripts/panel-integration.sh` | 唯一真实 Panel 联调入口。 |
| Contract Drift | `mise run contract-diff` | 对照官方 `remnawave/node` 2.7.x 的 contract 变化。 |

## 能力快照

<details open>
<summary>当前仓库已经覆盖</summary>

- Gin HTTP 层、公开路由注册、contract struct、response envelope。
- `SECRET_KEY` 解析、PEM normalize、mTLS、JWT RS256、zstd request body。
- `/node/xray/start`、`/node/xray/stop`、`/node/xray/healthcheck` 的内嵌 Xray 生命周期。
- handler、stats、Vision 和连接清理的部分接入。
- Stats online status/IP 通过内嵌 Xray stats `OnlineMap` 读取，失败时稳定降级为 `false` 或空列表。
- Docker 构建、CI、release 流程和真实 Panel live harness。

</details>

<details>
<summary>当前仍然明确不做</summary>

- 外部 `xray` 进程模式。
- Xray 配置落盘主路径。
- 内部 gRPC API inbound。
- plugin 运行时能力和状态持久化。
- nftables 真执行。

</details>

## 快速开始

本地开发最短路径：

```sh
mise install
mise run test
mise run build
```

启动本地服务：

```sh
NODE_PORT=2222 INTERNAL_REST_PORT=61001 mise exec -- go run ./cmd/rw-node-go
```

开发模式下不设置 `SECRET_KEY` 时，主服务会以本地 HTTP 模式启动，便于 route 和 contract 测试。生产部署应提供 `SECRET_KEY`，或使用默认启用 `REQUIRE_SECRET_KEY=true` 的 Docker 镜像，让缺少密钥的容器直接启动失败。

发布前验证：

```sh
mise run preflight
```

真实 Panel live harness 只通过 `scripts/panel-integration.sh` 触发。该流程会启用并禁用真实 Panel 测试节点，只能指向专用测试节点。

<details>
<summary>推荐的本地验证顺序</summary>

```sh
mise run fmt
mise run test
mise run build
mise run contract-diff
```

Docker 相关改动再补：

```sh
mise run docker-build
```

</details>

## 运行配置

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `NODE_PORT` | `2222` | Panel 访问节点 API 的端口。 |
| `INTERNAL_REST_PORT` | `61001` | 本机 internal REST API 端口，只监听 `127.0.0.1`。 |
| `SECRET_KEY` | 空 | 官方 Node 使用的 base64 JSON 密钥包；设置后启用 mTLS/JWT。 |
| `REQUIRE_SECRET_KEY` | `false` | 裸进程默认允许本地开发 HTTP；Docker 镜像默认设为 `true`。 |
| `RW_NODE_DIR` | `/opt/rw-node-go` | 节点运行目录预留入口。 |
| `LOG_LEVEL` | `info` | 日志级别。 |
| `REQUEST_BODY_LIMIT_BYTES` | `1073741824` | request body 上限，默认 1 GiB。 |
| `XRAY_LOCATION_ASSET` | 空 | Xray geodata 目录；Docker 镜像固定设置为 `/usr/local/share/xray`。 |

`INTERNAL_REST_PORT` 只允许本机访问，不要通过 Docker publish、防火墙、FRP 或 PaaS 入站暴露到公网。

## 运行结构

```mermaid
flowchart TD
    panel[Remnawave Panel] -->|HTTPS + mTLS + Bearer JWT| main[Main Gin API on 0.0.0.0:NODE_PORT]
    main --> state[Controller + runtime state]
    state --> xray[Embedded xray-core instance]
    xray --> runtime[Xray runtime in the same Go process]
    tooling[Local tooling] -->|HTTP on 127.0.0.1:INTERNAL_REST_PORT| internal[Internal Gin API]
```

<details>
<summary>运行边界</summary>

```text
Panel-facing contract
    -> main API
        -> controller
            -> embedded xray-core
                -> runtime feature and system fallback

local-only control plane
    -> internal API
```

</details>

设置 `SECRET_KEY` 后，主 API 通过 TLS server config、mTLS 和 JWT public key 校验 Panel 请求。官方 `/vision/*` route 仍走主 API 的 HTTPS/mTLS，但按官方 2.7.0 行为豁免 Bearer JWT。

不设置 `SECRET_KEY` 时，主 API 以本地 HTTP 模式启动，只用于开发和 contract 测试。Docker 镜像默认要求 `SECRET_KEY`。

## 目录导航

- [docs/architecture.md](docs/architecture.md)：架构分层、运行路径、运行时边界和 internal API 边界。
- [docs/contracts.md](docs/contracts.md)：Panel-facing contract 对齐、route 覆盖、stub 策略、golden fixture 和 contract drift 检查。
- [docs/development.md](docs/development.md)：本地开发、验证命令、真实 Panel harness、版本发布和实现规则。
- [docs/roadmap.md](docs/roadmap.md)：功能路线图和详细完成情况。
