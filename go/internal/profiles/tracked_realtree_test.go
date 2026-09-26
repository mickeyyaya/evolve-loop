package profiles

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

// realProfilesDir is the repo's live .evolve/profiles. Go's test cache does not
// track reads outside the module, so run go test -count=1 after a profile edit.
func realProfilesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".evolve", "profiles")
}

// TrackedRealProfileNames returns the git-tracked profile names; nil means bind every profile.
func TrackedRealProfileNames(t *testing.T) map[string]bool {
	t.Helper()
	root := filepath.Join(realProfilesDir(t), "..", "..")
	set, err := repostate.TrackedSet(root, ".evolve/profiles", ".json")
	if err == nil && len(set) == 0 {
		err = fmt.Errorf("empty tracked-profile set at %s — pathspec matched nothing (misresolved root or sparse checkout)", root)
	}
	if err != nil {
		t.Logf("TrackedRealProfileNames: %v — binding all on-disk profiles", err)
		return nil
	}
	return set
}

// RealTreeProfiles returns a Loader over the live .evolve/profiles and its git-tracked names.
// Real-tree scans go through it because the runtime mints untracked stubs no CI checkout has.
func RealTreeProfiles(t *testing.T) (*Loader, []string) {
	t.Helper()
	return treeProfiles(t, filepath.Join(realProfilesDir(t), "..", ".."))
}

// treeProfiles is RealTreeProfiles for an arbitrary repo root.
func treeProfiles(t *testing.T, root string) (*Loader, []string) {
	t.Helper()
	l := NewFromDir(filepath.Join(root, ".evolve", "profiles"))
	names, err := l.List()
	if err != nil {
		t.Fatalf("List profiles under %s: %v", root, err)
	}
	tracked, err := repostate.TrackedSet(root, ".evolve/profiles", ".json")
	if err == nil && len(tracked) == 0 {
		err = fmt.Errorf("empty tracked-profile set at %s — pathspec matched nothing (misresolved root or sparse checkout)", root)
	}
	if err != nil {
		t.Logf("TrackedSet: %v — binding all on-disk profiles", err)
		return l, names
	}
	kept := make([]string, 0, len(names))
	for _, n := range names {
		if !tracked[n] {
			t.Logf("untracked profile %q: runtime-minted state, not bound", n)
			continue
		}
		kept = append(kept, n)
	}
	return l, kept
}

func TestRealTreeProfiles_ExcludesUntrackedDecoy(t *testing.T) {
	tracked := TrackedRealProfileNames(t)
	if tracked == nil {
		t.Skip("no usable git context — filter disabled (bind-all fallback), nothing to prove")
	}
	// Phase sandboxes deny writes to the live .evolve/profiles, so the decoy goes in a mirror.
	root := mirrorTrackedProfiles(t, filepath.Join(realProfilesDir(t), "..", ".."), tracked)
	const decoy = "zz-decoy-mint-profiles-funnel"
	payload := `{"name":"` + decoy + `","role":"decoy","cli":"claude","model_tier_default":"fast"}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "profiles", decoy+".json"), []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	_, live := RealTreeProfiles(t)
	_, names := treeProfiles(t, root)
	if len(names) != len(live) || len(names) == 0 {
		t.Fatalf("filtered mirror list = %d, want the live funnel's %d — the funnel went dark or the mirror lost a profile", len(names), len(live))
	}
	for _, n := range names {
		if n == decoy {
			t.Fatalf("untracked decoy %q bound by the funnel — the cd49274beab2 false-RED class is re-armed", decoy)
		}
	}
}

// mirrorTrackedProfiles copies the tracked profiles into a fresh committed git repo and returns its root.
func mirrorTrackedProfiles(t *testing.T, real string, tracked map[string]bool) string {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "t")
	profDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name := range tracked {
		body, err := os.ReadFile(filepath.Join(real, ".evolve", "profiles", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(profDir, name+".json"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".evolve/profiles")
	git("commit", "-q", "-m", "mirror tracked profiles")
	return root
}
