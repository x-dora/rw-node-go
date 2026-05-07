package buildmeta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadProjectVersion(t *testing.T) {
	repoRoot := t.TempDir()
	versionPath := filepath.Join(repoRoot, VersionFileName)
	if err := os.WriteFile(versionPath, []byte(" 1.0.1 \n"), 0o600); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	got, err := ReadProjectVersion(repoRoot)
	if err != nil {
		t.Fatalf("ReadProjectVersion() error = %v", err)
	}
	if got != "1.0.1" {
		t.Fatalf("ReadProjectVersion() = %q, want 1.0.1", got)
	}
}

func TestComposeLdflags(t *testing.T) {
	got := ComposeLdflags("1.0.1", "abc123", "2026-05-07T01:02:03Z")
	for _, want := range []string{
		"github.com/x-dora/rw-node-go/internal/version.ProjectVersion=1.0.1",
		"github.com/x-dora/rw-node-go/internal/version.Commit=abc123",
		"github.com/x-dora/rw-node-go/internal/version.BuildDate=2026-05-07T01:02:03Z",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ComposeLdflags() = %q, want to contain %q", got, want)
		}
	}
	if strings.Contains(got, "NodeVersion=") {
		t.Fatalf("ComposeLdflags() = %q, want no NodeVersion override", got)
	}
}
