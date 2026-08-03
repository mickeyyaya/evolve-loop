//go:build acs

// Package cycle1181 materialises the cycle-1181 acceptance criteria for this
// lane's single triage-COMMITTED (## top_n) task:
//
//	todo-quarantine-dead-lane-code → resolve-todo-quarantine-dead-lane-code
//
// The carryover entry (first seen cycle 1159, cycles_unpicked 3) asks: mark
// whichever of `carryforward-filter-wire-fleet-rebase` / `menu-pass-preserve-
// committed-ids` did NOT land at cycle 1159 with a QUARANTINED-DEAD marker, or
// confirm a no-op if both landed. Scout's forensics say BOTH landed, so the
// verdict is no-op and the deliverable is a durable resolution doc plus the
// sanctioned removal of the entry from the live .evolve/state.json.
//
// Predicate strategy — the failure mode this cycle must avoid is precisely a
// doc that ASSERTS a verdict nothing checked (the D1 HIGH audit note that sank
// cycle 1164: a shipped doc claiming a carryoverTodos removal the live state
// never received). So the predicates split into three independent proofs:
//
//	001 — the doc artifact exists at the canonical path and carries the verdict
//	      and its cited evidence, plus the NEGATIVE half: a no-op verdict means
//	      neither lane's source may actually gain a QUARANTINED-DEAD marker.
//	002 — the doc's no-op verdict is SUBSTANTIATED by driving both cited lanes'
//	      real code (triagecap seed selection; core fleet-rebase classifier) over
//	      isolated temp trees. A doc claiming "both landed" over dead code fails
//	      here even though 001 would pass — this is the anti-assertion predicate.
//	003 — the LIVE state.json actually lost the entry (read independently of the
//	      doc), with anti-gaming guards: the rest of the backlog must survive and
//	      the locked read-modify-write must have bumped stateRevision.
//
// Diversity: 001 positive artifact + a negative source-marker guard, 002 two
// behavioral drivers each with a negative branch (an id that must be dropped /
// a candidate that must classify as Conflict, never as AlreadyLanded), 003 the
// live-state mutation with a wipe-the-list negative.
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

// resolutionDocRel is the canonical path of this cycle's deliverable doc,
// relative to the repo root. It is a SOURCE artifact (it ships with the cycle's
// commit), so it resolves under the worktree root, not the state root.
const resolutionDocRel = "docs/operations/carryover-resolutions/todo-quarantine-dead-lane-code.md"

// carryoverID is the state.json:carryoverTodos entry this cycle must retire.
const carryoverID = "todo-quarantine-dead-lane-code"

// stateRevisionPinned is .evolve/state.json:stateRevision as observed when these
// predicates were authored (TDD phase, cycle 1181).
//
// CORRECTED POST-AUDIT (cycle 1233). Cycle 1181 failed audit because it asserted
// stateRevision MUST BE UNCHANGED. While `evolve carryover apply-decisions --apply`
// leaves stateRevision untouched, the cycle boundary itself unconditionally bumps
// stateRevision via `persistCycleEndState` (storage/updatestate.go). An equality
// assertion is therefore unsatisfiable after the cycle ends, causing permanent
// test failures on all subsequent cycles. The guard is now `>=` the baseline.
const stateRevisionPinned = 1753

// statemapRevisionAtRetirement is `statemapRevision` as read immediately after
// the sanctioned apply. It is the counter the sanctioned path DOES advance, so a
// value below this means the entry left carryoverTodos by some route other than
// statemap's locked read-modify-write (i.e. a hand-edit). Legitimate later
// statemap writes only ever increase it.
const statemapRevisionAtRetirement = 45

// stateRoot resolves the MAIN project root (the STATE root): .evolve/ runtime
// data lives on main, not in the cycle worktree (issue #12 dual-root pattern).
// The ACS suite exports EVOLVE_PROJECT_ROOT; fall back to the repo root.
func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

// --- 001: the resolution doc exists, states the verdict, cites its evidence --

// TestC1181_001_ResolutionDocRecordsNoOpVerdictWithEvidence pins AC-1.
//
// The carryover entry is retired by a DURABLE artifact, not by a commit message:
// the next cycle that meets this question must be able to read why it is closed.
// So the doc must exist at the canonical path, name both scoped ids, state the
// no-op verdict, and cite the two landed-code evidence points scout verified
// (`lane_menu.go` for menu-pass-preserve-committed-ids; `carryforward_filter.go`
// / commit 9eacd83f for carryforward-filter-wire-fleet-rebase) plus the note
// that the entry's either/or framing is itself stale.
//
// Negative half: "no-op" is a claim with a physical consequence — if it is true,
// no QUARANTINED-DEAD marker is warranted, so neither cited source file may
// actually carry one. A cycle that hedged by writing the doc AND stamping a
// marker fails here.
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

	// Verdict + both scoped ids + both evidence points + the staleness note.
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

	// Negative: a no-op verdict forbids an actual QUARANTINED-DEAD marker.
	for _, src := range []string{
		"go/internal/triagecap/lane_menu.go",
		"go/internal/core/carryforward_filter.go",
	} {
		if !acsassert.FileNotContains(t, filepath.Join(root, src), "QUARANTINED-DEAD") {
			t.Errorf("%s carries a QUARANTINED-DEAD marker, but the resolution verdict is no-op — marking live, wired code dead is the inverse defect the carryover entry was meant to prevent", src)
		}
	}
}

