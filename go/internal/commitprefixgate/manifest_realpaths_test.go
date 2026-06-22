package commitprefixgate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findRepoRoot walks up from the test cwd to the dir holding the real manifest.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".evolve", "commit-prefix-scope.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate repo root containing .evolve/commit-prefix-scope.json")
	return ""
}

// globBaseDir returns the literal directory a glob pattern is anchored to (the
// path up to the last "/" before the first wildcard), or "" for a top-level
// wildcard (e.g. "**/*.md") that anchors to nothing checkable.
func globBaseDir(pattern string) string {
	star := strings.IndexAny(pattern, "*?[")
	if star < 0 {
		return pattern // literal path (e.g. a root-level file)
	}
	literal := pattern[:star]
	slash := strings.LastIndex(literal, "/")
	if slash < 0 {
		return "" // top-level wildcard — nothing to anchor on
	}
	return literal[:slash]
}

// TestManifestRequiredPaths_ResolveToRealTree guards against the dead-path
// regression the 2026-06-22 doc↔impl audit found (T1.2): after the script→Go
// migration deleted legacy/scripts/ and root acs/, every required_paths glob in
// .evolve/commit-prefix-scope.json still pointed there, so feat(guards) /
// feat(routing) / feat(audit) / feat(posthoc) / feat(token-opt) commits could
// NEVER satisfy Rule 1 ("≥1 diff path must match") — they became un-shippable
// without --bypass-prefix-gate. Every glob's anchor dir must exist in the tree.
func TestManifestRequiredPaths_ResolveToRealTree(t *testing.T) {
	root := findRepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "commit-prefix-scope.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m PrefixManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}
	for prefix, rule := range m.Prefixes {
		paths := append(append([]string{}, rule.RequiredPaths...), rule.ForbiddenOnlyPaths...)
		for _, pat := range paths {
			base := globBaseDir(pat)
			if base == "" {
				continue // top-level wildcard — not anchorable
			}
			if _, err := os.Stat(filepath.Join(root, base)); err != nil {
				t.Errorf("prefix %q: path %q anchors to %q which does not exist (dead manifest path)", prefix, pat, base)
			}
		}
	}
}
