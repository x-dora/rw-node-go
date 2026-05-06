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
