//go:build acs

// Package noorphan is the durable "zero scripts" regression guard that locks in
// the final slice (Wave E) of the bash→Go migration: every .sh and .py file was
// deleted and reimplemented in Go, and this predicate makes that permanent.
//
// It walks the repository tree and fails if ANY *.sh or *.py file reappears,
// excluding the runtime/state and vendored trees that are not source we own
// (.git/, .evolve/, .claude/, vendor/, node_modules/). A reintroduced script —
// a stray helper or a copied-in tool — fails the build with the offending paths
// listed, so the migration cannot silently regress.
//
// The ONE sanctioned exception is the curl|sh bootstrap installer (install.sh,
// in allowedScripts below): it must be shell because it runs before any evolve
// binary exists, to download/build it. The allowlist is keyed by exact
// repo-relative path and kept minimal — a new entry is a deliberate decision.
//
// It is acs-tagged (like every other go/acs/regression predicate) so it runs in
// the live cycle ACS gate, not the fast unit tier. Run it the same way go.yml's
// "test" step compiles the acs tier:
//
//	go test -tags acs ./acs/regression/noorphan/...
//
// It needs no .apicover-enforce / completeness enrollment: it is a test-only
// package outside ./internal/..., exactly like acs/regression/flagreaders.
package noorphan

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// excludedDirs are directory names pruned from the walk: runtime/state trees and
// vendored code that are not first-party source. A match on any path SEGMENT
// (not just a top-level dir) is pruned, so e.g. a nested vendor/ is also skipped.
var excludedDirs = map[string]bool{
	".git":         true,
	".evolve":      true,
	".claude":      true,
	"vendor":       true,
	"node_modules": true,
	"dist":         true, // build output (goreleaser dist/, landing/dist/) — generated, gitignored, not source
}

// allowedScripts are the sanctioned exceptions to the no-scripts rule, keyed by
// exact repo-relative (forward-slash) path. Keep this minimal.
var allowedScripts = map[string]bool{
	"install.sh": true, // curl|sh bootstrap installer — must be shell (runs before any binary exists)
}

// findOrphanScripts walks root and returns the repo-relative paths of every
// .sh/.py file outside excludedDirs and not in allowedScripts.
func findOrphanScripts(root string) ([]string, error) {
	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
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

// TestNoOrphanScripts asserts the repo contains zero *.sh and *.py files outside
// the excluded trees and the allowlist — the permanent "no scripts" invariant
// for the bash→Go migration. A reintroduced script fails here with the offenders.
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

// TestOrphanAllowlist proves the carve-out is tight: an allowlisted install.sh
// is NOT flagged, but a second stray script in the same dir still is — so the
// exception can't silently widen to "scripts are fine again".
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
