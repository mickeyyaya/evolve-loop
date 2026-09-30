//go:build acs

package cycle1181

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const resolutionDocRel = "docs/operations/carryover-resolutions/todo-quarantine-dead-lane-code.md"

const carryoverID = "todo-quarantine-dead-lane-code"

const stateRevisionPinned = 1753

const statemapRevisionAtRetirement = 45

func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

func TestC1181_001_ResolutionDocRecordsNoOpVerdictWithEvidence(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, resolutionDocRel)

	if !acsassert.FileExists(t, doc) {
		t.Fatalf("%s is missing; the carryover entry %q is retired by a durable resolution doc, not by a commit message — this is the cycle-1164 failure (the answer existed only in a stale worktree)", resolutionDocRel, carryoverID)
	}

	body, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("read %s: %v", resolutionDocRel, err)
	}
	if len(body) < 800 {
		t.Errorf("%s is %d bytes; a resolution doc that cannot fit the verdict plus its evidence citations is a stub, and a stub re-opens the question next cycle", resolutionDocRel, len(body))
	}

	for _, want := range []struct{ needle, why string }{
		{carryoverID, "the doc must name the carryover entry it retires so the next reader can match it to state.json"},
		{"menu-pass-preserve-committed-ids", "one of the two scoped ids the entry asks about"},
		{"carryforward-filter-wire-fleet-rebase", "the other scoped id the entry asks about"},
		{"lane_menu.go", "the landed-code evidence for menu-pass-preserve-committed-ids"},
		{"carryforward_filter.go", "the landed-code evidence for carryforward-filter-wire-fleet-rebase"},
		{"9eacd83f", "the commit that named+exercised the fleet-rebase classify surface on main"},
	} {
		if !acsassert.FileContains(t, doc, want.needle) {
			t.Errorf("%s does not mention %q — %s", resolutionDocRel, want.needle, want.why)
		}
	}

	if !acsassert.FileMatchesRegex(t, doc, `(?i)no-?op`) {
		t.Errorf("%s does not state a no-op verdict; both scoped ids landed, so the resolution IS 'no marker warranted' and the doc must say so explicitly rather than leave the reader to infer it", resolutionDocRel)
	}
	if !acsassert.FileMatchesRegex(t, doc, `(?i)either/or`) {
		t.Errorf("%s does not record that the entry's either/or framing is stale (neither branch is dead); without that note a future reader re-derives the same dead end", resolutionDocRel)
	}

	for _, src := range []string{
		"go/internal/triagecap/lane_menu.go",
		"go/internal/core/carryforward_filter.go",
	} {
		if !acsassert.FileNotContains(t, filepath.Join(root, src), "QUARANTINED-DEAD") {
			t.Errorf("%s carries a QUARANTINED-DEAD marker, but the resolution verdict is no-op — marking live, wired code dead is the inverse defect the carryover entry was meant to prevent", src)
		}
	}
}

