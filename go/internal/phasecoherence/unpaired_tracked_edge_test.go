package phasecoherence

// unpaired_tracked_edge_test.go — edge-case pins for the #421 tracked-only
// Direction-B binding, added with the 2026-08-09 zero-ship batch postmortem
// (docs/incidents/2026-08-09-zero-ship-batch.md). The base regression file
// (unpaired_tracked_test.go) pins tracked-bound / untracked-unbound / loud
// non-repo error; these close the corners the adversarial review flagged:
//   - error fidelity: the wrapped error must carry git's stderr ("not a git
//     repository"), not just "exit status 128" — that line is the fallback
//     diagnostic an operator reads during an incident.
//   - empty tracked set: a repo with NO tracked profiles returns an empty
//     set + nil error — the input that must trigger the caller's loud
//     bind-all fallback rather than silently unbinding the gate.
//   - staged-but-uncommitted counts as tracked (the stricter direction; no
//     CI-vs-plane skew that weakens the gate).
//   - nested tracked profiles must NOT alias a same-named top-level stub
//     into the binding set (basename collision).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initProfileRepo(t *testing.T) (root string, git func(...string)) {
	t.Helper()
	root = t.TempDir()
	git = func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, git
}

func writeProfileAt(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedProfiles_ErrorCarriesGitStderr(t *testing.T) {
	_, err := trackedProfiles(t.TempDir())
	if err == nil {
		t.Fatal("non-repo dir must error")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error %q must carry git's stderr reason — \"exit status 128\" alone is the uninformative diagnostic the review blocked", err)
	}
}

func TestTrackedProfiles_RepoWithNoTrackedProfilesReturnsEmptySetNilError(t *testing.T) {
	root, _ := initProfileRepo(t)
	writeProfileAt(t, root, ".evolve/profiles/minted-only.json") // never added

	set, err := trackedProfiles(root)
	if err != nil {
		t.Fatalf("a repo with zero tracked profiles is a SUCCESS at this layer: %v", err)
	}
	if len(set) != 0 {
		t.Errorf("set = %v, want empty — the caller's loud empty-set fallback (bind all) depends on this exact shape", set)
	}
}

func TestTrackedProfiles_StagedButUncommittedCountsAsTracked(t *testing.T) {
	root, git := initProfileRepo(t)
	writeProfileAt(t, root, ".evolve/profiles/fresh.json")
	git("add", ".evolve/profiles/fresh.json")

	set, err := trackedProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if !set["fresh"] {
		t.Error("a staged-but-uncommitted profile must count as tracked — the stricter direction; excluding it would open a plane-vs-CI skew that weakens the gate")
	}
}

func TestTrackedProfiles_NestedTrackedProfileDoesNotAliasTopLevelStub(t *testing.T) {
	root, git := initProfileRepo(t)
	writeProfileAt(t, root, ".evolve/profiles/archive/auditor.json")
	git("add", ".evolve/profiles/archive/auditor.json")
	writeProfileAt(t, root, ".evolve/profiles/auditor.json") // untracked top-level stub

	set, err := trackedProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if set["auditor"] {
		t.Error("a nested tracked profile must not alias a same-named UNTRACKED top-level stub into the binding set — Direction B walks only the top level, so the alias would bind a stub the walk then flags as dead config")
	}
}
