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

type TorrentBlockerReportsResponse struct {
	Reports []TorrentBlockerReport `json:"reports"`
}

type TorrentBlockerReport struct {
	Username string `json:"username,omitempty"`
	IP       string `json:"ip,omitempty"`
	Rule     string `json:"rule,omitempty"`
	At       string `json:"at,omitempty"`
}
