package xray

type UserProtocol string

const (
	ProtocolVLESS         UserProtocol = "vless"
	ProtocolTrojan        UserProtocol = "trojan"
	ProtocolShadowsocks   UserProtocol = "shadowsocks"
	ProtocolShadowsocks22 UserProtocol = "shadowsocks22"
	ProtocolHysteria      UserProtocol = "hysteria"
)
