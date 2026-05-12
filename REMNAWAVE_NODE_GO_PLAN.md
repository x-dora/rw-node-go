# Remnawave Node Go 自研实现方案

> 历史设计备忘：本文档保留早期外部 Xray 进程、内部 gRPC API、plugin 和可选内嵌模式的完整讨论，不再作为当前实现规范。当前协作规则、运行边界、contract 要求和功能状态以 `AGENTS.md`、`README.md`、`docs/*.md` 和实际代码为准。本文中关于外部 `xray` 进程、内部 gRPC、nftables 或未来能力的旧设想不应作为当前实现依据。

本文档整理一个用 Go 自研 Remnawave Node 兼容实现的技术方案。目标不是简单复刻已有的 `hteppl/remnawave-node-go`，而是以官方 `remnawave/node` 当前协议和行为为基准，做一个可维护、可测试、可长期跟随上游变化的轻量节点实现。

## 1. 背景与目标

RW-Node 当前是官方 `remnawave/node` 的轻量部署和打包方案，本仓库本身不包含 Node.js 应用源码。官方节点是 TypeScript/NestJS 应用，负责接收 Remnawave Panel 的节点控制请求，并管理本机 Xray-core。

社区已有 Go 版本 `hteppl/remnawave-node-go`，但它目前存在几个问题：

- 官方 Remnawave Node 已更新到 `2.7.0`，Go 版 README 中也说明 `v1.3.0+` 才计划兼容 Remnawave `2.7.0+`，正式兼容版尚未稳定。
- Go 版缺少 2.7.0 新增的插件、metadata、system stats 等完整行为。
- Go 版采用内嵌 `xray-core` 的路线，镜像和内存收益明显，但会增加对 Xray 内部 API 的耦合。

自研目标：

- 与官方 `remnawave/node` 的 Panel-facing API 保持兼容。
- 先使用外部 `xray` 进程 + Xray gRPC API，降低维护风险。
- 保留后续切换为内嵌 `xray-core` 的扩展空间。
- 镜像小、启动快、依赖少，适合 VPS、PaaS、边缘节点部署。
- 有清晰的兼容测试体系，避免上游协议变化时靠人工试错。

非目标：

- 不重新实现 Remnawave Panel。
- 不修改 Remnawave 用户协议、订阅协议或面板业务逻辑。
- 第一版不追求替代官方 Node 的全部高级插件能力，但所有 Panel 可能调用的接口必须至少兼容返回。

## 2. 官方 Node 当前形态

官方 Node 不是 Socket.IO 节点，而是 HTTPS REST 服务：

```text
Remnawave Panel
    |
    | HTTPS + mTLS + Bearer JWT RS256
    v
Remnawave Node API
    |
    | Xray gRPC API / process control
    v
Xray-core
```

核心点：

- Panel 通过节点的 `NODE_PORT` 访问 `/node/...` API。
- API 服务启用 mTLS，节点证书、节点私钥、CA 证书和 JWT 公钥来自 `SECRET_KEY`。
- 每个 `/node/...` 请求还需要 `Authorization: Bearer <jwt>`，JWT 使用 `RS256` 和 `jwtPublicKey` 验签。
- Node 收到 `/node/xray/start` 后，基于 Panel 下发的 `xrayConfig` 注入 Remnawave 内部 API inbound、stats、policy、routing，然后启动或重启 Xray。
- Node 后续通过 Xray gRPC 的 HandlerService、StatsService、RoutingService 动态增删用户、读取统计、控制路由。

官方 2.7.0 的重要新增点：

- 插件接口：`/node/plugin/...`
- torrent blocker 相关 webhook 和报告收集。
- nftables 相关封禁接口。
- system stats 和 interface stats 更完整。
- `SECRET_KEY` 取代旧的证书变量形式。

## 3. 兼容边界

### 3.1 环境变量

建议保持和官方 Node 接近，减少部署差异：

```env
NODE_PORT=2222
SECRET_KEY=...
XTLS_API_PORT=61000
LOG_LEVEL=info
RW_NODE_DIR=/opt/rw-node-go
INTERNAL_SOCKET_PATH=/tmp/remnawave-node.sock
INTERNAL_REST_TOKEN=<random>
DISABLE_HASHED_SET_CHECK=false
```

