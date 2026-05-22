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

func TestRestartDecisionReasons(t *testing.T) {
	runtimeState := NewRuntimeState()
	if decision := runtimeState.RestartDecision(true, Hashes{}, true); !decision.ShouldRestart || decision.Reason != RestartReasonForce {
		t.Fatalf("force decision = %#v", decision)
	}
	if decision := runtimeState.RestartDecision(false, Hashes{}, false); !decision.ShouldRestart || decision.Reason != RestartReasonCoreNotRunning {
		t.Fatalf("core not running decision = %#v", decision)
	}

	runtimeState.SetXrayStarted(nil, map[string]any{}, Hashes{
		EmptyConfig: "old",
		Inbounds: []InboundHash{{
			Tag:        "VLESS_INBOUND",
			UsersCount: 1,
			Hash:       "h1",
		}},
	})

	if decision := runtimeState.RestartDecision(false, Hashes{EmptyConfig: "new"}, true); !decision.ShouldRestart || decision.Reason != RestartReasonEmptyConfigHashChange {
		t.Fatalf("empty hash change decision = %#v", decision)
	}
	if decision := runtimeState.RestartDecision(false, Hashes{
		EmptyConfig: "old",
		Inbounds: []InboundHash{{
			Tag:        "VLESS_INBOUND",
			UsersCount: 2,
			Hash:       "h2",
		}},
	}, true); !decision.ShouldRestart || decision.Reason != RestartReasonInboundHashChange || decision.InboundTag != "VLESS_INBOUND" {
		t.Fatalf("inbound hash change decision = %#v", decision)
	}
	if decision := runtimeState.RestartDecision(false, Hashes{
		EmptyConfig: "old",
		Inbounds: []InboundHash{{
			Tag:        "VLESS_INBOUND",
			UsersCount: 1,
			Hash:       "h1",
		}},
	}, true); decision.ShouldRestart || decision.Reason != RestartReasonNoRestart {
		t.Fatalf("no restart decision = %#v", decision)
	}
}

func TestRuntimeStateSnapshotsAreIsolated(t *testing.T) {
	runtimeState := NewRuntimeState()
	version := "1.2.3"
	currentConfig := map[string]any{
		"inbounds": []any{
			map[string]any{"tag": "VLESS_INBOUND", "protocol": "vless"},
		},
	}
	hashes := Hashes{
		EmptyConfig: "empty",
		Inbounds: []InboundHash{{
			Tag:        "VLESS_INBOUND",
			UsersCount: 1,
			Hash:       "h1",
		}},
	}

	runtimeState.SetXrayStarted(&version, currentConfig, hashes)
	currentConfig["inbounds"].([]any)[0].(map[string]any)["tag"] = "MUTATED"
	hashes.Inbounds[0].Tag = "MUTATED"
	version = "mutated"

	snapshot := runtimeState.Snapshot()
	if got := snapshot.CurrentConfig["inbounds"].([]any)[0].(map[string]any)["tag"]; got != "VLESS_INBOUND" {
		t.Fatalf("snapshot config tag = %q, want VLESS_INBOUND", got)
	}
	if snapshot.LastHashes.Inbounds[0].Tag != "VLESS_INBOUND" {
		t.Fatalf("snapshot hash tag = %q, want VLESS_INBOUND", snapshot.LastHashes.Inbounds[0].Tag)
	}
	if snapshot.XrayVersion == nil || *snapshot.XrayVersion != "1.2.3" {
		t.Fatalf("snapshot version = %v, want 1.2.3", snapshot.XrayVersion)
	}

	snapshot.CurrentConfig["inbounds"].([]any)[0].(map[string]any)["tag"] = "SNAPSHOT_MUTATED"
	snapshot.LastHashes.Inbounds[0].Tag = "SNAPSHOT_MUTATED"
	*snapshot.XrayVersion = "snapshot-mutated"

	next := runtimeState.Snapshot()
	if got := next.CurrentConfig["inbounds"].([]any)[0].(map[string]any)["tag"]; got != "VLESS_INBOUND" {
		t.Fatalf("next snapshot config tag = %q, want VLESS_INBOUND", got)
	}
	if next.LastHashes.Inbounds[0].Tag != "VLESS_INBOUND" {
		t.Fatalf("next snapshot hash tag = %q, want VLESS_INBOUND", next.LastHashes.Inbounds[0].Tag)
	}
	if next.XrayVersion == nil || *next.XrayVersion != "1.2.3" {
		t.Fatalf("next snapshot version = %v, want 1.2.3", next.XrayVersion)
	}
}
