package state

import (
	"testing"

	"github.com/x-dora/rw-node-go/internal/version"
)

func TestNewRuntimeStateUsesPanelCompatibilityVersion(t *testing.T) {
	runtimeState := NewRuntimeState()
	if runtimeState.NodeVersion != version.NodeVersion {
		t.Fatalf("NodeVersion = %q, want %q", runtimeState.NodeVersion, version.NodeVersion)
	}
	if runtimeState.NodeVersion == version.ProjectVersion {
		t.Fatalf("NodeVersion = ProjectVersion = %q; Panel compatibility version must stay separate", runtimeState.NodeVersion)
	}
}

func TestRuntimeStateTracksInboundProtocol(t *testing.T) {
	runtimeState := NewRuntimeState()

	runtimeState.SetInboundProtocol("VLESS_INBOUND", "vless")

	if got := runtimeState.InboundProtocol("VLESS_INBOUND"); got != "vless" {
		t.Fatalf("InboundProtocol = %q, want vless", got)
	}
	tags := runtimeState.KnownInboundTags()
	if len(tags) != 1 || tags[0] != "VLESS_INBOUND" {
		t.Fatalf("KnownInboundTags = %#v", tags)
	}
}

func TestRuntimeStateTracksInboundProtocolsFromConfig(t *testing.T) {
	runtimeState := NewRuntimeState()

	runtimeState.SetInboundProtocolsFromConfig(map[string]any{
		"inbounds": []any{
			map[string]any{"tag": "VLESS_INBOUND", "protocol": "vless"},
			map[string]any{"tag": "TROJAN_INBOUND", "protocol": "trojan"},
			map[string]any{"tag": "", "protocol": "ignored"},
		},
	})

	if got := runtimeState.InboundProtocol("VLESS_INBOUND"); got != "vless" {
		t.Fatalf("VLESS protocol = %q, want vless", got)
	}
	if got := runtimeState.InboundProtocol("TROJAN_INBOUND"); got != "trojan" {
		t.Fatalf("TROJAN protocol = %q, want trojan", got)
	}
}
