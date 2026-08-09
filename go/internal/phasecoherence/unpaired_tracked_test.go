package phasecoherence

// unpaired_tracked_test.go — regression pin for the cycle-1402/1403/1405
// ship|gate-block storm (2026-08-09 batch, fingerprint cd49274beab2):
// Direction B of TestRepoPersonaProfilePairing bound EVERY on-disk profile
// by name, so a runtime-minted, gitignored-by-design stub
// (.evolve/profiles/defect-disposition-ledger.json) with no persona red'd the
// ship-time repo-contract pack on the live plane and blocked three
// audit-green lane ships in one batch — the identical-fingerprint ceiling
// then halted the whole batch. A profile git does not track can never reach
// main (CI checkouts do not contain it), so it is runtime state, not repo
// config, and must not be in the Direction-B binding set.

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
	writeProfile("defect-disposition-ledger") // the real incident stub, never added
	writeProfile("any-future-mint")           // arbitrary name: the exclusion is structural, not name-based

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
