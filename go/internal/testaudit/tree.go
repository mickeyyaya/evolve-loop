package testaudit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const centralBaselinesRelPath = "internal/testaudit/unwritable-baselines.json"

type centralBaselines struct {
	Packages []string  `json:"packages"`
	Rows     []Finding `json:"rows"`
}

func Rebaseline(root, pkg string, cfg Config) error {
	base := cleanPath(root)
	cleanPkg, targetDir, err := validateRebaselineTarget(base, pkg)
	if err != nil {
		return err
	}
	newRows, err := packageFindings(base, cleanPkg, cfg)
	if err != nil {
		return err
	}
	centralPath := filepath.Join(base, filepath.FromSlash(centralBaselinesRelPath))
	cb, cbExists, err := readCentralBaselines(centralPath)
	if err != nil {
		return err
	}
	if cbExists && slices.Contains(cb.Packages, cleanPkg) {
		return rebaselineCentral(centralPath, cb, cleanPkg, newRows)
	}
	return rebaselineLocal(targetDir, cleanPkg, newRows)
}

func CheckTree(root string, cfg Config) error {
	base := cleanPath(root)
	found, err := AnalyzeTree(base, cfg)
	if err != nil {
		return err
	}
	baseline, err := collectTreeBaselines(base)
	if err != nil {
		return err
	}
	return diffFindings(found, baseline)
}

func validateRebaselineTarget(base, pkg string) (string, string, error) {
	if pkg == "" || filepath.IsAbs(pkg) {
		return "", "", fmt.Errorf("testaudit: invalid package path %q", pkg)
	}
	cleanPkg := filepath.ToSlash(filepath.Clean(pkg))
	if cleanPkg == ".." || strings.HasPrefix(cleanPkg, "../") {
		return "", "", fmt.Errorf("testaudit: invalid package path %q", pkg)
	}
	targetDir := resolvePkgDir(base, cleanPkg)
	info, err := os.Stat(targetDir)
	if err != nil {
		return "", "", fmt.Errorf("testaudit: package %s: %w", pkg, err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("testaudit: package %s is not a directory", pkg)
	}
	return cleanPkg, targetDir, nil
}

func resolvePkgDir(base, cleanPkg string) string {
	if base == "." || base == "" {
		return filepath.FromSlash(cleanPkg)
	}
	return filepath.Join(base, filepath.FromSlash(cleanPkg))
}

func rebaselineLocal(targetDir, cleanPkg string, newRows []Finding) error {
	shardPath := filepath.Join(targetDir, filepath.FromSlash(shardRelPath))
	oldRows, exists, err := readShardFile(shardPath)
	if err != nil {
		return err
	}
	if exists {
		if added := addedFindings(oldRows, newRows); len(added) > 0 {
			return growthError(cleanPkg, oldRows, newRows, added)
		}
	}
	if !exists && len(newRows) == 0 {
		return nil
	}
	if exists && slices.Equal(oldRows, newRows) {
		return nil
	}
	return WriteShard(targetDir, newRows)
}

func rebaselineCentral(centralPath string, cb centralBaselines, cleanPkg string, newRows []Finding) error {
	var oldRows, kept []Finding
	for _, r := range cb.Rows {
		if r.Package == cleanPkg {
			oldRows = append(oldRows, r)
			continue
		}
		kept = append(kept, r)
	}
	oldRows = finish(oldRows, Config{})
	if added := addedFindings(oldRows, newRows); len(added) > 0 {
		return growthError(cleanPkg, oldRows, newRows, added)
	}
	if slices.Equal(oldRows, newRows) {
		return nil
	}
	cb.Rows = finish(append(kept, newRows...), Config{})
	slices.Sort(cb.Packages)
	raw, err := json.MarshalIndent(cb, "", "  ")
	if err == nil {
		err = writeAtomic(centralPath, append(raw, '\n'))
	}
	return err
}

func addedFindings(oldRows, newRows []Finding) []string {
	oldSet := make(map[Finding]bool, len(oldRows))
	for _, r := range oldRows {
		oldSet[r] = true
	}
	var added []string
	for _, r := range newRows {
		if !oldSet[r] {
			added = append(added, fmt.Sprintf("%s %s %s", r.Package, r.Test, r.Rule))
		}
	}
	return added
}

func growthError(pkg string, oldRows, newRows []Finding, added []string) error {
	return fmt.Errorf("testaudit: rebaseline %s refused: findings not in baseline (%d -> %d): %s", pkg, len(oldRows), len(newRows), strings.Join(added, "; "))
}

func readCentralBaselines(path string) (centralBaselines, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return centralBaselines{}, false, nil
	}
	if err != nil {
		return centralBaselines{}, false, err
	}
	var cb centralBaselines
	if err := json.Unmarshal(raw, &cb); err != nil {
		return centralBaselines{}, true, fmt.Errorf("testaudit: %s: %w", centralBaselinesRelPath, err)
	}
	return cb, true, nil
}

func collectTreeBaselines(base string) ([]Finding, error) {
	var all []Finding
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != base && skippedDir(d.Name()) {
			return filepath.SkipDir
		}
		rows, _, readErr := readShardFile(filepath.Join(path, filepath.FromSlash(shardRelPath)))
		if readErr != nil {
			return readErr
		}
		all = append(all, rows...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	cb, _, err := readCentralBaselines(filepath.Join(base, filepath.FromSlash(centralBaselinesRelPath)))
	if err != nil {
		return nil, err
	}
	all = append(all, cb.Rows...)
	return all, nil
}

func findModuleRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("testaudit: go.mod not found above working directory")
		}
		dir = parent
	}
}
