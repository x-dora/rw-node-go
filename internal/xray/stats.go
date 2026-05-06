package xray

import "strings"

const (
	StatsUserUplinkFormat       = "user>>>%s>>>traffic>>>uplink"
	StatsUserDownlinkFormat     = "user>>>%s>>>traffic>>>downlink"
	StatsUserOnlineFormat       = "user>>>%s>>>online"
	StatsInboundUplinkFormat    = "inbound>>>%s>>>traffic>>>uplink"
	StatsInboundDownlinkFormat  = "inbound>>>%s>>>traffic>>>downlink"
	StatsOutboundUplinkFormat   = "outbound>>>%s>>>traffic>>>uplink"
	StatsOutboundDownlinkFormat = "outbound>>>%s>>>traffic>>>downlink"
)

type SysStats struct {
	NumGoroutine uint32
	NumGC        uint32
	Alloc        uint64
	TotalAlloc   uint64
	Sys          uint64
	Mallocs      uint64
	Frees        uint64
	LiveObjects  uint64
	PauseTotalNs uint64
	Uptime       uint32
}

type UserTrafficStats struct {
	Username string
	Downlink int64
	Uplink   int64
}

type UserIPList struct {
	Username string
	IPs      []IPLastSeen
}

type IPLastSeen struct {
	IP       string
	LastSeen int64
}

type InboundTrafficStats struct {
	Inbound  string
	Downlink int64
	Uplink   int64
}

type OutboundTrafficStats struct {
	Outbound string
	Downlink int64
	Uplink   int64
}

func parseTrafficStatName(name string, prefix string) (tag string, direction string, ok bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 4 || parts[0] != prefix || parts[2] != "traffic" {
		return "", "", false
	}
	if parts[1] == "" || (parts[3] != "uplink" && parts[3] != "downlink") {
		return "", "", false
	}
	return parts[1], parts[3], true
}

func parseOnlineStatName(name string) (username string, ok bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 3 || parts[0] != "user" || parts[1] == "" || parts[2] != "online" {
		return "", false
	}
	return parts[1], true
}

func applyDirection(direction string, value int64, uplink *int64, downlink *int64) {
	switch direction {
	case "uplink":
		*uplink += value
	case "downlink":
		*downlink += value
	}
}