可选扩展：

```env
XRAY_BIN=/usr/local/bin/xray
XRAY_CONFIG_PATH=/opt/rw-node-go/xray/config.json
XRAY_ASSET_DIR=/usr/local/share/xray
INTERNAL_REST_PORT=61001
ENABLE_UNIX_SOCKET_INTERNAL=true
ENABLE_PLUGIN_STUBS=true
```

说明：

- `NODE_PORT` 是 Panel 调 Node API 的端口，不是代理入站端口。
- `XTLS_API_PORT` 只给 Node 控制器和本机 Xray gRPC 通信，不应暴露到公网、Docker 端口映射、FRP 或防火墙。
- `SECRET_KEY` 是 base64 编码 JSON，至少包含 `caCertPem`、`jwtPublicKey`、`nodeCertPem`、`nodeKeyPem`。
- `DISABLE_HASHED_SET_CHECK` 用于跳过 hash 比较，强制按请求重启 Xray。

### 3.2 `SECRET_KEY` 格式

解码流程：

```text
SECRET_KEY
  -> base64 decode
  -> JSON parse
  -> normalize PEM newline
  -> extract:
       caCertPem
       jwtPublicKey
       nodeCertPem
       nodeKeyPem
```

Go 侧数据结构：

```go
type NodePayload struct {
    CACertPEM    string `json:"caCertPem"`
    JWTPublicKey string `json:"jwtPublicKey"`
    NodeCertPEM  string `json:"nodeCertPem"`
    NodeKeyPEM   string `json:"nodeKeyPem"`
}
```

PEM normalize 要兼容：

- `\n` 字符串转真实换行。
- Windows `\r\n` 转 `\n`。
- `BEGIN/END` 头尾缺换行时补齐。
- 多余空行压缩。

### 3.3 HTTP 与认证

主 API 服务：

- 监听 `0.0.0.0:NODE_PORT`。
- 启用 HTTPS。
- 服务端证书使用 `nodeCertPem/nodeKeyPem`。
- Client CA 使用 `caCertPem`。
- `ClientAuth` 使用强制校验客户端证书。
- 校验 JWT：
  - Header: `Authorization: Bearer <token>`
  - Algorithm: `RS256`
  - Public key: `jwtPublicKey`

认证失败行为：

- mTLS 失败由 TLS 层拒绝。
- JWT 失败建议直接关闭连接或返回 401。为贴近官方行为，未知路径和部分非法访问可以直接 destroy socket。
- 业务失败多数保持 HTTP 200，通过 `response` 内容表达失败。

请求体：

- 支持普通 JSON。
- 支持 zstd 压缩 JSON。官方 Node 使用 zstd body parser，Panel 未来或当前可能发送 zstd request body。
- body size 上限要足够大，官方为 `1000mb`，Go 侧建议默认 `1GiB`，可配置。

响应：

大多数接口必须使用 envelope：

```json
{
  "response": {}
}
```

不要随意改变字段名、null/空数组、GET/POST 方法。

## 4. 必须实现的外部 API

### 4.1 Xray 控制

```text
POST /node/xray/start
GET  /node/xray/stop
GET  /node/xray/healthcheck
```

`POST /node/xray/start` 请求：

```json
{
  "internals": {
    "forceRestart": false,
    "hashes": {
      "emptyConfig": "hash",
      "inbounds": [
        {
          "usersCount": 1,
          "hash": "hash",
          "tag": "VLESS_INBOUND"
        }
      ]
    }
  },
  "xrayConfig": {}
}
```

响应：

```json
{
  "response": {
    "isStarted": true,
    "version": "25.1.1",
    "error": null,
    "nodeInformation": {
      "version": "0.1.0"
    },
    "system": {
      "info": {},
      "stats": {},
      "interface": {}
    }
  }
}
```

`GET /node/xray/stop` 响应：

```json
{
  "response": {
    "isStopped": true
  }
}
```

`GET /node/xray/healthcheck` 响应：

```json
{
  "response": {
    "isAlive": true,
    "xrayInternalStatusCached": true,
    "xrayVersion": "25.1.1",
    "nodeVersion": "0.1.0"
  }
}
```

