package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/x-dora/rw-node-go/internal/contractdiff"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "contract diff failed: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	var (
		tag           string
		baselinePath  string
		repo          string
		sourceDir     string
		writeBaseline bool
	)

	flags := flag.NewFlagSet("contract-diff", flag.ContinueOnError)
	flags.SetOutput(stdout)
	flags.StringVar(&tag, "tag", getenvDefault("CONTRACT_TAG", "2.7.0"), "official remnawave/node tag to check")
	flags.StringVar(&baselinePath, "baseline", "testdata/contracts/official-2.7.0/upstream-contract.sha256.json", "baseline manifest path")
	flags.StringVar(&repo, "repo", "remnawave/node", "GitHub repository in owner/name form")
	flags.StringVar(&sourceDir, "source-dir", getenvDefault("CONTRACT_SOURCE_DIR", ""), "local remnawave/node checkout to scan instead of downloading a GitHub tarball")
	flags.BoolVar(&writeBaseline, "write-baseline", false, "write the scanned manifest to -baseline instead of comparing")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if tag == "" {
		return fmt.Errorf("-tag must not be empty")
	}
	if baselinePath == "" {
		return fmt.Errorf("-baseline must not be empty")
	}
	if repo == "" {
		return fmt.Errorf("-repo must not be empty")
	}

	var root string
	var cleanup func()
	var err error
	if sourceDir != "" {
		root = sourceDir
		cleanup = func() {}
	} else {
		root, cleanup, err = downloadRepository(tag, repo)
		if err != nil {
			return err
		}
	}
	defer cleanup()

	source := "github.com/" + strings.TrimPrefix(repo, "github.com/")
	current, err := contractdiff.ScanRepository(root, source, tag, time.Now().UTC())
	if err != nil {
		return err
	}

	if writeBaseline {
		if err := contractdiff.SaveManifest(baselinePath, current); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "wrote contract baseline %s for %s %s (%d files)\n", baselinePath, source, tag, len(current.Files))
		return nil
	}

	baseline, err := contractdiff.LoadManifest(baselinePath)
	if err != nil {
		return err
	}

	diff := contractdiff.Compare(baseline, current)
	if !diff.HasChanges() {
		fmt.Fprintf(stdout, "contract unchanged: baseline %s, checked %s, scanned %d files\n", baseline.Tag, tag, len(current.Files))
		return nil
	}

	fmt.Fprintf(stdout, "official contract drift detected: baseline %s, checked %s\n", baseline.Tag, tag)
	if len(diff.Added) > 0 {
		fmt.Fprintln(stdout, "added files:")
		for _, file := range diff.Added {
			fmt.Fprintf(stdout, "  + %s %s\n", file.Path, file.SHA256)
		}
	}
	if len(diff.Deleted) > 0 {
		fmt.Fprintln(stdout, "deleted files:")
		for _, file := range diff.Deleted {
			fmt.Fprintf(stdout, "  - %s %s\n", file.Path, file.SHA256)
		}
	}
	if len(diff.Changed) > 0 {
		fmt.Fprintln(stdout, "changed files:")
		for _, file := range diff.Changed {
			fmt.Fprintf(stdout, "  * %s %s -> %s\n", file.Path, file.Baseline, file.Current)
		}
	}
	return fmt.Errorf("official contract changed; update Go contracts, routes, golden fixtures, and baseline after review")
}

func getenvDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func downloadRepository(tag string, repo string) (string, func(), error) {
	tmp, err := os.MkdirTemp("", "rw-node-go-contract-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp directory: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(tmp)
	}

	url := fmt.Sprintf("https://github.com/%s/archive/refs/tags/%s.tar.gz", strings.TrimPrefix(repo, "github.com/"), tag)
	resp, err := http.Get(url)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("download GitHub tarball %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cleanup()
		return "", nil, fmt.Errorf("download GitHub tarball %s: HTTP %d", url, resp.StatusCode)
	}

	root, err := extractTarGzip(tmp, resp.Body)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("extract GitHub tarball for %s %s: %w", repo, tag, err)
	}
	return root, cleanup, nil
}

func extractTarGzip(dest string, input io.Reader) (string, error) {
	gzipReader, err := gzip.NewReader(input)
	if err != nil {
		return "", err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		cleanName := filepath.Clean(filepath.FromSlash(header.Name))
		if cleanName == "." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) || filepath.IsAbs(cleanName) {
			return "", fmt.Errorf("unsafe tar path %q", header.Name)
		}

		target := filepath.Join(dest, cleanName)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fsMode(header.FileInfo().Mode()))
			if err != nil {
				return "", err
			}
			_, copyErr := io.Copy(file, tarReader)
			closeErr := file.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
		}
	}
	return findExtractedRoot(dest)
}

func fsMode(mode os.FileMode) os.FileMode {
	if mode == 0 {
		return 0o644
	}
	return mode.Perm()
}

func findExtractedRoot(dest string) (string, error) {
	entries, err := os.ReadDir(dest)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dest, entry.Name())
		if _, err := os.Stat(filepath.Join(path, "libs", "contract", "api")); err == nil {
			return path, nil
		}
		dirs = append(dirs, path)
	}
	if len(dirs) == 1 {
		return dirs[0], nil
	}
	return "", fmt.Errorf("tarball did not contain a recognizable repository root")
}
