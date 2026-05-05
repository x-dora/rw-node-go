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
	Type        string         `json:"type"`
	RuleTag     string         `json:"ruleTag"`
	InboundTag  string         `json:"inboundTag"`
	Protocol    string         `json:"protocol"`
	User        string         `json:"user"`
	IP          string         `json:"ip"`
	Destination string         `json:"destination"`
	Raw         map[string]any `json:"raw,omitempty"`
}
