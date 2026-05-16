# Agent 协作说明

本仓库是 Remnawave Node 兼容服务的 Go 实现，目标 contract 是官方 `remnawave/node` dev/2.8.0 面向 Panel 的 API。协作时优先保持公开接口稳定，再逐步把 stub 替换为真实运行时能力。

详细功能进度只在 `docs/roadmap.md` 维护；本文件只记录协作规则和不可违背的工程约束。

## 当前阶段

- 当前唯一运行模式是内嵌 `xray-core`；不要重新引入外部 `xray` 进程、Xray 配置落盘、内部 gRPC API inbound 或 internal mTLS。
- Panel-facing contract 必须稳定：路由路径、HTTP method、JSON 字段名和 response envelope 变更前必须对照官方 `remnawave/node` dev/2.8.0。
- Handler、stats 和连接清理已部分接入内嵌 Xray feature 或系统能力；真实 Panel + Xray 的完整验收仍在推进。
- Stats online status/IP 已通过内嵌 Xray stats `OnlineMap` 接入；不可用或读取失败时稳定降级为 `false` 或空列表。
- 真实 Panel live harness 只能通过 `scripts/panel-integration.sh` 触发。`run`、`enable` 和 `disable` 会修改真实 Panel 节点状态，必须使用完整节点 UUID，只能指向测试节点，并在结束或失败清理时 disable 节点和停止本地进程。
- 项目自身发布版本由根目录 `VERSION` 管理；Panel-facing `nodeVersion` 是兼容性版本，默认继续上报官方 dev/2.8.0 的 `2.8.0`。
- Plugin 功能不做真实实现；只保留 Panel-facing contract adapter，不能保存插件状态、注入 Xray 配置、接收 webhook、触发 Xray restart 或执行 nftables。

## 必须参考

- `tmp/remnawave-node` 是官方仓库参考，当前对齐目标是 dev 提交 `a5acdeb28840e21c2622a6362dc6824b6e70eea5`，必要时必须参考其 contract、controller、service、Xray 配置生成和错误处理实现。
- `tmp/remnawave-node/libs/contract` 是官方 dev/2.8.0 contract 入口。
- `tmp/remnawave-node-go` 只用于内嵌 `xray-core` 结构和行为对照，不复制它的 contract 或框架结构。
- `REMNAWAVE_NODE_GO_PLAN.md` 是历史设计备忘，不是当前实现规范。
- 不要修改 `tmp/` 下的参考仓库。

## 工程约束

- HTTP 层使用 Gin。
- 保持公开 JSON 字段名、路由路径、HTTP method 和 response envelope 稳定。
- 新增或修改 Panel-facing contract 前，必须对照官方 `remnawave/node` contract 和相关实现。
- Stub 必须明确，不能伪装成真实 Xray、stats、plugin、nftables 或 conntrack 能力。
- 优先标准库和必要的小依赖；新增依赖要有明确理由。
- 不要打印 `SECRET_KEY`、JWT、节点私钥、客户端证书或 bearer token。
- `INTERNAL_REST_PORT` 只允许本机访问，不要在 Docker 示例里暴露。
- `NODE_TLS_CLIENT_AUTH` 默认必须保持 `mtls`；只有前置可信代理已完成客户端证书校验且源站访问被限制时，才允许显式设为 `none`。Go 侧所有已注册 Panel-facing route 都必须校验 JWT。官方 dev/2.8.0 已移除 `/vision/*` public contract，Go 侧不得重新注册这些 route，除非明确记录 deliberate divergence。
- 不做无关重构，不移动公开 API 边界，不把参考仓库结构复制进本项目。

## 测试要求

- 新增 route、公开 contract struct 或响应形状时必须补测试。
- 从 stub 进入真实行为实现后，应尽量补 integration test。
- 真实 Panel live harness 不能作为 `go test` 测试暴露；涉及节点联通时必须断言 Panel 侧 `isConnected=true`，并在清理阶段 disable 测试节点；清理失败必须显式失败或输出清晰错误。
- Contract 变化要对照官方 `remnawave/node` 和 golden fixture。
- Xray 真实行为需要覆盖 start/stop/healthcheck、用户管理、统计和降级路径。
- 文档状态矩阵必须和当前代码一致，不能把未实现项标记为已完成。

