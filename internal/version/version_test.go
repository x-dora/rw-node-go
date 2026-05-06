package version

import "testing"

func TestVersionSeparation(t *testing.T) {
	if ProjectVersion == "" {
		t.Fatal("ProjectVersion is empty")
	}
	if NodeVersion != "2.7.0" {
		t.Fatalf("NodeVersion = %q, want 2.7.0", NodeVersion)
	}
	if ProjectVersion == NodeVersion {
		t.Fatalf("ProjectVersion = %q, want a distinct project release version", ProjectVersion)
	}
}
