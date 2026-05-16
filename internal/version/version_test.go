package version

import "testing"

func TestVersionSeparation(t *testing.T) {
	if ProjectVersion == "" {
		t.Fatal("ProjectVersion is empty")
	}
	if ProjectVersion != "dev" {
		t.Fatalf("ProjectVersion = %q, want dev fallback when not injected", ProjectVersion)
	}
	if NodeVersion != "2.8.0" {
		t.Fatalf("NodeVersion = %q, want 2.8.0", NodeVersion)
	}
	if ProjectVersion == NodeVersion {
		t.Fatalf("ProjectVersion = %q, want a distinct project release version", ProjectVersion)
	}
}
