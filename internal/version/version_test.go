package version

import "testing"

func TestVersionSeparation(t *testing.T) {
	if ProjectVersion == "" {
		t.Fatal("ProjectVersion is empty")
	}
	if ProjectVersion != "dev" {
		t.Fatalf("ProjectVersion = %q, want dev fallback when not injected", ProjectVersion)
	}
	if NodeVersion != "3.4.1" {
		t.Fatalf("NodeVersion = %q, want 3.4.1", NodeVersion)
	}
	if ProjectVersion == NodeVersion {
		t.Fatalf("ProjectVersion = %q, want a distinct project release version", ProjectVersion)
	}
}