### 4.2 用户管理

```text
POST /node/handler/add-user
POST /node/handler/add-users
POST /node/handler/remove-user
POST /node/handler/remove-users
POST /node/handler/get-inbound-users
POST /node/handler/get-inbound-users-count
POST /node/handler/drop-users-connections
POST /node/handler/drop-ips
```

协议类型必须覆盖：

- `vless`
- `trojan`
- `shadowsocks`
- `shadowsocks22`
- `hysteria`

`add-user` 要注意官方行为：

- 先记录请求涉及的 inbound tag。
- 从所有已知 inbound 中移除同名用户，避免残留。
- 如果有 `prevVlessUuid`，从本地 hash set 中移除旧 uuid，否则移除当前 `vlessUuid`。
- 再按请求把用户添加到目标 inbound。
- 添加成功后更新本地 inbound -> user hash 状态。

`add-users` 要注意：

- `affectedInboundTags` 先加入已知 inbound 集合。
- 每个用户先从所有已知 inbound 移除。
- 再按 `inboundData` 添加。
- `shadowsocks22` 的 key/密码处理要和官方保持一致。

统一成功响应：

```json
{
  "response": {
    "success": true,
    "error": null
  }
}
```

获取 inbound 用户数：

```json
{
  "response": {
    "count": 10
  }
}
```

获取 inbound 用户：

```json
{
  "response": {
    "users": [
      {
        "username": "user-id",
        "email": "user-id",
        "level": 0
      }
    ]
  }
}
```

### 4.3 统计

```text
GET  /node/stats/get-system-stats
POST /node/stats/get-users-stats
POST /node/stats/get-user-online-status
POST /node/stats/get-user-ip-list
GET  /node/stats/get-users-ip-list
POST /node/stats/get-inbound-stats
POST /node/stats/get-outbound-stats
POST /node/stats/get-all-inbounds-stats
POST /node/stats/get-all-outbounds-stats
POST /node/stats/get-combined-stats
```

Xray stats 名称格式：

```text
user>>>USERNAME>>>traffic>>>uplink
user>>>USERNAME>>>traffic>>>downlink
user>>>USERNAME>>>online

inbound>>>TAG>>>traffic>>>uplink
inbound>>>TAG>>>traffic>>>downlink

outbound>>>TAG>>>traffic>>>uplink
outbound>>>TAG>>>traffic>>>downlink
```

`get-users-stats` 响应：

```json
{
  "response": {
    "users": [
      {
        "username": "user-id",
        "downlink": 1024,
        "uplink": 2048
      }
    ]
  }
}
```

流量为 0 的用户建议过滤，保持官方行为。

`get-user-online-status` 响应：

```json
{
  "response": {
    "isOnline": true
  }
}
```

`get-user-ip-list` 响应：

```json
{
  "response": {
    "ips": [
      {
        "ip": "203.0.113.1",
        "lastSeen": "2026-05-05T12:00:00.000Z"
      }
    ]
  }
}
```

`get-users-ip-list` 是 GET，响应：

```json
{
  "response": {
    "users": [
      {
        "userId": "user-id",
        "ips": [
          {
            "ip": "203.0.113.1",
            "lastSeen": "2026-05-05T12:00:00.000Z"
          }
        ]
      }
    ]
  }
}
```

在线 IP 依赖 Xray 的 online stats 能力和系统权限。没有权限时应该返回空数组或 false，不要让接口报错。

### 4.4 Vision 与连接处理

官方 contract 中有：

```text
POST /vision/block-ip
POST /vision/unblock-ip
```

请求：

```json
{
  "ip": "203.0.113.1",
  "username": "user-id"
}
```

响应：

```json
{
  "response": {
    "success": true,
    "error": null
  }
}
```

连接踢出相关：

```text
POST /node/handler/drop-users-connections
POST /node/handler/drop-ips
```

实现选项：

- 优先用 netlink/conntrack 删除连接。
- 也可以调用 `conntrack -D`，但会增加容器依赖。
- 有 nftables 时可将 IP 加入 set 并配置 timeout。
- 没有权限时返回成功并记录 warning，避免 Panel 卡住。

