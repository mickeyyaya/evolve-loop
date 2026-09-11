//go:build integration

package commitprefixgate

import (
	"os/exec"
	"path"
	"strings"
	"testing"
)

func trackedManifestAnchorExists(t *testing.T, root string) func(string) bool {
	t.Helper()
	output, err := exec.Command("git", "-C", root, "ls-files", "--stage", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	anchors := make(map[string]struct{})
	symlinks := make(map[string]string)
	for _, entry := range strings.Split(string(output), "\x00") {
		metadata, tracked, ok := strings.Cut(entry, "\t")
		if !ok || tracked == "" {
			continue
		}
		anchors[tracked] = struct{}{}
		for dir := path.Dir(tracked); dir != "."; dir = path.Dir(dir) {
			anchors[dir] = struct{}{}
		}
		if strings.HasPrefix(metadata, "120000 ") {
			target, err := exec.Command("git", "-C", root, "show", ":"+tracked).Output()
			if err != nil {
				t.Fatalf("read tracked symlink %s: %v", tracked, err)
			}
			symlinks[tracked] = strings.TrimSpace(string(target))
		}
	}
	return func(anchor string) bool {
		return trackedAnchorExists(anchors, symlinks, anchor)
	}
}

// TestManifestRequiredPaths_CleanCheckoutResolvesWithoutGeneratedOutput is the
// clean-checkout contract. It is integration-tagged because Git subprocesses
// are outside the default unit-test cost envelope.
func TestManifestRequiredPaths_CleanCheckoutResolvesWithoutGeneratedOutput(t *testing.T) {
	root := findRepoRoot(t)
	manifest := loadManifest(t, root)
	missing := missingManifestAnchors(manifest, trackedManifestAnchorExists(t, root))
	for prefix, patterns := range missing {
		t.Errorf("prefix %q: %v anchor(s) are not tracked for a clean checkout", prefix, patterns)
	}
}

func TestTrackedManifestAnchorRejectsMissingSymlinkDescendant(t *testing.T) {
	exists := trackedManifestAnchorExists(t, findRepoRoot(t))
	if exists(".agents/skills/loop/__missing_clean_checkout_anchor__") {
		t.Fatal("tracked symlink accepted a descendant missing from its tracked target")
	}
}
