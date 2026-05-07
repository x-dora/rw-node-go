package main

import "testing"

func TestNormalizeOutputPathAddsWindowsExecutableSuffix(t *testing.T) {
	got := normalizeOutputPath("bin/rw-node-go", "windows")
	if got != "bin/rw-node-go.exe" {
		t.Fatalf("normalizeOutputPath() = %q, want bin/rw-node-go.exe", got)
	}
}

func TestNormalizeOutputPathKeepsExplicitSuffix(t *testing.T) {
	got := normalizeOutputPath("bin/rw-node-go.exe", "windows")
	if got != "bin/rw-node-go.exe" {
		t.Fatalf("normalizeOutputPath() = %q, want bin/rw-node-go.exe", got)
	}
}

func TestNormalizeOutputPathKeepsUnixPath(t *testing.T) {
	got := normalizeOutputPath("bin/rw-node-go", "linux")
	if got != "bin/rw-node-go" {
		t.Fatalf("normalizeOutputPath() = %q, want bin/rw-node-go", got)
	}
}
