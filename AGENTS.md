# Agent 协作说明

本仓库是 Remnawave Node 兼容服务的 Go 实现，目标 contract 是官方 `remnawave/node` 2.7.x 面向 Panel 的 API。协作时优先保持公开接口稳定，再逐步把 stub 替换为真实运行时能力。

## 当前阶段

- 已完成项目骨架、Gin HTTP 层、公开路由注册、contract struct、response envelope、CI 和 Docker 构建流程。
- 已完成 `SECRET_KEY` 解析、mTLS、JWT RS256、zstd request body、Xray config 注入和外部 Xray 进程启动/停止基础能力。
- `/node/xray/start`、`/node/xray/stop`、`/node/xray/healthcheck` 已接入进程控制，但 Xray gRPC Handler/Stats/Routing client 仍未实现。
- handler、stats、plugin、vision 相关 controller 当前主要是兼容 stub 或占位响应，不能写成真实能力。
- 后续实现顺序按 `REMNAWAVE_NODE_GO_PLAN.md` 和 `docs/roadmap.md` 推进；如果本地计划和官方仓库实现冲突，以官方仓库为准。

## 必须参考

- `REMNAWAVE_NODE_GO_PLAN.md` 是本地完整实现方案，但不是最高优先级规范。
- `docs/roadmap.md` 是当前阶段进度和下一步任务说明。
- `tmp/remnawave-node` 是官方 2.7.0 仓库，必要时必须参考其 contract、controller、service、plugin、Xray 配置生成和错误处理实现。
- `tmp/remnawave-node/libs/contract` 是官方 2.7.0 contract 入口。
- `tmp/remnawave-node-go` 只用于行为对照，不复制它的框架结构。
- 不要修改 `tmp/` 下的参考仓库。

## 工程约束

- HTTP 层使用 Gin。
- 保持公开 JSON 字段名、路由路径、HTTP method 和 response envelope 稳定。
- 新增或修改 Panel-facing contract 前，必须对照官方 `remnawave/node` contract 和相关实现。
- `REMNAWAVE_NODE_GO_PLAN.md`、`docs/roadmap.md` 或当前代码注释与 `tmp/remnawave-node` 官方实现冲突时，以 `tmp/remnawave-node` 为准。
- stub 必须明确，不能伪装成真实 Xray、stats、plugin、nftables 或 conntrack 能力。
- 优先标准库和必要的小依赖；新增依赖要有明确理由。
- 不要打印 `SECRET_KEY`、JWT、节点私钥、客户端证书或 bearer token。
- `XTLS_API_PORT` 和内部控制接口必须保持本机访问，不要在 Docker 示例里暴露。
- 不做无关重构，不移动公开 API 边界，不把参考仓库结构复制进本项目。

## 测试要求

- 新增 route、公开 contract struct 或响应形状时必须补测试。
- 从 stub 进入真实行为实现后，应尽量补 integration test。
- contract 变化要对照官方 `remnawave/node` 和 golden fixture。
- Xray 真实行为需要覆盖 start/stop/healthcheck、用户管理、统计和降级路径。
- 文档状态矩阵必须和当前代码一致，不能把未实现项标记为已完成。

## 文档维护

- `README.md` 写项目定位、当前状态、运行方式和功能进度。
- `AGENTS.md` 写协作规则、当前阶段、工程约束和测试要求。
- `docs/architecture.md` 写架构和运行时边界。
- `docs/contracts.md` 写 contract 对齐、路由覆盖和 stub 策略。
- `docs/development.md` 写本地开发规范和验证命令。
- `docs/roadmap.md` 写 M0-M6 需要实现的能力和完成情况。
- 不在 README 或 AGENTS 里写临时准备步骤、参考仓库拉取命令或忽略规则。

## Git 提交

- 提交信息使用中文，采用标准、有效、可靠的 Conventional Commits 格式。
- 推荐格式：`docs: 规范项目文档和协作说明`、`feat: 实现 Xray 启动流程`、`fix: 修复 JWT 校验失败处理`。
- subject 要简洁说明本次变更；必要时在正文说明原因、影响范围和验证结果。
- 不把无关改动混入同一个 commit。

## 常用命令

- `mise run fmt`
- `mise run test`
- `mise run build`
- `mise run docker-build`
