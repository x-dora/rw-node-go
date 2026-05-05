package contracts

type SystemStatsPayload struct {
	Info      map[string]any `json:"info"`
	Stats     map[string]any `json:"stats"`
	Interface map[string]any `json:"interface"`
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
