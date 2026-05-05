package contractdiff

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompareReportsAddedDeletedAndChangedFiles(t *testing.T) {
	baseline := Manifest{
		Files: []FileHash{
			{Path: "libs/contract/api/routes.ts", SHA256: "same"},
			{Path: "libs/contract/commands/stats/get-users-stats.command.ts", SHA256: "old"},
			{Path: "libs/contract/models/xray-webhook.schema.ts", SHA256: "deleted"},
		},
	}
	current := Manifest{
		Files: []FileHash{
			{Path: "libs/contract/api/routes.ts", SHA256: "same"},
			{Path: "libs/contract/commands/stats/get-users-stats.command.ts", SHA256: "new"},
			{Path: "libs/contract/models/new.schema.ts", SHA256: "added"},
		},
	}

	diff := Compare(baseline, current)
	if !diff.HasChanges() {
		t.Fatalf("expected changes")
	}
	if len(diff.Added) != 1 || diff.Added[0].Path != "libs/contract/models/new.schema.ts" {
		t.Fatalf("unexpected added files: %#v", diff.Added)
	}
	if len(diff.Deleted) != 1 || diff.Deleted[0].Path != "libs/contract/models/xray-webhook.schema.ts" {
		t.Fatalf("unexpected deleted files: %#v", diff.Deleted)
	}
	if len(diff.Changed) != 1 || diff.Changed[0].Path != "libs/contract/commands/stats/get-users-stats.command.ts" {
		t.Fatalf("unexpected changed files: %#v", diff.Changed)
	}
}

func TestCompareReturnsNoChangesForIdenticalManifests(t *testing.T) {
	baseline := Manifest{Files: []FileHash{{Path: "libs/contract/api/routes.ts", SHA256: "same"}}}
	current := Manifest{Files: []FileHash{{Path: "libs/contract/api/routes.ts", SHA256: "same"}}}

	diff := Compare(baseline, current)
	if diff.HasChanges() {
		t.Fatalf("expected no changes: %#v", diff)
	}
}

func TestScanRepositoryIncludesOnlyPanelFacingContractFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "libs/contract/api/routes.ts", "routes")
	writeFile(t, root, "libs/contract/api/index.ts", "index")
	writeFile(t, root, "libs/contract/commands/handler/add-user.command.ts", "add")
	writeFile(t, root, "libs/contract/constants/errors/errors.ts", "errors")
	writeFile(t, root, "libs/contract/constants/xray/stats.ts", "stats")
	writeFile(t, root, "libs/contract/models/xray-webhook.schema.ts", "webhook")
	writeFile(t, root, "libs/contract/package.json", "{}")
	writeFile(t, root, "src/modules/handler/handler.controller.ts", "not contract")

	manifest, err := ScanRepository(root, DefaultSource, "2.7.0", time.Unix(0, 0))
	if err != nil {
		t.Fatalf("scan repository: %v", err)
	}

	paths := make([]string, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		paths = append(paths, file.Path)
	}
	want := []string{
		"libs/contract/api/routes.ts",
		"libs/contract/commands/handler/add-user.command.ts",
		"libs/contract/constants/errors/errors.ts",
		"libs/contract/constants/xray/stats.ts",
		"libs/contract/models/xray-webhook.schema.ts",
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %#v, want %#v", paths, want)
		}
	}
}

func TestScanRepositoryFailsWhenContractDirectoryIsMissing(t *testing.T) {
	_, err := ScanRepository(t.TempDir(), DefaultSource, "2.7.0", time.Unix(0, 0))
	if err == nil {
		t.Fatalf("expected missing directory error")
	}
}

func writeFile(t *testing.T, root string, rel string, data string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