func TestC1181_002_BothScopedLanesAreLiveNotDead(t *testing.T) {
	t.Run("menu_pass_preserves_committed_ids", func(t *testing.T) {
		root := t.TempDir()
		evolveDir := filepath.Join(root, ".evolve")
		inbox := filepath.Join(evolveDir, "inbox")

		writeInboxItem(t, inbox, "heavy-a", 0.95)
		writeInboxItem(t, inbox, "heavy-b", 0.94)
		writeInboxItem(t, inbox, "light-committed", 0.10)
		writeInboxItem(t, filepath.Join(inbox, "processed"), "consumed-committed", 0.99)

		committed := []triagecap.FleetCandidate{
			{ID: "light-committed", Weight: 0.10, Files: []string{"go/internal/a/a.go"}},
			{ID: "consumed-committed", Weight: 0.99, Files: []string{"go/internal/b/b.go"}},
		}

		got := menuIDs(triagecap.SelectWaveSeedMenus(evolveDir, committed, 2, 2, nil))

		if !got["light-committed"] {
			t.Errorf("SelectWaveSeedMenus dropped committed id 'light-committed' (weight 0.10) in favour of heavier backlog — the seed is going through the committed-BLIND path, i.e. menu-pass-preserve-committed-ids is NOT landed and the doc's no-op verdict is false")
		}
		if got["consumed-committed"] {
			t.Errorf("SelectWaveSeedMenus re-pinned 'consumed-committed' (already in inbox/processed/) — preserving committed ids must not preserve CONSUMED ones; that is the cycle-1116 re-pin defect")
		}
	})

	t.Run("fleet_rebase_classifier_is_wired", func(t *testing.T) {
		ctx := context.Background()
		dir := newGitRepo(t)

		gitRun(t, dir, "checkout", "-b", "cand")
		writeAndCommit(t, dir, "feature.txt", "feature\n", "feat: candidate work")
		candSha := gitOut(t, dir, "rev-parse", "HEAD")
		gitRun(t, dir, "checkout", "main")
		gitRun(t, dir, "cherry-pick", candSha)

		verdict, err := core.ClassifyFleetRebaseCandidate(ctx, dir, "cand", "main")
		if err != nil {
			t.Fatalf("ClassifyFleetRebaseCandidate(already-landed): %v — the fleet-rebase classify surface must be callable, else carryforward-filter-wire-fleet-rebase never landed", err)
		}
		if verdict != core.FleetRebaseAlreadyLanded {
			t.Errorf("ClassifyFleetRebaseCandidate(cherry-picked candidate) = %v; want FleetRebaseAlreadyLanded — an already-absorbed candidate must short-circuit instead of burning a replay+re-audit (the 948 PASS-but-unlanded duplicate waste)", verdict)
		}

		conflictDir := newGitRepo(t)
		writeAndCommit(t, conflictDir, "shared.txt", "base\n", "chore: shared base")
		gitRun(t, conflictDir, "checkout", "-b", "cand")
		writeAndCommit(t, conflictDir, "shared.txt", "candidate edit\n", "feat: candidate edit")
		gitRun(t, conflictDir, "checkout", "main")
		writeAndCommit(t, conflictDir, "shared.txt", "main edit\n", "feat: divergent main edit")

		conflictVerdict, err := core.ClassifyFleetRebaseCandidate(ctx, conflictDir, "cand", "main")
		if err != nil {
			t.Fatalf("ClassifyFleetRebaseCandidate(conflicting): %v; a genuine conflict is a verdict, never an error", err)
		}
		if conflictVerdict != core.FleetRebaseConflict {
			t.Errorf("ClassifyFleetRebaseCandidate(conflicting candidate) = %v; want FleetRebaseConflict — labelling a real 3-way conflict as AlreadyLanded silently drops overlapping work", conflictVerdict)
		}
	})
}

func TestC1181_003_CarryoverEntryRetiredFromLiveState(t *testing.T) {
	statePath := filepath.Join(stateRoot(t), ".evolve", "state.json")
	body, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read live state.json at %s: %v", statePath, err)
	}

	var state struct {
		CarryoverTodos []struct {
			ID string `json:"id"`
		} `json:"carryoverTodos"`
		StateRevision    int `json:"stateRevision"`
		StatemapRevision int `json:"statemapRevision"`
	}
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatalf("parse live state.json: %v", err)
	}

	for _, todo := range state.CarryoverTodos {
		if todo.ID == carryoverID {
			t.Errorf("%q is STILL present in the live .evolve/state.json:carryoverTodos after the cycle — the resolution doc is prose until the state actually loses the entry; a doc-only close is the D1 HIGH defect that sank cycle 1164 and the entry gets re-picked next triage", carryoverID)
			break
		}
	}

	if n := len(state.CarryoverTodos); n < 20 {
		t.Errorf("live carryoverTodos has %d entries; the cycle must retire exactly ONE id, not truncate the backlog — a wiped list greens a naive absence check while destroying tracked work", n)
	}
	if state.StateRevision < stateRevisionPinned {
		t.Errorf("live stateRevision = %d; want >= %d — stateRevision bumps unconditionally at cycle end, so it must be at least the pre-cycle baseline. A value below it indicates state corruption", state.StateRevision, stateRevisionPinned)
	}
	if state.StatemapRevision < statemapRevisionAtRetirement {
		t.Errorf("live statemapRevision = %d; want >= %d — this is the counter the sanctioned locked read-modify-write actually advances, so a value below the post-retirement reading means carryoverTodos lost the entry by hand-edit rather than through statemap", state.StatemapRevision, statemapRevisionAtRetirement)
	}
}

func writeInboxItem(t *testing.T, dir, id string, weight float64) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	body, err := json.MarshalIndent(map[string]any{
		"id": id, "title": "fixture " + id, "kind": "bug", "weight": weight,
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal item %s: %v", id, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-07-29T00-00-00Z-"+id+".json"), body, 0o644); err != nil {
		t.Fatalf("write item %s: %v", id, err)
	}
}

func menuIDs(menus [][]triagecap.FleetCandidate) map[string]bool {
	out := map[string]bool{}
	for _, menu := range menus {
		for _, c := range menu {
			out[c.ID] = true
		}
	}
	return out
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "config", "user.email", "acs@example.test")
	gitRun(t, dir, "config", "user.name", "ACS Fixture")
	writeAndCommit(t, dir, "README.md", "base\n", "chore: base")
	return dir
}

func writeAndCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	gitRun(t, dir, "add", name)
	gitRun(t, dir, "commit", "-q", "-m", msg)
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(regexp.MustCompile(`\s+$`).ReplaceAll(out, nil))
}
