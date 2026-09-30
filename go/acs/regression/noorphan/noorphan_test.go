//go:build acs

package noorphan

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var excludedDirs = map[string]bool{
	".git":         true,
	".evolve":      true,
	".claude":      true,
	"vendor":       true,
	"node_modules": true,
	"dist":         true,
}

var allowedScripts = map[string]bool{
	"install.sh": true,
}

func findOrphanScripts(root string) ([]string, error) {
	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) && info != nil && info.IsDir() {
				fmt.Fprintf(os.Stderr, "[noorphan] NOTE: %s denied by the sandbox — skipped\n", path)
				return filepath.SkipDir
			}
			return err
		}
		if info.IsDir() {
			if excludedDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".sh", ".py":
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			if allowedScripts[filepath.ToSlash(rel)] {
				return nil
			}
			offenders = append(offenders, rel)
		}
		return nil
	})
	return offenders, err
}

func TestNoOrphanScripts(t *testing.T) {
	offenders, err := findOrphanScripts(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf("found %d orphan script(s) — the bash→Go migration forbids *.sh/*.py "+
			"(excluding %v; allowlisted: %v); delete or port to Go:\n  %s",
			len(offenders), sortedKeys(excludedDirs), sortedKeys(allowedScripts), strings.Join(offenders, "\n  "))
	}
}

func TestOrphanAllowlist(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"install.sh", "stray.sh", "tool.py"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	offenders, err := findOrphanScripts(dir)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(offenders)
	want := []string{"stray.sh", "tool.py"}
	if !reflect.DeepEqual(offenders, want) {
		t.Errorf("offenders = %v, want %v (install.sh allowlisted; stray.sh + tool.py still caught)", offenders, want)
	}
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
