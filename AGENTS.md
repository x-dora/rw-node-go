# Agent 协作说明

本仓库是 Remnawave Node 兼容服务的 Go 实现骨架，目标 contract 是官方 `remnawave/node` 2.7.x 面向 Panel 的 API。

## 当前边界

- 当前优先保持框架完整：公开路由、contract 类型、CI/CD、Docker 和文档必须稳定。
- 内部行为按 `REMNAWAVE_NODE_GO_PLAN.md` 的 M1-M6 逐步实现。
- stub 不能伪装成真实能力；未实现的 Xray、stats、plugin、nftables、conntrack 行为必须保持明确。

## 必须参考

- `REMNAWAVE_NODE_GO_PLAN.md` 是本地实现方案。
- `tmp/remnawave-node/libs/contract` 是官方 2.7.0 contract 参考。
- `tmp/remnawave-node-go` 只用于行为对照，不复制它的框架结构。
- 不要修改 `tmp/` 下的参考仓库。

## 编码规则

- HTTP 层使用 Gin。
- 保持公开 JSON 字段名、路由路径、HTTP method 和 response envelope 稳定。
- 优先标准库和必要的小依赖；新增依赖要有明确理由。
- 不要打印 `SECRET_KEY`、JWT、节点私钥、客户端证书或 bearer token。
- `XTLS_API_PORT` 和内部控制接口必须保持本机访问，不要在 Docker 示例里暴露。

## 测试要求

- 新增 route、公开 contract struct 或响应形状时必须补测试。
- 从 stub 进入真实行为实现后，应尽量补 integration test。
- contract 变化要对照官方 `remnawave/node` 和 golden fixture。

## 常用命令

- `mise run fmt`
- `mise run test`
- `mise run build`
- `mise run docker-build`
