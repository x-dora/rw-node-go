# 开发说明

本文档记录本项目的长期开发规范。临时准备步骤、参考仓库维护和个人环境细节不要写入入口文档。

## 工具链

项目使用 `mise` 管理 Go 版本和常用任务：

```sh
mise install
```

常用命令：

```sh
mise run fmt
mise run test
mise run build
mise run docker-build
```

`mise run lint` 依赖本地已安装 `golangci-lint`。

## 本地运行

开发模式可以不设置 `SECRET_KEY`，此时服务使用 HTTP，便于 route 和 contract 测试：

```sh
NODE_PORT=2222 mise exec -- go run ./cmd/rw-node-go
```

设置 `SECRET_KEY` 后会启用 HTTPS、mTLS 和 JWT RS256 校验。`SECRET_KEY` 内容不得写入日志、测试输出或文档示例。

## 实现规则

- HTTP 层使用 Gin，route 注册集中在 `internal/httpapi/router.go`。
- Panel-facing response 使用 `httpapi.WriteEnvelope`。
- 新增公开请求或响应类型放在 `internal/contracts`。
- 真实运行时行为应通过 controller 调用内部抽象，不要在 HTTP handler 中堆业务逻辑。
- Xray 相关能力优先通过 `internal/xray` 抽象实现。
- system、conntrack、nftables 能力放在 `internal/system`，权限不足时应稳定降级。
- plugin 状态和报告处理放在 `internal/plugin`，不要让 plugin stub 伪装成真实封禁能力。

## 测试策略

- 新增 route、公开 contract struct 或 response shape 时必须补单元测试。
- 从 stub 进入真实 Xray 行为时，应补 integration test 或明确记录无法在 CI 中验证的原因。
- mTLS/JWT/zstd、response envelope、router、config builder 和 process core 属于基础能力，修改时必须跑完整测试。
- contract golden 应只保存必要 fixture，避免复制大段上游源码。
- 计划文档和官方 `tmp/remnawave-node` 实现冲突时，以官方仓库为准，并同步修正文档中的错误假设。

建议验证顺序：

```sh
mise run fmt
mise run test
mise run build
```

涉及 Docker 的改动再运行：

```sh
mise run docker-build
```

## 文档归属

- `README.md`：项目定位、当前状态、运行方式和功能进度。
- `AGENTS.md`：协作规则、工程约束、当前阶段和测试要求。
- `docs/architecture.md`：架构分层、运行路径和未完成边界。
- `docs/contracts.md`：contract 对齐、route 覆盖和 stub 策略。
- `docs/roadmap.md`：M0-M6 路线图和当前完成情况。
- `REMNAWAVE_NODE_GO_PLAN.md`：完整技术方案和长期实现细节；若与官方仓库冲突，以官方仓库为准。
