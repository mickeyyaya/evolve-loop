//go:build acs

// Package noorphan is the durable "zero scripts" regression guard that locks in
// the final slice (Wave E) of the bash→Go migration: every .sh and .py file was
// deleted and reimplemented in Go, and this predicate makes that permanent.
//
// It walks the repository tree and fails if ANY *.sh or *.py file reappears,
// excluding the runtime/state and vendored trees that are not source we own
// (.git/, .evolve/, .claude/, vendor/, node_modules/). A reintroduced script —
// whether a stray helper, a copied-in tool, or a regenerated install.sh — fails
// the build with the offending paths listed, so the migration cannot silently
// regress.
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
}

// TestNoOrphanScripts asserts the repo contains zero *.sh and *.py files outside
// the excluded trees — the permanent "no scripts" invariant for the bash→Go
// migration. A reintroduced script fails here with the full offender list.
func TestNoOrphanScripts(t *testing.T) {
	root := acsassert.RepoRoot(t)

	var offenders []string
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
			offenders = append(offenders, rel)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", root, walkErr)
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf("found %d orphan script(s) — the bash→Go migration forbids *.sh/*.py "+
			"(excluding %v); delete or port to Go:\n  %s",
			len(offenders), sortedKeys(excludedDirs), strings.Join(offenders, "\n  "))
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
