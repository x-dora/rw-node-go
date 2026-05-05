package contracts

type PluginSyncRequest struct {
	Plugin *PluginDescriptor `json:"plugin"`
}

type PluginDescriptor struct {
	Config map[string]any `json:"config"`
	UUID   string         `json:"uuid"`
	Name   string         `json:"name"`
}

type AcceptedResponse struct {
	Accepted bool `json:"accepted"`
}

type BlockIPsRequest struct {
	IPs []BlockIPRequest `json:"ips"`
}

type BlockIPRequest struct {
	IP      string `json:"ip"`
	Timeout int    `json:"timeout"`
}

type UnblockIPsRequest struct {
	IPs []string `json:"ips"`
}

type TorrentBlockerReportsResponse struct {
	Reports []TorrentBlockerReport `json:"reports"`
}

type TorrentBlockerReport struct {
	ActionReport TorrentBlockerActionReport `json:"actionReport"`
	XrayReport   XrayWebhookReport          `json:"xrayReport"`
}

type TorrentBlockerActionReport struct {
	Blocked       bool   `json:"blocked"`
	IP            string `json:"ip"`
	BlockDuration int    `json:"blockDuration"`
	WillUnblockAt string `json:"willUnblockAt"`
	UserID        string `json:"userId"`
	ProcessedAt   string `json:"processedAt"`
}

type XrayWebhookReport struct {
	Email          *string `json:"email"`
	Level          *int    `json:"level"`
	Protocol       *string `json:"protocol"`
	Network        string  `json:"network"`
	Source         *string `json:"source"`
	Destination    string  `json:"destination"`
	RouteTarget    *string `json:"routeTarget"`
	OriginalTarget *string `json:"originalTarget"`
	InboundTag     *string `json:"inboundTag"`
	InboundName    *string `json:"inboundName"`
	InboundLocal   *string `json:"inboundLocal"`
	OutboundTag    *string `json:"outboundTag"`
	Timestamp      int64   `json:"ts"`
}