### 4.5 插件接口

Remnawave 2.7.0 后需要关注插件接口：

```text
POST /node/plugin/sync
POST /node/plugin/torrent-blocker/collect
POST /node/plugin/nftables/block-ips
POST /node/plugin/nftables/unblock-ips
POST /node/plugin/nftables/recreate-tables
```

第一版至少要提供兼容 stub，不能 404。

`sync` 请求：

```json
{
  "plugin": {
    "config": {},
    "uuid": "00000000-0000-0000-0000-000000000000",
    "name": "torrent-blocker"
  }
}
```

或：

```json
{
  "plugin": null
}
```

响应：

```json
{
  "response": {
    "accepted": true
  }
}
```

`torrent-blocker/collect` 响应：

```json
{
  "response": {
    "reports": []
  }
}
```

`nftables/*` 响应：

```json
{
  "response": {
    "accepted": true
  }
}
```

后续完整实现 torrent blocker：

- `plugin/sync` 记录插件启用状态和配置。
- `start` 生成 Xray config 时，如果 torrent blocker 启用，注入 blackhole outbound 和 bittorrent routing rule。
- routing rule webhook 指向内部 `/internal/webhook`。
- webhook 收集报告并去重。
- `collect` 被 Panel 拉取后返回报告并按策略清理。

后续完整实现 nftables：

- 根据 `block-ips` 请求创建/更新 nft set。
- 支持 timeout。
- `unblock-ips` 从 set 删除。
- `recreate-tables` 重建表和链。

## 5. 内部 API

建议支持两种内部通道：

```text
Unix socket: /tmp/remnawave-node.sock
TCP: 127.0.0.1:61001
```

内部路由：

```text
GET  /internal/get-config
POST /internal/webhook
```

`get-config` 用于调试当前 Xray 完整配置，默认只监听本机或 Unix socket，不对公网开放。

`webhook` 用于接收 Xray routing webhook，例如 torrent blocker 报告。

如果使用 TCP 内部端口，必须：

- 只监听 `127.0.0.1`。
- 可选加 `?token=<INTERNAL_REST_TOKEN>`。
- 不暴露到 Docker publish、FRP、PaaS 入站或公网。

## 6. Xray 配置生成

收到 `/node/xray/start` 后，Go 程序不能直接原样启动 Panel 下发的 `xrayConfig`，必须注入 Remnawave 内部控制能力。

注入内容：

```json
{
  "stats": {},
  "api": {
    "services": ["HandlerService", "StatsService", "RoutingService"],
    "tag": "REMNAWAVE_API"
  }
}
```

注入 API inbound：

```json
{
  "tag": "REMNAWAVE_API_INBOUND",
  "port": 61000,
  "listen": "127.0.0.1",
  "protocol": "dokodemo-door",
  "settings": {
    "address": "127.0.0.1"
  },
  "streamSettings": {
    "security": "tls",
    "tlsSettings": {
      "alpn": ["h2"],
      "serverName": "internal.remnawave.local",
      "disableSystemRoot": true,
      "rejectUnknownSni": true,
      "certificates": []
    }
  }
}
```

注入 routing：

```json
{
  "inboundTag": ["REMNAWAVE_API_INBOUND"],
  "outboundTag": "REMNAWAVE_API"
}
```

注入 policy：

```json
{
  "policy": {
    "levels": {
      "0": {
        "statsUserUplink": true,
        "statsUserDownlink": true,
        "statsUserOnline": true
      }
    },
    "system": {
      "statsInboundDownlink": true,
      "statsInboundUplink": true,
      "statsOutboundDownlink": true,
      "statsOutboundUplink": true
    }
  }
}
```

注意：

- `statsUserOnline` 最好按 `CAP_NET_ADMIN` 能力动态设置。
- 如果用户原配置已有 policy，要合并，不应覆盖其他用户配置。
- 如果用户原配置已有 routing/outbounds/inbounds，要保留并追加。
- API inbound 必须排在 inbounds 前面也可以，但 tag 不能冲突。
- 配置写入文件前应 JSON marshal 并可选 pretty print 到内部 debug API。

## 7. Xray 运行模式

### 7.1 推荐第一版：外部进程模式

