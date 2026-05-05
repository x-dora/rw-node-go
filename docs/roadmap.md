# 路线图

状态说明：`[x]` 已完成，`[~]` 部分完成，`[ ]` 未完成。

## M0: 协议冻结

- [x] 建立 Go module、项目结构、Gin HTTP 层和 controller 分层。
- [x] 注册官方 2.7.x 计划内 Panel-facing routes。
- [x] 建立 `internal/contracts` 请求和响应类型。
- [x] 统一 response envelope。
- [~] 已建立官方 2.7.0 Panel-facing route manifest、代表性 JSON golden fixture 和 Go contract 形状测试；官方 release contract diff 流程仍未完成。

## M1: 节点握手与 Xray 生命周期

- [x] `SECRET_KEY` base64 JSON 解析和 PEM normalize。
- [x] HTTPS mTLS server config。
- [x] JWT RS256 校验。
- [x] zstd request body。
- [x] Xray config 注入 API inbound、API service、routing、policy stats 和内部 mTLS 证书。
- [x] 外部 Xray 进程启动、停止、配置写入和 StatsService gRPC ready 检查。
- [x] `/node/xray/start`、`/node/xray/stop`、`/node/xray/healthcheck` 已按官方缓存语义接入基础流程。
- [~] Xray gRPC client 已建立基础连接和 StatsService health check；Handler/Stats/Routing 业务方法和真实 Panel + Xray 验收仍未完成。

## M2: 用户动态管理

- [ ] HandlerService client。
- [ ] `add-user`、`add-users`。
- [ ] `remove-user`、`remove-users`。
- [ ] `get-inbound-users`、`get-inbound-users-count`。
- [ ] VLESS、Trojan、Shadowsocks、Shadowsocks2022、Hysteria user builder。
- [ ] inbound/user hash 状态管理和残留用户清理。

## M3: 基础统计

- [~] system stats 已有基础快照返回。
- [~] StatsService client 已用于内部 health check，流量查询方法仍未接入 controller。
- [ ] users stats、inbound stats、outbound stats、combined stats；当前 stub 响应形状已按官方 2.7.0 contract 修正为空数组或 tag 回显。
- [ ] reset 语义。
- [ ] 完整 system CPU、memory、disk、network、interface stats。

## M4: 在线 IP 与连接处理

- [ ] user online status。
- [ ] user IP list 和 users IP list。
- [ ] drop users connections。
- [ ] drop IPs。
- [ ] Vision block/unblock。
- [ ] 无 `CAP_NET_ADMIN` 环境的稳定降级。

## M5: 插件兼容

- [~] plugin routes 已注册，并返回 accepted 或空 reports。
- [ ] plugin sync 状态持久化或运行时状态管理。
- [ ] torrent blocker config injection、internal webhook、report collect。
- [ ] nftables block、unblock、recreate tables。
- [ ] 权限不足时的稳定降级和测试。

## M6: 发布与跟随上游

- [x] CI test/build。
- [x] Dockerfile 和 Docker multi-arch workflow。
- [~] contract golden 回归矩阵已有官方 2.7.0 route/request/response fixture；仍需接入官方 release diff 提醒。
- [ ] 官方 `remnawave/node` release contract diff 提醒。
- [ ] 真实 Panel + Xray integration test。
- [ ] 镜像发布前的兼容性验收清单。
