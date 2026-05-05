package contractdiff

import "sort"

type Diff struct {
	Added   []FileHash
	Deleted []FileHash
	Changed []ChangedFile
}

type ChangedFile struct {
	Path     string
	Baseline string
	Current  string
}

func Compare(baseline Manifest, current Manifest) Diff {
	baseFiles := map[string]string{}
	for _, file := range baseline.Files {
		baseFiles[file.Path] = file.SHA256
	}

	currentFiles := map[string]string{}
	for _, file := range current.Files {
		currentFiles[file.Path] = file.SHA256
	}

	diff := Diff{}
	for path, hash := range currentFiles {
		baseHash, ok := baseFiles[path]
		if !ok {
			diff.Added = append(diff.Added, FileHash{Path: path, SHA256: hash})
			continue
		}
		if baseHash != hash {
			diff.Changed = append(diff.Changed, ChangedFile{
				Path:     path,
				Baseline: baseHash,
				Current:  hash,
			})
		}
	}
	for path, hash := range baseFiles {
		if _, ok := currentFiles[path]; !ok {
			diff.Deleted = append(diff.Deleted, FileHash{Path: path, SHA256: hash})
		}
	}

	sort.Slice(diff.Added, func(i, j int) bool { return diff.Added[i].Path < diff.Added[j].Path })
	sort.Slice(diff.Deleted, func(i, j int) bool { return diff.Deleted[i].Path < diff.Deleted[j].Path })
	sort.Slice(diff.Changed, func(i, j int) bool { return diff.Changed[i].Path < diff.Changed[j].Path })
	return diff
}

func (d Diff) HasChanges() bool {
	return len(d.Added) > 0 || len(d.Deleted) > 0 || len(d.Changed) > 0
}
