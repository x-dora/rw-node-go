# Agent 协作说明

本仓库是 Remnawave Node 兼容服务的 Go 实现，目标 contract 是官方 `remnawave/node` 2.7.x 面向 Panel 的 API。协作时优先保持公开接口稳定，再逐步把 stub 替换为真实运行时能力。

## 当前阶段

- 当前唯一运行模式是内嵌 `xray-core`；不要重新引入外部 `xray` 进程、Xray 配置落盘、内部 gRPC API inbound 或 internal mTLS。
- 已完成项目骨架、Gin HTTP 层、公开路由注册、contract struct、response envelope、CI、Docker 构建和 release 流程。
- 已完成 `SECRET_KEY` 解析、mTLS、JWT RS256、zstd request body。
- `/node/xray/start`、`/node/xray/stop`、`/node/xray/healthcheck` 已接入内嵌 Xray instance 生命周期。
- handler、stats 和 Vision 已通过内嵌 Xray feature 部分接入；drop connections 已通过 conntrack best-effort 接入并保留稳定降级，online IP 仍是降级响应。
- 已建立脚本专用真实 Panel live harness：只能通过 `scripts/panel-integration.sh` 触发；可启动本地节点、调用 Panel API enable 测试节点、等待 Panel 报告 `isConnected=true`、跑最小 smoke，并在结束或失败清理时 disable 节点和停止本地进程。该 harness 会修改真实 Panel 节点状态，`run`、`enable` 和 `disable` 必须使用完整节点 UUID，只能指向测试节点。
- 项目自身发布版本由根目录 `VERSION` 管理，从 `1.0.0` 开始；Panel-facing `nodeVersion` 是兼容性版本，默认继续上报官方 2.7.x 的 `2.7.0`。
- plugin 功能不做真实实现；只保留 Panel-facing contract adapter，不能保存插件状态、注入 Xray 配置、接收 webhook、触发 Xray restart 或执行 nftables。

## 必须参考

- `tmp/remnawave-node` 是官方 2.7.0 仓库，必要时必须参考其 contract、controller、service、Xray 配置生成和错误处理实现。
- `tmp/remnawave-node/libs/contract` 是官方 2.7.0 contract 入口。
- `tmp/remnawave-node-go` 只用于内嵌 `xray-core` 结构和行为对照，不复制它的 contract 或框架结构。
- `REMNAWAVE_NODE_GO_PLAN.md` 是历史设计备忘，不是当前实现规范。
- 不要修改 `tmp/` 下的参考仓库。

## 工程约束

- HTTP 层使用 Gin。
- 保持公开 JSON 字段名、路由路径、HTTP method 和 response envelope 稳定。
- 新增或修改 Panel-facing contract 前，必须对照官方 `remnawave/node` contract 和相关实现。
- stub 必须明确，不能伪装成真实 Xray、stats、plugin、nftables 或 conntrack 能力。
- 优先标准库和必要的小依赖；新增依赖要有明确理由。
- 不要打印 `SECRET_KEY`、JWT、节点私钥、客户端证书或 bearer token。
- `INTERNAL_REST_PORT` 只允许本机访问，不要在 Docker 示例里暴露。
- 不做无关重构，不移动公开 API 边界，不把参考仓库结构复制进本项目。

## 测试要求

- 新增 route、公开 contract struct 或响应形状时必须补测试。
- 从 stub 进入真实行为实现后，应尽量补 integration test。
- 真实 Panel live harness 不能作为 `go test` 测试暴露；涉及节点联通时必须断言 Panel 侧 `isConnected=true`，并在清理阶段 disable 测试节点；清理失败必须显式失败或输出清晰错误。
- contract 变化要对照官方 `remnawave/node` 和 golden fixture。
- Xray 真实行为需要覆盖 start/stop/healthcheck、用户管理、统计和降级路径。
- 文档状态矩阵必须和当前代码一致，不能把未实现项标记为已完成。

## 文档维护

- `README.md` 写项目定位、当前状态、运行方式和功能进度。
- `AGENTS.md` 写协作规则、当前阶段、工程约束和测试要求。
- `docs/architecture.md` 写架构和运行时边界。
- `docs/contracts.md` 写 contract 对齐、路由覆盖和 stub 策略。
- `docs/development.md` 写本地开发规范和验证命令。
- `docs/roadmap.md` 写需要实现的能力和完成情况。
- 新增功能、改变运行方式、改变公开 API/contract、改变测试/联调流程或改变配置项时，必须同步更新对应文档；至少检查 `README.md`、`docs/development.md`、`docs/contracts.md`、`docs/roadmap.md`、`.env*.example` 和本文件是否需要调整。
- 文档更新要和实际行为一致：如果代码会 enable/disable 真实 Panel 节点、修改运行端口、读取 `geoip.dat`/`geosite.dat`、改变日志或清理策略，文档必须明确风险、前置条件、命令和清理行为。
- 修改 GitHub Actions、release 流程、镜像发布或恢复入口时，必须同步验证 release 顺序、GHCR 权限说明和相关文档，不能只改 workflow 不改说明。
- 不在 README 或 AGENTS 里写临时准备步骤、参考仓库拉取命令或忽略规则。

## Git 提交

- 提交信息必须使用中文，严格采用 Conventional Commits 格式：`<type>(<scope>): <简短描述>`；scope 要能指向模块、能力或工具链，例如 `ci`、`release`、`xray`、`docs`、`docker`。
- 可用 type：`feat`、`fix`、`refactor`、`style`、`chore`、`docs`、`test`、`ci`、`perf`。配置 CI/CD、release、Docker workflow 时优先使用 `ci`；工具、依赖和构建脚本使用 `chore`；纯文档使用 `docs`。
- `style` 只用于纯空白、缩进、换行、引号等不影响逻辑的格式变更；lint 规则修复、路径 API 替换、变量重命名或代码结构变化应使用 `refactor` 或更具体的 type。
- subject 必须具体说明真实变更，禁止使用“清理”“优化”“调整”“更新”“统一”“整理”等模糊词；不要写“清理代码”“提升可读性”“修复问题”这类无法追踪意图的描述。
- 提交前必须分析 diff 中文件名和内容来判断 type/scope。新增 API、模型、路由用 `feat`；修复可复现缺陷用 `fix`；工具迁移或 lock 文件替换用 `chore(deps)` 或 `chore(tools)`；多类变更混在同一提交时选主导意图，并在 body 逐条说明。
- 多处变更必须在标题下空一行，用 `- ` 列表写 body；每条说明具体文件、规则或行为变化，避免只写“完善流程”。
- 示例：
  - `ci(release): 发布包内置 Xray geodata`
  - `fix(auth): 修复 JWT 过期时间校验`
  - `chore(deps): 从 poetry 迁移到 uv`
- 不把无关改动混入同一个 commit。

## 常用命令

- `mise run fmt`
- `mise run test`
- `mise run build`
- `mise run preflight`
- `mise run docker-build`