## 文档维护

- `README.md` 写项目定位、关键能力快照、最短运行路径、配置入口和文档导航。
- `AGENTS.md` 写协作规则、当前阶段硬约束、工程约束、测试要求、文档维护规则、提交规则和常用命令。
- `docs/architecture.md` 写架构分层、运行路径、运行时边界和 internal API 边界。
- `docs/contracts.md` 写 Panel-facing contract 对齐、route 覆盖、stub 策略、golden fixture 和 contract drift 检查。
- `docs/development.md` 写本地开发、验证命令、真实 Panel harness 操作、版本发布操作和实现规则。
- `docs/roadmap.md` 写需要实现的能力和详细完成情况；README 和 AGENTS 不维护完整进度矩阵。
- 新增功能、改变运行方式、改变公开 API/contract、改变测试/联调流程或改变配置项时，必须同步更新对应文档；至少检查 `README.md`、`docs/development.md`、`docs/contracts.md`、`docs/roadmap.md`、`.env*.example` 和本文件是否需要调整。
- 文档更新要和实际行为一致：如果代码会 enable/disable 真实 Panel 节点、修改运行端口、读取 `geoip.dat`/`geosite.dat`、改变日志或清理策略，文档必须明确风险、前置条件、命令和清理行为。
- 文档更新时优先对照当前代码、`VERSION`、`.mise.toml`、`.env*.example`、`scripts/` 和 `.github/workflows/` 的实际行为，避免写入会随版本或 workflow 漂移的硬编码状态。
- 修改 GitHub Actions、release 流程、镜像发布或恢复入口时，必须同步验证 release 顺序、GHCR 权限说明和相关文档，不能只改 workflow 不改说明。
- 不在 README 或 AGENTS 里写临时准备步骤、参考仓库拉取命令或忽略规则。

## Git 提交

- 提交信息必须使用中文，严格采用 Conventional Commits 格式：`<type>(<scope>): <简短描述>`；scope 要能指向模块、能力或工具链，例如 `ci`、`release`、`xray`、`docs`、`docker`。
- 可用 type：`feat`、`fix`、`refactor`、`style`、`chore`、`docs`、`test`、`ci`、`perf`。配置 CI/CD、release、Docker workflow 时优先使用 `ci`；工具、依赖和构建脚本使用 `chore`；纯文档使用 `docs`。
- `style` 只用于纯空白、缩进、换行、引号等不影响逻辑的格式变更；lint 规则修复、路径 API 替换、变量重命名或代码结构变化应使用 `refactor` 或更具体的 type。
- Subject 必须具体说明真实变更，禁止使用“清理”“优化”“调整”“更新”“统一”“整理”等模糊词；不要写“清理代码”“提升可读性”“修复问题”这类无法追踪意图的描述。
- 提交前必须分析 diff 中文件名和内容来判断 type/scope。新增 API、模型、路由用 `feat`；修复可复现缺陷用 `fix`；工具迁移或 lock 文件替换用 `chore(deps)` 或 `chore(tools)`；多类变更混在同一提交时选主导意图，并在 body 逐条说明。
- 多处变更必须在标题下空一行，用 `- ` 列表写 body；每条说明具体文件、规则或行为变化，避免只写“完善流程”。
- 示例：
  - `ci(release): 发布包内置 Xray geodata`
  - `fix(auth): 修复 JWT 过期时间校验`
  - `chore(deps): 从 poetry 迁移到 uv`
- 不把无关改动混入同一个 commit。

## 常用命令

- `mise run fmt`
- `mise run lint`
- `mise run test`
- `mise run build`
- `mise run preflight`
- `mise run docker-build`