流程：

```text
Panel POST /node/xray/start
  -> 校验请求
  -> 判断 hash 是否需要重启
  -> 生成 full xray config
  -> 写入 config.json
  -> stop old xray if running
  -> start xray -config config.json
  -> 等待 gRPC StatsService 可用
  -> 返回 isStarted=true
```

优点：

- 最接近官方 Node 架构。
- Xray 可独立升级，不需要重新编译 Go 程序。
- 避免和 `xray-core` 内部 API 强耦合。
- 更容易定位故障，用户可以直接运行 `xray -test -config`。

缺点：

- 多一个进程。
- 镜像略大。
- 需要管理进程生命周期和日志。

### 7.2 后续可选：内嵌 xray-core 模式

优点：

- 单二进制。
- 镜像和内存可能更小。
- 没有进程管理成本。

风险：

- `xray-core` 内部 feature API 变动可能破坏构建。
- geodata 初始化、DNS、router、stats、inbound manager 生命周期都要自己处理。
- 不同平台构建和依赖更复杂。

建议在外部进程模式稳定后再做实验分支。

## 8. Go 项目结构建议

```text
cmd/rw-node-go/main.go

internal/config
  env.go
  secret.go
  paths.go

internal/httpapi
  server.go
  tls.go
  jwt.go
  zstd.go
  response.go
  router.go

internal/contracts
  xray.go
  handler.go
  stats.go
  vision.go
  plugin.go
  errors.go

internal/controller
  xray_controller.go
  handler_controller.go
  stats_controller.go
  vision_controller.go
  plugin_controller.go
  internal_controller.go

internal/xray
  process.go
  config.go
  grpc_client.go
  handler.go
  stats.go
  routing.go
  users.go
  health.go
  version.go

internal/state
  hashes.go
  inbounds.go
  runtime.go
  plugin.go

internal/system
  stats.go
  network.go
  netadmin.go
  conntrack.go
  nftables.go

internal/plugin
  sync.go
  torrent_blocker.go
  nftables.go
  reports.go

internal/testkit
  certs.go
  jwt.go
  panel_client.go
  golden.go
```

依赖建议：

- HTTP router：`net/http` + `chi` 或 `gin`。如果追求最少依赖，用 `chi` 即可。
- JWT：`github.com/golang-jwt/jwt/v5`。
- zstd：`github.com/klauspost/compress/zstd`。
- system stats：`github.com/shirou/gopsutil/v4`。
- Xray gRPC：优先用 Xray-core protobuf 生成的 client 或直接复用其 Go module 中的 generated pb。
- nftables：`github.com/google/nftables`。
- conntrack/netlink：可评估 `github.com/florianl/go-conntrack` 或直接使用 netlink 库。

## 9. 状态管理

需要持有以下运行时状态：

```go
type RuntimeState struct {
    XrayRunning bool
    XrayVersion string
    NodeVersion string
    CurrentConfig map[string]any
    LastHashes Hashes
    InboundUsers map[string]map[string]struct{}
    KnownInboundTags map[string]struct{}
    Plugins PluginState
}
```

hash 判断：

```go
type Hashes struct {
    EmptyConfig string        `json:"emptyConfig"`
    Inbounds    []InboundHash `json:"inbounds"`
}

type InboundHash struct {
    UsersCount int    `json:"usersCount"`
    Hash       string `json:"hash"`
    Tag        string `json:"tag"`
}
```

`IsNeedRestartCore` 逻辑：

- 如果没有旧 hash，必须重启。
- 如果 `emptyConfig` 变化，必须重启。
- 如果 inbound tag 集合变化，必须重启。
- 如果某个 inbound hash 或 usersCount 变化，要结合本地动态用户状态判断。
- 第一版可以保守：只要任一 hash 变化就重启。后续再优化减少重启。

本地状态是否需要持久化：

- 第一版可以只放内存，Xray 重启时由 Panel 再同步。
- 若要支持进程重启后的快速恢复，可将 last config 和 hashes 写到 `RW_NODE_DIR/state.json`。
- 持久化文件不得包含 Panel JWT、证书私钥以外的敏感扩展；如果包含完整 `SECRET_KEY` 或私钥，必须设权限 `0600`。

