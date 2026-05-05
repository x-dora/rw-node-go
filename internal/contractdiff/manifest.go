package contractdiff

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const DefaultSource = "github.com/remnawave/node"

var scanDirs = []string{
	"libs/contract/api",
	"libs/contract/commands",
	"libs/contract/constants/errors",
	"libs/contract/constants/xray",
	"libs/contract/models",
}

type Manifest struct {
	Source      string     `json:"source"`
	Tag         string     `json:"tag"`
	GeneratedAt time.Time  `json:"generatedAt"`
	Files       []FileHash `json:"files"`
}

type FileHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read baseline manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode baseline manifest: %w", err)
	}
	if manifest.Source == "" {
		return Manifest{}, fmt.Errorf("baseline manifest source is empty")
	}
	if manifest.Tag == "" {
		return Manifest{}, fmt.Errorf("baseline manifest tag is empty")
	}
	if len(manifest.Files) == 0 {
		return Manifest{}, fmt.Errorf("baseline manifest has no files")
	}
	return manifest, nil
}

func SaveManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write baseline manifest: %w", err)
	}
	return nil
}

func ScanRepository(root string, source string, tag string, generatedAt time.Time) (Manifest, error) {
	if source == "" {
		source = DefaultSource
	}

	var files []FileHash
	for _, dir := range scanDirs {
		absDir := filepath.Join(root, filepath.FromSlash(dir))
		info, err := os.Stat(absDir)
		if err != nil {
			if os.IsNotExist(err) {
				return Manifest{}, fmt.Errorf("official contract scan path %q is missing; inspect upstream contract structure manually", dir)
			}
			return Manifest{}, fmt.Errorf("stat contract scan path %q: %w", dir, err)
		}
		if !info.IsDir() {
			return Manifest{}, fmt.Errorf("official contract scan path %q is not a directory; inspect upstream contract structure manually", dir)
		}

		err = filepath.WalkDir(absDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !isContractFile(path) {
				return nil
			}
			hash, err := hashFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, FileHash{
				Path:   filepath.ToSlash(rel),
				SHA256: hash,
			})
			return nil
		})
		if err != nil {
			return Manifest{}, fmt.Errorf("scan contract path %q: %w", dir, err)
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return Manifest{
		Source:      source,
		Tag:         tag,
		GeneratedAt: generatedAt.UTC(),
		Files:       files,
	}, nil
}

func isContractFile(path string) bool {
	name := filepath.Base(path)
	if name == "index.ts" {
		return false
	}
	return strings.EqualFold(filepath.Ext(name), ".ts")
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %q: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
