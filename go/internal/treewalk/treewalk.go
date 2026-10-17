// Package treewalk snapshots a directory tree and reports the paths a later walk adds.
package treewalk

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type Snapshot struct {
	Paths []string `json:"paths"`
}

func Take(root string) (Snapshot, error) {
	info, err := os.Stat(root)
	if err != nil {
		return Snapshot{}, fmt.Errorf("treewalk: %w", err)
	}
	if !info.IsDir() {
		return Snapshot{}, fmt.Errorf("treewalk: %s is not a directory", root)
	}
	paths := []string{}
	err = filepath.WalkDir(root, func(path string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("treewalk: walk %s: %w", root, err)
	}
	sort.Strings(paths)
	return Snapshot{Paths: paths}, nil
}

func Diff(before, after Snapshot) []string {
	seen := make(map[string]struct{}, len(before.Paths))
	for _, p := range before.Paths {
		seen[p] = struct{}{}
	}
	added := []string{}
	for _, p := range after.Paths {
		if _, ok := seen[p]; !ok {
			added = append(added, p)
		}
	}
	return added
}

func (s Snapshot) Write(path string) error {
	body, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("treewalk: encode snapshot: %w", err)
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("treewalk: write snapshot: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("treewalk: write snapshot: %w", err)
	}
	return nil
}

func Load(path string) (Snapshot, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("treewalk: read baseline: %w", err)
	}
	var s Snapshot
	if err := json.Unmarshal(body, &s); err != nil {
		return Snapshot{}, fmt.Errorf("treewalk: parse baseline %s: %w", path, err)
	}
	return s, nil
}
