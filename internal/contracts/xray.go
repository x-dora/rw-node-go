package contracts

type StartXrayRequest struct {
	Internals  StartInternals `json:"internals"`
	XrayConfig map[string]any `json:"xrayConfig"`
}

type StartInternals struct {
	ForceRestart bool   `json:"forceRestart"`
	Hashes       Hashes `json:"hashes"`
}

type Hashes struct {
	EmptyConfig string        `json:"emptyConfig"`
	Inbounds    []InboundHash `json:"inbounds"`
}

type InboundHash struct {
	UsersCount int    `json:"usersCount"`
	Hash       string `json:"hash"`
	Tag        string `json:"tag"`
}

type StartXrayResponse struct {
	IsStarted       bool               `json:"isStarted"`
	Version         string             `json:"version"`
	Error           *string            `json:"error"`
	NodeInformation NodeInformation    `json:"nodeInformation"`
	System          SystemStatsPayload `json:"system"`
}

type StopXrayResponse struct {
	IsStopped bool `json:"isStopped"`
}

type HealthcheckResponse struct {
	IsAlive                  bool   `json:"isAlive"`
	XrayInternalStatusCached bool   `json:"xrayInternalStatusCached"`
	XrayVersion              string `json:"xrayVersion"`
	NodeVersion              string `json:"nodeVersion"`
}

type NodeInformation struct {
	Version string `json:"version"`
}
