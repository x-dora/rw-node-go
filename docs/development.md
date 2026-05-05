# 开发说明

## 初始化参考仓库

```sh
git clone --depth 1 --branch 2.7.0 https://github.com/remnawave/node.git tmp/remnawave-node
git clone --depth 1 --branch master https://github.com/hteppl/remnawave-node-go.git tmp/remnawave-node-go
```

`tmp/` 只做本地参考，不要修改或提交。

## 常用命令

```sh
mise run fmt
mise run test
mise run build
mise run docker-build
```

`mise run lint` 需要本地已安装 `golangci-lint`。

## 本地运行

```sh
NODE_PORT=2222 mise exec -- go run ./cmd/rw-node-go
```

当前启动的是 HTTP stub 服务。mTLS/JWT 强校验会在 M1 实现。

## Docker

```sh
docker build -t ghcr.io/x-dora/rw-node-go:local .
```

Dockerfile 会编译 Go 程序并生成 Alpine 运行镜像。当前镜像不包含 Xray 二进制。