## 10. 错误处理策略

保持官方风格：

- 认证失败：拒绝请求。
- 未知路由：尽量关闭连接或返回官方类似 404 文本。
- 业务错误：多数返回 HTTP 200，加 `response.error` 或 `response.success=false`。
- Xray start 失败：返回 `isStarted=false` 和错误字符串，不要让 Panel 只看到 HTTP 500。
- Stats 获取失败：按接口返回空数组、0 或 false，必要时返回官方错误码结构。

示例：

```json
{
  "response": {
    "success": false,
    "error": "xray core not running"
  }
}
```

日志：

- 默认 info。
- debug 时打印路由、inbound tag、用户数量、hash 变化原因。
- 不打印完整 `SECRET_KEY`、私钥、JWT。
- 用户 ID 是否敏感取决于部署场景，建议 debug 才打印。

## 11. 安全设计

必须做到：

- 主 API 强制 HTTPS+mTLS。
- JWT 只接受 `RS256`。
- `SECRET_KEY` 解码失败直接启动失败。
- Xray gRPC API 只监听 `127.0.0.1`。
- 内部 REST/Unix socket 不暴露公网。
- `XTLS_API_PORT` 不通过 Docker publish。
- Docker 默认非特权运行；需要在线 IP 和连接踢出时再授予 `CAP_NET_ADMIN`。
- nftables/conntrack 能力缺失时降级，不应让核心代理不可用。

容器权限建议：

```yaml
cap_add:
  - NET_ADMIN
  - NET_RAW
```

如果不需要在线 IP、连接踢出和 nftables，可以不加。

防火墙建议：

- `NODE_PORT` 只允许 Remnawave Panel IP 访问。
- 用户代理端口按实际 inbound 暴露。
- `XTLS_API_PORT`、内部 socket/port 不允许外部访问。

## 12. Docker 镜像设计

第一版镜像：

```text
rw-node-go:latest
  /usr/local/bin/rw-node-go
  /usr/local/bin/xray
  /usr/local/share/xray/geoip.dat
  /usr/local/share/xray/geosite.dat
```

基础镜像：

- 构建阶段：`golang:<version>-alpine` 或 Debian slim。
- 运行阶段：`alpine` 或 `gcr.io/distroless/base-debian12`。
- 如果需要 shell entrypoint 和调试，优先 alpine。

运行入口：

```sh
exec /usr/local/bin/rw-node-go
```

镜像 tag：

```text
ghcr.io/x-dora/rw-node-go:latest
ghcr.io/x-dora/rw-node-go:<version>
ghcr.io/x-dora/rw-node-go:latest-embedded
```

如果要接入当前仓库：

```text
docker/Dockerfile.go
docker/docker-entrypoint.go.sh
.github/workflows/docker-build-go.yml
```

建议先不要改现有 `release.yml` 主流程，等 Go 节点兼容性稳定后再并入。

## 13. 测试方案

### 13.1 Contract golden tests

从官方 `remnawave/node` 的 `libs/contract/commands` 整理请求/响应样例，固化为 golden JSON。

覆盖：

- 路由 method/path。
- 请求字段和可选字段。
- 响应 envelope。
- null、空数组、日期格式。
- 错误响应。

### 13.2 mTLS/JWT 集成测试

测试工具生成：

- CA cert/key。
- Node cert/key。
- Panel client cert/key。
- JWT RSA key pair。
- `SECRET_KEY`。

测试：

- 无 client cert 被拒。
- 错误 CA 被拒。
- 无 JWT 被拒。
- 错误 alg JWT 被拒。
- 正确 mTLS + JWT 能访问 healthcheck。

### 13.3 Xray 集成测试

使用真实 `xray` 二进制：

- 下发最小 VLESS inbound config。
- 调 `/node/xray/start`。
- 等待 gRPC StatsService 可用。
- 调 `/node/xray/healthcheck`。
- 调 `/node/xray/stop`。

### 13.4 用户管理测试

覆盖：

- add vless user。
- add trojan user。
- add shadowsocks user。
- add shadowsocks22 user。
- add hysteria user。
- bulk add。
- remove single。
- bulk remove。
- get inbound users。
- get inbound users count。

