# rw-node-go

`rw-node-go` 是一个用 Go 编写的 Remnawave Node 兼容实现骨架。目标是对齐官方 `remnawave/node` 2.7.x 面向 Panel 的 API，同时保持运行时轻量、结构清晰、便于后续持续跟随上游。

当前仓库处于框架阶段：路由、contract 类型、项目结构、CI/CD、Docker 和基础文档已经落地；真实 Xray 进程控制、Xray gRPC、用户管理、统计、torrent blocker、nftables 等内部行为暂未实现。

HTTP 层使用 Gin。`tmp/remnawave-node-go` 只作为行为参考，不沿用该社区实现的框架结构。

## 快速开始

```sh
mise install
mise run test
mise run build
```

启动本地 stub 服务：

```sh
NODE_PORT=2222 mise exec -- go run ./cmd/rw-node-go
```

框架会为计划内 API 返回兼容的 JSON envelope。`/node/xray/start` 当前会明确返回 `isStarted=false` 和 `not implemented`，直到 M1 的 Xray 生命周期实现完成。

## 环境变量

框架阶段只保留少量必要配置：

```env
NODE_PORT=2222
SECRET_KEY=
XTLS_API_PORT=61000
LOG_LEVEL=info
RW_NODE_DIR=/opt/rw-node-go
XRAY_BIN=/usr/local/bin/xray
```

说明：

- `NODE_PORT`：Panel 访问节点 API 的端口。
- `SECRET_KEY`：官方 Node 使用的 base64 JSON 密钥包，后续 M1 会用于 mTLS 和 JWT。
- `XTLS_API_PORT`：本机 Xray gRPC API 端口，只允许本机访问。
- `LOG_LEVEL`：日志级别，当前主要保留接口。
- `RW_NODE_DIR`：节点运行目录；Xray 配置默认派生为 `$RW_NODE_DIR/xray/config.json`。
- `XRAY_BIN`：外部 Xray 二进制路径。

内部 REST、Unix socket、插件开关、hash 跳过等选项先不公开，等对应功能实现后再加入文档。

## 镜像

Docker workflow 在 `main` 分支验证多架构构建，在 `v*` tag 发布时推送镜像：

```text
ghcr.io/x-dora/rw-node-go
```

当前镜像只包含 `rw-node-go` 二进制，并预留 Xray 运行目录；暂不下载或内置 Xray。

## 参考仓库

参考仓库放在 `tmp/`，只用于对照 contract 和行为：

```sh
git clone --depth 1 --branch 2.7.0 https://github.com/remnawave/node.git tmp/remnawave-node
git clone --depth 1 --branch master https://github.com/hteppl/remnawave-node-go.git tmp/remnawave-node-go
```

`tmp/` 已被 git 和 Docker 忽略。
