package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/x-dora/rw-node-go/internal/buildmeta"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "rw-build failed: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var (
		outputPath  string
		packagePath string
		repoRoot    string
		commit      string
		buildDate   string
		goOS        string
		goARCH      string
	)

	flags := flag.NewFlagSet("rw-build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&outputPath, "o", filepath.Join("bin", "rw-node-go"), "output binary path")
	flags.StringVar(&packagePath, "package", "./cmd/rw-node-go", "Go package path to build")
	flags.StringVar(&repoRoot, "repo-root", ".", "repository root containing VERSION")
	flags.StringVar(&commit, "commit", envOrFallback("COMMIT", ""), "git commit to inject into build metadata")
	flags.StringVar(&buildDate, "build-date", envOrFallback("BUILD_DATE", time.Now().UTC().Format(time.RFC3339)), "build date to inject into build metadata")
	flags.StringVar(&goOS, "goos", runtime.GOOS, "target GOOS for the built binary")
	flags.StringVar(&goARCH, "goarch", runtime.GOARCH, "target GOARCH for the built binary")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if outputPath == "" {
		return fmt.Errorf("-o must not be empty")
	}
	outputPath = normalizeOutputPath(outputPath, goOS)
	if packagePath == "" {
		return fmt.Errorf("-package must not be empty")
	}

	projectVersion, err := buildmeta.ReadProjectVersion(repoRoot)
	if err != nil {
		return err
	}
	if commit == "" {
		commit = gitCommit(repoRoot)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	ldflags := buildmeta.ComposeLdflags(projectVersion, commit, buildDate)
	goArgs := []string{"build", "-trimpath", "-buildvcs=false", "-o", outputPath, "-ldflags", ldflags, packagePath}
	cmd := exec.Command("go", goArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(
		filterEnv(os.Environ(), "CGO_ENABLED", "GOOS", "GOARCH"),
		"CGO_ENABLED=0",
		"GOOS="+goOS,
		"GOARCH="+goARCH,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}

	return nil
}

func envOrFallback(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func normalizeOutputPath(path, goOS string) string {
	if goOS == "windows" && filepath.Ext(path) == "" {
		return path + ".exe"
	}
	return path
}

func gitCommit(repoRoot string) string {
	output, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	commit := strings.TrimSpace(string(output))
	if commit == "" {
		return "unknown"
	}
	return commit
}

func filterEnv(env []string, names ...string) []string {
	drop := make(map[string]struct{}, len(names))
	for _, name := range names {
		drop[name] = struct{}{}
	}

	out := make([]string, 0, len(env))
	for _, item := range env {
		key, _, found := strings.Cut(item, "=")
		if !found {
			continue
		}
		if _, exists := drop[key]; exists {
			continue
		}
		out = append(out, item)
	}
	return out
}