// --- 002: the no-op verdict is substantiated by driving both lanes -----------

// TestC1181_002_BothScopedLanesAreLiveNotDead is the cycle-1181 CRUX: it makes
// the doc's central claim falsifiable instead of merely asserted.
//
// A resolution doc is only worth landing if "both landed" is TRUE. Predicate 001
// would pass on a doc that says so over dead code; this one calls the real
// surfaces and asserts their landed behavior:
//
//	(a) menu-pass-preserve-committed-ids — triagecap.SelectWaveSeedMenus must
//	    route committed candidates through the committed-AWARE widen path, so a
//	    committed id survives into the seed even when its weight is below the
//	    backlog's top entries. The committed-blind SelectFleetWidthTopN path
//	    (the pre-fix behavior) drops it.
//	    Negative: an id the inbox lifecycle has already CONSUMED must still be
//	    pruned — "preserve committed" must not have degraded into "preserve
//	    everything", which would re-pin consumed work (the cycle-1116 re-pin).
//
//	(b) carryforward-filter-wire-fleet-rebase — core.ClassifyFleetRebaseCandidate
//	    must exist and classify over a real git tree: an already-landed candidate
//	    reads AlreadyLanded (short-circuit, the 948 duplicate-waste fix).
//	    Negative: a genuinely conflicting candidate must read Conflict, NEVER
//	    AlreadyLanded — mislabelling a conflict as landed silently drops real
//	    overlapping work, so the negative is the load-bearing half.
func TestC1181_002_BothScopedLanesAreLiveNotDead(t *testing.T) {
	t.Run("menu_pass_preserves_committed_ids", func(t *testing.T) {
		root := t.TempDir()
		evolveDir := filepath.Join(root, ".evolve")
		inbox := filepath.Join(evolveDir, "inbox")

		// Heavy pending backlog: a committed-blind top-N of width 2 would take
		// only heavy-a/heavy-b and drop the low-weight committed id entirely.
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

		// Candidate branch adds feature.txt; then the same change is landed on
		// main under a different sha (cherry-pick) ⇒ patch-id supersession.
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

		// Negative: a genuine conflict must NOT be reported as already landed.
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

// --- 003: the LIVE state actually lost the carryover entry ------------------

// TestC1181_003_CarryoverEntryRetiredFromLiveState pins AC-2, read independently
// of the doc.
//
// This is the exact gap that sank cycle 1164 (D1 HIGH): the resolution existed
// as prose while the live .evolve/state.json still carried the entry, so the
// next triage re-picked it. The predicate therefore parses the LIVE state file
// under the STATE root and asserts the id is gone.
//
// Two anti-gaming guards, because "make the id absent" has a trivially wrong
// implementation:
//
//	(a) the rest of the backlog must survive — truncating or wiping
//	    carryoverTodos would green a naive absence check while destroying 60
//	    other tracked items;
//	(b) statemapRevision must have strictly advanced past the pre-cycle baseline,
//	    which the sanctioned locked read-modify-write
//	    (`evolve carryover apply-decisions --apply`) bumps.
//	    stateRevision is no longer pinned to equality because it bumps unconditionally
//	    at cycle end, so it must be >= the baseline.
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

// --- fixture helpers --------------------------------------------------------

// writeInboxItem drops an inbox item JSON carrying id and weight into dir,
// mirroring .evolve/inbox/ naming.
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

// menuIDs flattens lane menus to a flat id set for membership assertions.
func menuIDs(menus [][]triagecap.FleetCandidate) map[string]bool {
	out := map[string]bool{}
	for _, menu := range menus {
		for _, c := range menu {
			out[c.ID] = true
		}
	}
	return out
}

// newGitRepo initialises an isolated repo on branch `main` with one commit, so
// the fleet-rebase classifier has real refs (and a real merge-base) to work
// over. Identity is set locally: the suite must not depend on operator git config.
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

// writeAndCommit writes name with content and commits it.
func writeAndCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	gitRun(t, dir, "add", name)
	gitRun(t, dir, "commit", "-q", "-m", msg)
}

// gitRun runs git in dir and fails the test on a non-zero exit.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gitOut runs git in dir and returns trimmed stdout.
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
