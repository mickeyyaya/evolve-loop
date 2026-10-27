package acsassert

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ModuleDir(start string) (string, error) {
	if start == "" {
		return "", errors.New("acsassert.ModuleDir: start is empty")
	}
	info, err := os.Stat(start)
	if err != nil {
		return "", fmt.Errorf("acsassert.ModuleDir: %w", err)
	}
	dir := start
	if !info.IsDir() {
		dir = filepath.Dir(start)
	}
	for {
		if mod, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !mod.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("acsassert.ModuleDir: no go.mod at or above %s", start)
		}
		dir = parent
	}
}

func OutputTail(output string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if n >= len(lines) {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func FuncCoverage(coverFuncOutput, funcName string) (float64, error) {
	lowest, found := 0.0, false
	for _, line := range strings.Split(coverFuncOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != funcName {
			continue
		}
		raw := fields[len(fields)-1]
		pct, err := strconv.ParseFloat(strings.TrimSuffix(raw, "%"), 64)
		if err != nil || !strings.HasSuffix(raw, "%") {
			return 0, fmt.Errorf("acsassert.FuncCoverage: %s has no percentage in %q", funcName, line)
		}
		if !found || pct < lowest {
			lowest, found = pct, true
		}
	}
	if !found {
		return 0, fmt.Errorf("acsassert.FuncCoverage: cover output lists no func %s", funcName)
	}
	return lowest, nil
}

func WalkModule(root string, keep func(rel string) bool) ([]string, error) {
	if root == "" {
		return nil, errors.New("acsassert.WalkModule: root is empty")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("acsassert.WalkModule: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("acsassert.WalkModule: %s is not a directory", root)
	}
	cleanRoot := filepath.Clean(root)
	var files []string
	err = filepath.WalkDir(cleanRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, cleanRoot+string(filepath.Separator)))
		if keep == nil || keep(rel) {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("acsassert.WalkModule: %w", err)
	}
	return files, nil
}
