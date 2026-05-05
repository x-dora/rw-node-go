package contracts

type SystemStatsPayload struct {
	Info  NodeSystemInfo  `json:"info"`
	Stats NodeSystemStats `json:"stats"`
}

type NodeSystemInfo struct {
	Arch              string   `json:"arch"`
	CPUs              int      `json:"cpus"`
	CPUModel          string   `json:"cpuModel"`
	MemoryTotal       uint64   `json:"memoryTotal"`
	Hostname          string   `json:"hostname"`
	Platform          string   `json:"platform"`
	Release           string   `json:"release"`
	Type              string   `json:"type"`
	Version           string   `json:"version"`
	NetworkInterfaces []string `json:"networkInterfaces"`
}

type NodeSystemStats struct {
	MemoryFree uint64            `json:"memoryFree"`
	MemoryUsed uint64            `json:"memoryUsed"`
	Uptime     uint64            `json:"uptime"`
	LoadAvg    []float64         `json:"loadAvg"`
	Interface  *NetworkInterface `json:"interface"`
}

type NetworkInterface struct {
	Interface     string `json:"interface"`
	RxBytesPerSec uint64 `json:"rxBytesPerSec"`
	TxBytesPerSec uint64 `json:"txBytesPerSec"`
	RxTotal       uint64 `json:"rxTotal"`
	TxTotal       uint64 `json:"txTotal"`
}

type SystemStatsResponse struct {
	System SystemStatsPayload `json:"system"`
}

type UsersStatsResponse struct {
	Users []UserTrafficStats `json:"users"`
}

type UserTrafficStats struct {
	Username string `json:"username"`
	Downlink int64  `json:"downlink"`
	Uplink   int64  `json:"uplink"`
}

type UserOnlineStatusResponse struct {
	IsOnline bool `json:"isOnline"`
}

type UserIPListResponse struct {
	IPs []IPLastSeen `json:"ips"`
}

type UsersIPListResponse struct {
	Users []UserIPList `json:"users"`
}

type UserIPList struct {
	UserID string       `json:"userId"`
	IPs    []IPLastSeen `json:"ips"`
}

type IPLastSeen struct {
	IP       string `json:"ip"`
	LastSeen string `json:"lastSeen"`
}

type TrafficStatsResponse struct {
	Downlink int64 `json:"downlink"`
	Uplink   int64 `json:"uplink"`
}

type AllInboundsStatsResponse struct {
	Inbounds map[string]TrafficStatsResponse `json:"inbounds"`
}

type AllOutboundsStatsResponse struct {
	Outbounds map[string]TrafficStatsResponse `json:"outbounds"`
}

type CombinedStatsResponse struct {
	Users     []UserTrafficStats              `json:"users"`
	Inbounds  map[string]TrafficStatsResponse `json:"inbounds"`
	Outbounds map[string]TrafficStatsResponse `json:"outbounds"`
	System    SystemStatsPayload              `json:"system"`
}