验证方式：

- 通过 Xray HandlerService 查询 inbound。
- 或用客户端实际连接测试。

### 13.5 Stats 测试

覆盖：

- users stats reset=false。
- users stats reset=true。
- inbound/outbound stats。
- combined stats。
- online status。
- user IP list。

如果在线 IP 能力依赖权限，CI 可分两组：

- 无 `CAP_NET_ADMIN`：期望空列表/false。
- 有 `CAP_NET_ADMIN`：期望能看到 IP。

### 13.6 Plugin 测试

第一阶段：

- 所有 `/node/plugin/...` 不返回 404。
- `sync` 返回 accepted。
- `collect` 返回 reports 数组。

完整阶段：

- torrent blocker 开启后，生成配置包含 bittorrent routing rule 和 blackhole outbound。
- webhook 能收集报告。
- collect 能返回报告并清理。
- nftables block/unblock/recreate 操作真实 nft 表。

### 13.7 对比测试

同一套请求分别打：

- 官方 `remnawave/node:2.7.0`
- 自研 `rw-node-go`

比较：

- HTTP status。
- JSON schema。
- 关键字段。
- 失败行为。

不要要求日志完全一致。

## 14. 开发里程碑

### M0: 协议冻结

周期：1-2 天。

产出：

- 从官方 `2.7.0` 提取 contract。
- 建立 Go structs。
- 建立 golden JSON。
- 明确第一版支持矩阵。

验收：

- 所有 contract tests 编译通过。
- 文档列出未实现项和 stub 项。

### M1: 节点握手与 Xray 生命周期

周期：3-5 天。

产出：

- `SECRET_KEY` 解析。
- HTTPS mTLS。
- JWT RS256。
- zstd body。
- `/node/xray/healthcheck`。
- `/node/xray/start`。
- `/node/xray/stop`。
- 外部 xray 进程管理。
- full config 生成和内部 API inbound 注入。

验收：

- 真实 Panel 能添加节点。
- Panel 能看到节点 alive。
- Xray 能启动并通过内部 gRPC 健康检查。

### M2: 用户动态管理

周期：5-7 天。

产出：

- `add-user`
- `add-users`
- `remove-user`
- `remove-users`
- `get-inbound-users`
- `get-inbound-users-count`
- VLESS/Trojan/Shadowsocks/Shadowsocks2022/Hysteria user builder。
- inbound/user hash 状态管理。

验收：

- Panel 添加/删除用户后，用户能实际连接。
- inbound 用户数正确。
- 变更用户 inbound 不残留旧 inbound。

### M3: 基础统计

周期：4-6 天。

产出：

- `get-users-stats`
- `get-system-stats`
- inbound/outbound stats。
- combined stats。
- reset 语义。
- system CPU/mem/disk/net/interface stats。

验收：

- Panel dashboard 有基础流量和系统数据。
- reset 后流量不重复上报。

### M4: 在线 IP 与连接踢出

周期：3-6 天。

产出：

- `get-user-online-status`
- `get-user-ip-list`
- `get-users-ip-list`
- `drop-users-connections`
- `drop-ips`
- Vision block/unblock。

验收：

- 有权限环境能看到在线 IP。
- drop IP/user 能断开现有连接。
- 无权限环境稳定降级。

### M5: 插件兼容

周期：4-8 天。

产出：

- `plugin/sync`
- torrent blocker config injection。
- internal webhook。
- `torrent-blocker/collect`。
- nftables block/unblock/recreate。

验收：

- Remnawave 2.7.0 插件相关调用无报错。
- torrent blocker 报告能被收集和上报。
- nftables 封禁可生效。

### M6: 发布与跟随上游

周期：持续。

产出：

- Docker multi-arch。
- GitHub Actions。
- Renovate 或脚本跟踪官方 Node release。
- contract diff 检测。
- 回归测试矩阵。

验收：

- 每次官方 Node 更新后能自动提示 contract 变化。
- Go 节点镜像可独立发布。

## 15. 风险与应对

### 15.1 上游 contract 变化

风险：

- Remnawave Panel 与 Node 的 API 不是稳定公开标准，可能随版本变化。

应对：

