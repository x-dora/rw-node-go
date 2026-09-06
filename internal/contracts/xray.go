package contracts

import "encoding/json"

type StartXrayRequest struct {
	Internals  StartInternals `json:"internals"`
	XrayConfig map[string]any `json:"xrayConfig"`
}

type StartInternals struct {
	// Metadata mirrors the official NodeMetadataSchema (optional since 3.3.0).
	// Accepted and currently unused by the embedded-core runtime.
	Metadata *NodeMetadata `json:"metadata,omitempty"`
	// Integrations mirrors the official integrations record (optional since
	// 3.2.0). Accepted and ignored; plugin features stay adapter-only.
	Integrations map[string]json.RawMessage `json:"integrations,omitempty"`
	ForceRestart bool                       `json:"forceRestart"`
	Hashes       Hashes                     `json:"hashes"`
}

// NodeMetadata mirrors the official NodeMetadataSchema.
type NodeMetadata struct {
	Name        string   `json:"name"`
	UUID        string   `json:"uuid"`
	ID          int      `json:"id"`
	Tags        []string `json:"tags"`
	CountryCode string   `json:"countryCode"`
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
	Version         *string            `json:"version"`
	Error           *string            `json:"error"`
	NodeInformation NodeInformation    `json:"nodeInformation"`
	System          SystemStatsPayload `json:"system"`
}

type StopXrayResponse struct {
	IsStopped bool `json:"isStopped"`
}

type HealthcheckResponse struct {
	IsAlive                  bool    `json:"isAlive"`
	XrayInternalStatusCached bool    `json:"xrayInternalStatusCached"`
	XrayVersion              *string `json:"xrayVersion"`
	NodeVersion              string  `json:"nodeVersion"`
}

type NodeInformation struct {
	Version *string `json:"version"`
}
