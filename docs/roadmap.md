# 路线图

本文档是详细功能进度矩阵的单一事实来源。开发命令、真实 Panel harness 操作和发布流程见 [docs/development.md](development.md)。

状态说明：`[x]` 已完成，`[~]` 部分完成，`[ ]` 未完成。

## M0: 协议冻结

- [x] 建立 Go module、项目结构、Gin HTTP 层和 controller 分层。
- [x] 注册官方 2.7.x 计划内 Panel-facing routes。
- [x] 建立 `internal/contracts` 请求和响应类型。
- [x] 统一 response envelope。
- [~] 官方 2.7.0 Panel-facing route manifest、代表性 JSON golden fixture 和 Go contract 形状测试已建立，已覆盖 plugin null、非空 inbound users 字段和代表性错误 envelope；手动 contract drift 检查已接入，自动 release 监控仍未完成。

## M1: 节点握手与内嵌 Xray 生命周期

- [x] `SECRET_KEY` base64 JSON 解析和 PEM normalize。
- [x] 主服务启动时读取当前工作目录 `.env`，且不覆盖已存在的系统环境变量。
- [x] HTTPS mTLS server config。
- [x] JWT RS256 校验。
- [x] zstd request body。
- [x] 内嵌 `xray-core` instance 启动、停止和重复 start 替换旧 instance。
- [x] Xray config 最小 stats/policy 注入。
- [x] `/node/xray/start`、`/node/xray/stop`、`/node/xray/healthcheck` 已按官方缓存在线状态语义接入基础流程。
- [x] 启动失败时保留上一份内存 config/hash/version 作为 internal 诊断快照，同时把 running 和 cached health 标记为 false。
- [x] 启动、服务监听、Xray 配置下发、start/restart/stop 和失败路径已输出官方风格脱敏摘要日志。
- [~] 真实 Panel + Xray config 脚本验收：live harness 已能启动本地节点、enable Panel 测试节点、等待 `isConnected=true`、结束前 disable 节点；仍需扩大到更多 config/profile/handler/stats 场景。

## M2: 用户动态管理

- [x] 内嵌 Xray inbound feature 访问。
- [x] `add-user`、`add-users`。
- [x] `remove-user`、`remove-users`。
- [x] `get-inbound-users`、`get-inbound-users-count`；`get-inbound-users` 已按官方 runtime model 返回 `username`、`level`、`protocol`。
- [x] VLESS、Trojan、Shadowsocks、Shadowsocks2022、Hysteria user builder。
- [~] inbound/user hash 状态管理和残留用户清理已接入内存态；真实 Panel + Xray 验收仍未完成。

## M3: 基础统计

- [x] system stats 已按官方 2.7.0 响应形状返回宿主机 CPU、memory、uptime、load average、network interface 列表、默认网卡速率、插件计数占位和 Xray sys stats。
- [~] users、inbound、outbound、combined 流量统计已接入内嵌 Xray stats feature；`get-users-stats` 已按官方行为过滤上下行均为 0 的用户。
- [~] reset 语义已在内嵌 stats counter 上实现；真实 Panel + Xray 验收仍未完成。
- [~] user online status、user IP list 和 users IP list 已通过内嵌 Xray stats OnlineMap 接入；OnlineMap 不可用或读取失败时稳定降级为 `false` 或空列表；真实 Panel + Xray 验收仍未完成。

## M4: Internal API 与连接处理

- [x] `INTERNAL_REST_PORT` 本机 internal server。
- [x] `GET /internal/get-config` 返回当前内存 config。
- [~] `/vision/block-ip`、`/vision/unblock-ip` 是官方主 API unprefixed route，已接入内嵌 routing feature；真实 Panel + Xray 验收仍未完成。
- [~] drop users connections 已通过 OnlineMap 用户 IP list + conntrack best-effort 接入；无在线 IP、OnlineMap 不可用或无系统能力时稳定 no-op。
- [~] drop IPs 已通过 conntrack best-effort 接入；无权限或无系统能力时稳定降级。
- [~] Vision block/unblock dynamic routing feature 实现。
- [x] 无对应系统能力环境的稳定降级测试。

## M5: Plugin 接口适配

- [x] plugin routes 已注册。
- [x] `/node/plugin/sync` 只做 accepted 响应，不保存状态、不触发 Xray restart，并覆盖 `plugin: null` adapter-only 场景。
- [x] `/node/plugin/torrent-blocker/collect` 固定返回空 reports。
- [x] `/node/plugin/nftables/*` 固定返回 accepted，不执行 nftables。
- [ ] plugin feature intentionally unsupported；若未来恢复真实能力，需要单独重新设计，不复用当前 adapter stub。

## M6: 发布与跟随上游

- [x] CI test/lint/build。
- [x] Dockerfile 和 Docker multi-arch workflow；runtime 镜像使用 `scratch`，镜像内已按 Xray-core 方式预置 `/usr/local/share/xray/geoip.dat` 和 `/usr/local/share/xray/geosite.dat`，`main` push 会推送滚动开发镜像 `ghcr.io/x-dora/rw-node-go:dev`。
- [x] 项目发布版本和 Panel 兼容版本已拆分：项目版本由根目录 `VERSION` 管理，Panel-facing `nodeVersion` 继续默认上报 `2.7.0`。
- [x] 本地 `mise run build`、CI、Docker 和 release 已统一到同一个构建入口读取 `VERSION` 并注入 `ProjectVersion`。
- [x] Release workflow 已接入：普通 `main` push 更新滚动 `pre-release` 和 Linux `tar.gz` 资产；`VERSION` 变更后先推 GHCR 多架构镜像，成功后再创建正式 release 并上传 Linux `tar.gz` 资产，支持已有正式 release 的手动镜像补推恢复入口。
- [x] Xray geodata 已按 Xray-core 的 `Loyalsoldier/v2ray-rules-dat` release 资产流程接入定时下载、sha256 校验和 Actions cache。
- [x] 发布前门禁已接入：Preflight workflow 覆盖格式检查、测试、lint、构建和 contract diff，并支持受控运行真实 Panel live harness。
- [~] contract golden 回归矩阵已有官方 2.7.0 route/request/response fixture、非空 inbound users 字段、plugin null 和代表性错误 envelope；已补官方 contract hash baseline，仍需继续扩展更多 fixture 变体。
- [~] 官方 `remnawave/node` contract drift 检查已支持手动 workflow、本地 `mise run contract-diff`、`CONTRACT_TAG` 和 `CONTRACT_SOURCE_DIR` 本地源码 fallback；尚未接入自动 release 监控。
- [~] 真实 Panel + Xray 脚本验收：已建立可重复执行的 live harness 和结构化日志，当前覆盖 Panel 连通、节点 enable/disable、Panel 侧连接状态断言、最小 smoke 和可选只读 extended smoke；仍需补齐 handler、stats、Vision 和失败路径覆盖。
- [x] 镜像发布前的基础兼容性验收清单已固化为 `mise run preflight` 和 Preflight workflow；更完整真实 Panel 场景仍需继续扩展。
