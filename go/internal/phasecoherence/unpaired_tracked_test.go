package phasecoherence

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTrackedProfiles_MintedStubIsRuntimeStateNotRepoConfig(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")

	profDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProfile := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(profDir, name+".json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeProfile("auditor")
	git("add", ".evolve/profiles/auditor.json")
	writeProfile("defect-disposition-ledger")
	writeProfile("any-future-mint") // arbitrary name: the exclusion is structural, not name-based

	tracked, err := trackedProfiles(root)
	if err != nil {
		t.Fatalf("trackedProfiles: %v", err)
	}
	if !tracked["auditor"] {
		t.Errorf("tracked profile auditor.json missing from the Direction-B binding set — the drift gate would stop seeing real repo config")
	}
	for _, stub := range []string{"defect-disposition-ledger", "any-future-mint"} {
		if tracked[stub] {
			t.Errorf("untracked minted stub %q is in the Direction-B binding set — it is runtime state that can never red main, binding it re-arms the cd49274beab2 ship-block storm", stub)
		}
	}
}

func TestTrackedProfiles_NonRepoDirFailsLoudly(t *testing.T) {
	if _, err := trackedProfiles(t.TempDir()); err == nil {
		t.Error("trackedProfiles outside a git repo must return an error so the pairing test can fall back to strict all-profiles binding, not silently bind nothing")
	}
}

func TestTrackedProfiles_RealTreePlantedDecoyNotBound(t *testing.T) {
	real := repoRootForPairing(t)
	pre, err := trackedProfiles(real)
	if err != nil || len(pre) == 0 {
		t.Skipf("no usable git context (tracked=%d err=%v) — pairing test binds all profiles, nothing to prove", len(pre), err)
	}
	root := mirrorTrackedProfiles(t, real, pre)
	const decoy = "zz-decoy-mint-phasecoherence"
	if err := os.WriteFile(filepath.Join(root, ".evolve", "profiles", decoy+".json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tracked, err := trackedProfiles(root)
	if err != nil {
		t.Fatalf("trackedProfiles: %v", err)
	}
	if len(tracked) != len(pre) {
		t.Fatalf("mirror tracked set = %d, want the live set's %d — the mirror must carry every real profile", len(tracked), len(pre))
	}
	if tracked[decoy] {
		t.Fatalf("untracked decoy %q entered the Direction-B binding set — the cd49274beab2 ship-block class is re-armed", decoy)
	}
}

// mirrorTrackedProfiles commits the live tracked profiles into a temp repo so decoys are planted
// there: a phase sandbox denies writes under .evolve/profiles, and tests must not touch tracked config.
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
