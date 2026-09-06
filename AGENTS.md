# Agent 协作说明

本仓库是 Remnawave Node 兼容服务的 Go 实现，目标 contract 是官方 [`remnawave/node`](https://github.com/remnawave/node) [`3.4.1`](https://github.com/remnawave/node/tree/3.4.1) 面向 Panel 的 API。协作时优先保持公开接口稳定，再逐步把 stub 替换为真实运行时能力。

详细功能进度只在 [`docs/roadmap.md`](docs/roadmap.md) 维护；本文件只记录协作规则和不可违背的工程约束。

## 当前阶段

- 当前唯一运行模式是内嵌 [`xray-core`](https://github.com/XTLS/Xray-core)；不要重新引入外部 `xray` 进程、Xray 配置落盘、内部 gRPC API inbound 或 internal mTLS。
- Panel-facing contract 必须稳定：路由路径、HTTP method、JSON 字段名和 response envelope 变更前必须对照官方 [`remnawave/node`](https://github.com/remnawave/node) [`3.4.1`](https://github.com/remnawave/node/tree/3.4.1)。
- Handler、stats 和连接清理已部分接入内嵌 Xray feature 或系统能力；真实 Panel + Xray 的完整验收仍在推进。
- Stats online status/IP 已通过内嵌 Xray stats `OnlineMap` 接入；不可用或读取失败时稳定降级为 `false` 或空列表。
- `get-geocheck` 依赖镜像内置的官方 `geocheck` 二进制；二进制缺失或执行失败时稳定降级为 `A018` 错误。start 时按 Panel 下发的 `xrayConfig.geodata.assets` 下载 geodata 资产（失败落空 stub，不阻断启动）。
- 真实 Panel live harness 只能通过 [`scripts/panel-integration.sh`](scripts/panel-integration.sh) 触发。`run`、`enable` 和 `disable` 会修改真实 Panel 节点状态，必须使用完整节点 UUID，只能指向测试节点，并在结束或失败清理时 disable 节点和停止本地进程。
- 项目自身发布版本由根目录 [`VERSION`](VERSION) 管理；Panel-facing `nodeVersion` 是兼容性版本，默认上报官方 3.4.1 的 `3.4.1`。
- Plugin 功能不做真实实现；只保留 Panel-facing contract adapter，不能保存插件状态、注入 Xray 配置、接收 webhook、触发 Xray restart 或执行 nftables。官方 3.0.0 新增的 pre-start plugin（stale socket 清理）同样不实现。
- 与官方 3.4.1 的 deliberate divergence 记录在 [`docs/contracts.md`](docs/contracts.md) 和 [`docs/architecture.md`](docs/architecture.md)：缺少 `DISABLE_HASHED_SET_CHECK`、internal REST 用 loopback TCP 而非 abstract unix socket、pre-start plugin 不实现、`geodata.core` 换 xray 二进制不实现（内嵌 core 运行时不可替换）、instance lock 重复实例告警不实现、`internals.integrations` 接受但忽略。改动这些行为前先更新文档。

## 必须参考

- 官方 [`remnawave/node`](https://github.com/remnawave/node) 仓库需要一份本地 checkout 作为参考，当前对齐目标是 tag [`3.4.1`](https://github.com/remnawave/node/tree/3.4.1)（commit `44912631321664dbd5822e9bf8d96766ccff7c93`）。必要时必须参考其 contract、controller、service、Xray 配置生成和错误处理实现；`libs/contract` 是官方 [`3.4.1 contract`](https://github.com/remnawave/node/tree/3.4.1/libs/contract) 入口。
- 本地 checkout 的实际路径是本机配置，不写进跟踪文档。用 `CONTRACT_SOURCE_DIR` 指定，或从未跟踪的 `CLAUDE.local.md` 读取；获取和校验方式见 [`docs/development.md`](docs/development.md)。
- 不要修改本地参考仓库的内容，也不要把它复制进本项目。
- [`REMNAWAVE_NODE_GO_PLAN.md`](REMNAWAVE_NODE_GO_PLAN.md) 是历史设计备忘，不是当前实现规范。

## 工程约束

- HTTP 层使用 Gin。
- 保持公开 JSON 字段名、路由路径、HTTP method 和 response envelope 稳定。
- 新增或修改 Panel-facing contract 前，必须对照官方 [`remnawave/node`](https://github.com/remnawave/node) contract 和相关实现。
- Stub 必须明确，不能伪装成真实 Xray、stats、plugin、nftables 或 conntrack 能力。
- 优先标准库和必要的小依赖；新增依赖要有明确理由。
- 不要打印 `SECRET_KEY`、JWT、节点私钥、客户端证书或 bearer token。
- `INTERNAL_REST_PORT` 只允许本机访问，不要在 Docker 示例里暴露。
- `NODE_TLS_CLIENT_AUTH` 默认必须保持 `mtls`；只有前置可信代理已完成客户端证书校验且源站访问被限制时，才允许显式设为 `none`。主 API TLS 最低版本是 TLS 1.3，跟随官方 3.4.1 的 `httpsOptions.minVersion`，不要下调。`SNI_VERIFICATION` 默认保持 `false`，与官方一致；开启后仅放行从 SECRET_KEY 派生的 SNI。Go 侧所有已注册 Panel-facing route 都必须校验 JWT。官方已移除 `/vision/*` public contract 和 `get-inbound-users`/`get-inbound-users-count` 两条 route，Go 侧不得重新注册这些 route，除非明确记录 deliberate divergence。
- 不做无关重构，不移动公开 API 边界，不把参考仓库结构复制进本项目。

## 测试要求

- 新增 route、公开 contract struct 或响应形状时必须补测试。
- 从 stub 进入真实行为实现后，应尽量补 integration test。
- 真实 Panel live harness 不能作为 `go test` 测试暴露；涉及节点联通时必须断言 Panel 侧 `isConnected=true`，并在清理阶段 disable 测试节点；清理失败必须显式失败或输出清晰错误。
- Contract 变化要对照官方 [`remnawave/node`](https://github.com/remnawave/node) 和 golden fixture。
- Xray 真实行为需要覆盖 start/stop/healthcheck、用户管理、统计和降级路径。
- 文档状态矩阵必须和当前代码一致，不能把未实现项标记为已完成。

## 文档维护

- [`README.md`](README.md) 写项目定位、关键能力快照、最短运行路径、配置入口和文档导航。
- [`AGENTS.md`](AGENTS.md) 写协作规则、当前阶段硬约束、工程约束、测试要求、文档维护规则、提交规则和常用命令。
- [`docs/architecture.md`](docs/architecture.md) 写架构分层、运行路径、运行时边界和 internal API 边界。
- [`docs/contracts.md`](docs/contracts.md) 写 Panel-facing contract 对齐、route 覆盖、stub 策略、golden fixture 和 contract drift 检查。
- [`docs/development.md`](docs/development.md) 写本地开发、验证命令、真实 Panel harness 操作、版本发布操作和实现规则。
- [`docs/roadmap.md`](docs/roadmap.md) 写需要实现的能力和详细完成情况；README 和 AGENTS 不维护完整进度矩阵。
- 新增功能、改变运行方式、改变公开 API/contract、改变测试/联调流程或改变配置项时，必须同步更新对应文档；至少检查 [`README.md`](README.md)、[`docs/development.md`](docs/development.md)、[`docs/contracts.md`](docs/contracts.md)、[`docs/roadmap.md`](docs/roadmap.md)、`.env*.example` 和本文件是否需要调整。
- 文档更新要和实际行为一致：如果代码会 enable/disable 真实 Panel 节点、修改运行端口、读取 `geoip.dat`/`geosite.dat`、改变日志或清理策略，文档必须明确风险、前置条件、命令和清理行为。
- 文档更新时优先对照当前代码、[`VERSION`](VERSION)、[`.mise.toml`](.mise.toml)、`.env*.example`、[`scripts/`](scripts/) 和 [`.github/workflows/`](.github/workflows/) 的实际行为，避免写入会随版本或 workflow 漂移的硬编码状态。
- 修改 GitHub Actions、release 流程、镜像发布或恢复入口时，必须同步验证 release 顺序、GHCR 权限说明和相关文档，不能只改 workflow 不改说明。
- 不在 README 或 AGENTS 里写临时准备步骤、参考仓库拉取命令或忽略规则。
- `CLAUDE.md` 是指向 [`AGENTS.md`](AGENTS.md) 的符号链接，只改 `AGENTS.md`。不要用复制覆盖 `CLAUDE.md`，那会断开链接让两份内容分叉。Windows 上重建链接需要开发者模式，仓库已设 `core.symlinks true`。

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
