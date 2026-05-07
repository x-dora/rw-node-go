package buildmeta

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// VersionFileName is the repository root file that defines the project release version.
	VersionFileName = "VERSION"
)

var semverPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$`)

// ReadProjectVersion reads and validates the project release version from VERSION.
func ReadProjectVersion(repoRoot string) (string, error) {
	versionPath := filepath.Join(repoRoot, VersionFileName)
	raw, err := os.ReadFile(versionPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", versionPath, err)
	}

	projectVersion := strings.TrimSpace(string(raw))
	if projectVersion == "" {
		return "", fmt.Errorf("%s is empty", versionPath)
	}
	if !semverPattern.MatchString(projectVersion) {
		return "", fmt.Errorf("%s must be a semver value, got: %s", versionPath, projectVersion)
	}

	return projectVersion, nil
}

// ComposeLdflags returns the linker flags used to inject build metadata.
func ComposeLdflags(projectVersion, commit, buildDate string) string {
	return strings.Join([]string{
		"-s",
		"-w",
		"-buildid=",
		"-X",
		"github.com/x-dora/rw-node-go/internal/version.ProjectVersion=" + projectVersion,
		"-X",
		"github.com/x-dora/rw-node-go/internal/version.Commit=" + commit,
		"-X",
		"github.com/x-dora/rw-node-go/internal/version.BuildDate=" + buildDate,
	}, " ")
}
