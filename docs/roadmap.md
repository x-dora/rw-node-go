# 路线图

状态说明：`[x]` 已完成，`[~]` 部分完成，`[ ]` 未完成。

## M0: 协议冻结

- [x] 建立 Go module、项目结构、Gin HTTP 层和 controller 分层。
- [x] 注册官方 2.7.x 计划内 Panel-facing routes。
- [x] 建立 `internal/contracts` 请求和响应类型。
- [x] 统一 response envelope。
- [~] 已建立官方 2.7.0 Panel-facing route manifest、代表性 JSON golden fixture 和 Go contract 形状测试；官方 release contract diff 自动提醒仍未完成。

## M1: 节点握手与内嵌 Xray 生命周期

- [x] `SECRET_KEY` base64 JSON 解析和 PEM normalize。
- [x] HTTPS mTLS server config。
- [x] JWT RS256 校验。
- [x] zstd request body。
- [x] 内嵌 `xray-core` instance 启动、停止和重复 start 替换旧 instance。
- [x] Xray config 最小 stats/policy 注入。
- [x] `/node/xray/start`、`/node/xray/stop`、`/node/xray/healthcheck` 已按官方缓存在线状态语义接入基础流程。
- [ ] 真实 Panel + Xray config integration test。

## M2: 用户动态管理

- [x] 内嵌 Xray inbound feature 访问。
- [x] `add-user`、`add-users`。
- [x] `remove-user`、`remove-users`。
- [x] `get-inbound-users`、`get-inbound-users-count`。
- [x] VLESS、Trojan、Shadowsocks、Shadowsocks2022、Hysteria user builder。
- [~] inbound/user hash 状态管理和残留用户清理已接入内存态；真实 Panel + Xray 验收仍未完成。

## M3: 基础统计

- [x] system stats 已按官方 2.7.0 响应形状返回宿主机 CPU、memory、uptime、load average、network interface 列表、默认网卡速率、插件计数占位和 Xray sys stats。
- [~] users、inbound、outbound、combined 流量统计已接入内嵌 Xray stats feature。
- [~] reset 语义已在内嵌 stats counter 上实现；真实 Panel + Xray 验收仍未完成。
- [~] user online status、user IP list 和 users IP list 已通过内嵌 Xray stats OnlineMap 接入；真实 Panel + Xray 验收仍未完成。

## M4: Internal API 与连接处理

- [x] `INTERNAL_REST_PORT` 本机 internal server。
- [x] `GET /internal/get-config` 返回当前内存 config。
- [~] `/vision/block-ip`、`/vision/unblock-ip` 是官方主 API unprefixed route，已接入内嵌 routing feature；真实 Panel + Xray 验收仍未完成。
- [~] drop users connections 已通过 OnlineMap 用户 IP list + conntrack best-effort 接入；无在线 IP 或无系统能力时稳定 no-op。
- [~] drop IPs 已通过 conntrack best-effort 接入；无权限或无系统能力时稳定降级。
- [~] Vision block/unblock dynamic routing feature 实现。
- [x] 无对应系统能力环境的稳定降级测试。

## M5: Plugin 接口适配

- [x] plugin routes 已注册。
- [x] `/node/plugin/sync` 只做 accepted 响应，不保存状态、不触发 Xray restart。
- [x] `/node/plugin/torrent-blocker/collect` 固定返回空 reports。
- [x] `/node/plugin/nftables/*` 固定返回 accepted，不执行 nftables。
- [ ] plugin feature intentionally unsupported；若未来恢复真实能力，需要单独重新设计，不复用当前 adapter stub。

## M6: 发布与跟随上游

- [x] CI test/build。
- [x] Dockerfile 和 Docker multi-arch workflow。
- [~] contract golden 回归矩阵已有官方 2.7.0 route/request/response fixture；已补官方 contract hash baseline，仍需扩展更多 fixture 变体。
- [~] 官方 `remnawave/node` release contract diff 提醒已支持手动 workflow 和本地 `mise run contract-diff`；尚未接入自动 release 监控。
- [ ] 真实 Panel + Xray integration test。
- [ ] 镜像发布前的兼容性验收清单。
