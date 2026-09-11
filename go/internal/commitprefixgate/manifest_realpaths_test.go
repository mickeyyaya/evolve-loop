package commitprefixgate

import (
	"encoding/json"
	"os"
	"path"
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
	m := loadManifest(t, root)
	missing := missingManifestAnchors(m, func(anchor string) bool {
		_, err := os.Stat(filepath.Join(root, anchor))
		return err == nil
	})
	for prefix, patterns := range missing {
		for _, pattern := range patterns {
			t.Errorf("prefix %q: path %q anchors to %q which does not exist (dead manifest path)", prefix, pattern, globBaseDir(pattern))
		}
	}
}

func loadManifest(t *testing.T, root string) PrefixManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "commit-prefix-scope.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest PrefixManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}
	return manifest
}

// missingManifestAnchors reports patterns whose literal anchor is unavailable.
// Callers choose whether availability means present on disk or tracked by Git.
func missingManifestAnchors(m PrefixManifest, anchorExists func(string) bool) map[string][]string {
	missing := map[string][]string{}
	for prefix, rule := range m.Prefixes {
		paths := append(append([]string{}, rule.RequiredPaths...), rule.ForbiddenOnlyPaths...)
		for _, pattern := range paths {
			anchor := globBaseDir(pattern)
			if anchor == "" {
				continue // top-level wildcard — not anchorable
			}
			if !anchorExists(anchor) {
				missing[prefix] = append(missing[prefix], pattern)
			}
		}
	}
	return missing
}

func trackedAnchorExists(anchors map[string]struct{}, symlinks map[string]string, anchor string) bool {
	seen := make(map[string]struct{})
	for range 8 {
		if _, ok := anchors[anchor]; ok {
			return true
		}
		if _, repeated := seen[anchor]; repeated {
			return false
		}
		seen[anchor] = struct{}{}

		link := ""
		for candidate := range symlinks {
			if strings.HasPrefix(anchor, candidate+"/") && len(candidate) > len(link) {
				link = candidate
			}
		}
		if link == "" {
			return false
		}
		suffix := strings.TrimPrefix(anchor, link+"/")
		anchor = path.Clean(path.Join(path.Dir(link), symlinks[link], suffix))
	}
	return false
}

func TestTrackedAnchorExistsResolvesSymlinkTarget(t *testing.T) {
	t.Parallel()
	anchors := map[string]struct{}{
		".agents/skills/loop":       {},
		"skills/loop":               {},
		"skills/loop/SKILL.md":      {},
		"skills/loop/examples/demo": {},
	}
	symlinks := map[string]string{".agents/skills/loop": "../../skills/loop"}
	if !trackedAnchorExists(anchors, symlinks, ".agents/skills/loop/SKILL.md") {
		t.Error("tracked symlink target lost its tracked descendant")
	}
	if trackedAnchorExists(anchors, symlinks, ".agents/skills/loop/generated/only-local.md") {
		t.Error("tracked symlink accepted a descendant missing from its tracked target")
	}
}

// TestManifestRequiredPaths_RejectsMissingRequiredAnchor is the negative
// canary for missingManifestAnchors: an entry whose anchor is genuinely absent
// MUST still be flagged. Without this, a future refactor of the anchor check
// (e.g. one that silently skips unresolvable paths) could make both real-tree
// and clean-checkout variants vacuously green.
func TestManifestRequiredPaths_RejectsMissingRequiredAnchor(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Only "go/evolve" exists; "go/bin" is deliberately absent.
	if err := os.WriteFile(filepath.Join(tmp, "go", "evolve"), []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := PrefixManifest{Prefixes: map[string]PrefixRule{
		"chore(build)": {RequiredPaths: []string{"go/evolve", "go/bin/**"}},
	}}
	missing := missingManifestAnchors(m, func(anchor string) bool {
		_, err := os.Stat(filepath.Join(tmp, anchor))
		return err == nil
	})
	if len(missing["chore(build)"]) == 0 {
		t.Errorf("expected missingManifestAnchors to flag go/bin/**; got none")
	}
	for _, pat := range missing["chore(build)"] {
		if pat == "go/evolve" {
			t.Errorf("missingManifestAnchors incorrectly flagged present anchor %q", pat)
		}
	}
}
