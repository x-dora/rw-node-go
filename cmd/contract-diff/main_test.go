package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesAndChecksLocalBaseline(t *testing.T) {
	root := t.TempDir()
	writeFixtureContract(t, root, "libs/contract/api/routes.ts", "routes")
	writeFixtureContract(t, root, "libs/contract/commands/handler/add-user.command.ts", "add")
	writeFixtureContract(t, root, "libs/contract/constants/errors/errors.ts", "errors")
	writeFixtureContract(t, root, "libs/contract/constants/xray/stats.ts", "stats")
	writeFixtureContract(t, root, "libs/contract/models/xray-webhook.schema.ts", "webhook")

	baseline := filepath.Join(t.TempDir(), "baseline.json")
	var output bytes.Buffer
	err := run([]string{
		"-tag", "dev",
		"-source-dir", root,
		"-baseline", baseline,
		"-write-baseline",
	}, &output)
	if err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	if !strings.Contains(output.String(), "wrote contract baseline") {
		t.Fatalf("unexpected write output: %s", output.String())
	}

	output.Reset()
	err = run([]string{
		"-tag", "dev",
		"-source-dir", root,
		"-baseline", baseline,
	}, &output)
	if err != nil {
		t.Fatalf("check baseline: %v", err)
	}
	if !strings.Contains(output.String(), "contract unchanged") {
		t.Fatalf("unexpected check output: %s", output.String())
	}
}

func TestRunReportsLocalDrift(t *testing.T) {
	root := t.TempDir()
	writeFixtureContract(t, root, "libs/contract/api/routes.ts", "routes")
	writeFixtureContract(t, root, "libs/contract/commands/handler/add-user.command.ts", "add")
	writeFixtureContract(t, root, "libs/contract/constants/errors/errors.ts", "errors")
	writeFixtureContract(t, root, "libs/contract/constants/xray/stats.ts", "stats")
	writeFixtureContract(t, root, "libs/contract/models/xray-webhook.schema.ts", "webhook")

	baseline := filepath.Join(t.TempDir(), "baseline.json")
	if err := run([]string{"-source-dir", root, "-baseline", baseline, "-write-baseline"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	writeFixtureContract(t, root, "libs/contract/api/routes.ts", "changed")
	var output bytes.Buffer
	err := run([]string{"-source-dir", root, "-baseline", baseline}, &output)
	if err == nil {
		t.Fatalf("expected drift error")
	}
	if !strings.Contains(output.String(), "official contract drift detected") {
		t.Fatalf("unexpected drift output: %s", output.String())
	}
}

func TestRunUsesContractSourceDirEnv(t *testing.T) {
	root := t.TempDir()
	writeFixtureContract(t, root, "libs/contract/api/routes.ts", "routes")
	writeFixtureContract(t, root, "libs/contract/commands/handler/add-user.command.ts", "add")
	writeFixtureContract(t, root, "libs/contract/constants/errors/errors.ts", "errors")
	writeFixtureContract(t, root, "libs/contract/constants/xray/stats.ts", "stats")
	writeFixtureContract(t, root, "libs/contract/models/xray-webhook.schema.ts", "webhook")

	baseline := filepath.Join(t.TempDir(), "baseline.json")
	if err := run([]string{"-source-dir", root, "-baseline", baseline, "-write-baseline"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	t.Setenv("CONTRACT_SOURCE_DIR", root)
	var output bytes.Buffer
	err := run([]string{"-baseline", baseline}, &output)
	if err != nil {
		t.Fatalf("check baseline from env source dir: %v", err)
	}
	if !strings.Contains(output.String(), "contract unchanged") {
		t.Fatalf("unexpected check output: %s", output.String())
	}
}

func writeFixtureContract(t *testing.T, root string, rel string, data string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