- 固定支持矩阵，例如 `Panel 2.7.x -> rw-node-go 0.1.x`。
- CI 自动拉官方 `remnawave/node` 最新 tag，diff `libs/contract`。
- contract 变化时先失败 CI，再人工适配。

### 15.2 Xray gRPC API 变化

风险：

- Xray Handler/Stats/Routing protobuf 或行为变化。

应对：

- 外部进程模式下锁定 Xray 版本。
- Docker tag 同时标注 rw-node-go 和 Xray 版本。
- 每次升级 Xray 跑完整集成测试。

### 15.3 在线 IP 和踢连接依赖权限

风险：

- PaaS 或普通容器没有 `CAP_NET_ADMIN`。

应对：

- 启动时检测能力。
- 没权限时 `statsUserOnline=false` 或接口返回空。
- 日志明确提示，但不影响代理主功能。

### 15.4 插件能力不完整

风险：

- Panel 2.7.0 调插件接口时 404 或 schema 不匹配。

应对：

- 第一版就提供 plugin stub。
- 后续逐步实现真实 torrent blocker 和 nftables。

### 15.5 内嵌 xray-core 维护成本

风险：

- 内嵌路线短期看起来更轻，但长期可能被 Xray 内部变化拖累。

应对：

- 第一版不内嵌。
- 把 Xray 控制抽象成 interface：

```go
type Core interface {
    Start(config []byte) error
    Stop() error
    IsRunning() bool
    Version() string
    Handler() HandlerClient
    Stats() StatsClient
}
```

以后可以增加 `EmbeddedCore`，不影响 HTTP contract 层。

## 16. 最小可用版本范围

MVP 建议包含：

- mTLS/JWT。
- Xray start/stop/healthcheck。
- VLESS/Trojan/Shadowsocks/Shadowsocks2022/Hysteria 添加删除。
- 用户和 inbound/outbound 基础统计。
- plugin stub。
- Docker 镜像。

MVP 暂不要求：

- 完整 torrent blocker。
- 完整 nftables。
- 完整在线 IP。
- 内嵌 xray-core。
- 高级 observability。

这样可以最快验证“Panel 能管理、用户能连接、流量能上报”这三个核心问题。

## 17. 与当前 RW-Node 仓库的关系

当前仓库主要做官方 Node 的轻量打包，不包含上游源码。Go 节点建议先独立开发，稳定后再接入本仓库：

第一阶段：

- 新增方案文档。
- 不改现有 release/build 流程。

第二阶段：

- 新增 `docker/Dockerfile.go`。
- 新增 `docker/docker-entrypoint.go.sh`。
- 新增单独 workflow `docker-build-go.yml`。
- 发布独立镜像 tag，例如 `latest-go`。

第三阶段：

- README 中增加 Go 实验版说明。
- 安装脚本增加可选参数，例如 `--variant go`。
- PaaS FRP 版如需支持，再增加 `latest-go-paas-frp`。

不建议一开始替换现有 `latest`，避免影响已有用户。

## 18. 推荐实现顺序

最终推荐路线：

```text
1. 建立独立 Go repo 或当前仓库子目录实验工程
2. 抽官方 2.7.0 contract -> Go structs -> golden tests
3. 实现 mTLS/JWT/zstd/response envelope
4. 实现 Xray config 注入和外部进程生命周期
5. 接真实 Panel 验证 start/healthcheck
6. 实现用户 add/remove/bulk
7. 实现 stats
8. plugin stub 全覆盖
9. Docker multi-arch
10. 再补 online IP、drop connection、torrent blocker、nftables
```

判断是否进入下一阶段的标准：

- 每个阶段必须能用真实 Panel 和真实 Xray 跑通。
- 每个接口必须有 contract test。
- 不靠“Panel 暂时没调用”来省略路由，所有官方 contract 中的路由至少要有兼容返回。

## 19. 参考资料

- 官方 Node 仓库：`https://github.com/remnawave/node`
- 社区 Go 版：`https://github.com/hteppl/remnawave-node-go`
- Remnawave Node 文档：`https://docs.rw/docs/install/remnawave-node/`
- Xray-core：`https://github.com/XTLS/Xray-core`
