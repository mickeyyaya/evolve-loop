package loopwave

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

const cycle1838Item = "rollback-fail-open-and-vacuous-tests"

func pausedHolder(t *testing.T, h *harness, cycle int, goal string) {
	t.Helper()
	runDir := h.ports.Workspace(cycle)
	worktree := filepath.Join(h.evolveDir, "worktrees", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(runDir, "cycle-state.json"), map[string]any{
		"cycle_id": cycle, "phase": "tdd", "goal_hash": goal,
		"checkpoint": map[string]any{"enabled": true, "reason": "quota-likely", "resumeFromPhase": "tdd", "worktreePath": worktree},
	})
	if err := runlease.Write(runDir, runlease.Lease{RunID: "r", OwnerPID: os.Getpid()}, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func startedCycle(t *testing.T, h *harness, cycle int, goal string) {
	t.Helper()
	writeJSON(t, filepath.Join(h.ports.Workspace(cycle), "cycle-state.json"), map[string]any{"cycle_id": cycle, "phase": "triage", "goal_hash": goal})
}

func claimAndRestore(t *testing.T, h *harness, cycle int, id string) string {
	t.Helper()
	item := map[string]any{"id": id, "weight": 0.9, "files": []string{"pkg/" + id + ".go"}}
	name := "2026-09-30T11-07-44Z-" + id + ".json"
	claim := filepath.Join(h.evolveDir, "inbox", "processing", fmt.Sprintf("cycle-%d", cycle), name)
	writeJSON(t, claim, item)
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", name), item)
	return claim
}

func TestPlanFn_Cycle1838Regression_ARestoredCopyOfAPausedClaimIsNeverPlannedThenReleased(t *testing.T) {
	h := newHarness(t)
	h.ports.LastCycle = func(context.Context) (int, error) { return 1835, nil }
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)
	pausedHolder(t, h, 1836, "goal-wave-80")
	claim := claimAndRestore(t, h, 1836, cycle1838Item)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "beta")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "gamma")
	writeJSON(t, filepath.Join(h.ports.Workspace(1835), triagecap.TriageDecisionName()),
		map[string]any{"top_n": []map[string]any{{"id": cycle1838Item, "files": []string{"pkg/x.go"}}, {"id": "beta", "files": []string{"pkg/beta.go"}}}})

	wave81, _, err := e.PlanFn(2)(context.Background(), 81)
	if err != nil {
		t.Fatal(err)
	}
	if topNIDs(t, wave81)[cycle1838Item] {
		t.Fatalf("wave 81 planned the item that cycle 1836 still holds: %s", wave81)
	}
	if _, err := os.Stat(claim); err != nil {
		t.Fatalf("a paused claim with no newer goal must stay (resume pending): %v", err)
	}

	startedCycle(t, h, 1837, "goal-wave-81")
	h.stderr.Reset()
	wave82, _, err := e.PlanFn(2)(context.Background(), 82)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claim); !os.IsNotExist(err) {
		t.Fatalf("once a newer goal runs, the stale claim must be released: %v", err)
	}
	if line := `wave plan: released the claim of "` + cycle1838Item + `" held by cycle-1836 (duplicate-removed)`; !strings.Contains(h.stderr.String(), line) {
		t.Errorf("stderr must log one line per release, want %q in:\n%s", line, h.stderr.String())
	}
	if !topNIDs(t, wave82)[cycle1838Item] {
		t.Errorf("the released item is dispatchable again and the widen plans it: %s", wave82)
	}

	opts := inboxmover.Options{ProjectRoot: h.root}
	if err := inboxmover.ClaimPending(opts, 1839, []string{cycle1838Item}); err != nil {
		t.Fatalf("the next lane's triage claim: %v", err)
	}
	if loc, err := inboxmover.Locate(filepath.Join(h.evolveDir, "inbox"), cycle1838Item); err != nil || loc.Cycle != 1839 {
		t.Fatalf("the item must be held once, by cycle 1839: %+v %v", loc, err)
	}
	if survey, err := inboxmover.SurveyClaims(opts); err != nil || len(survey.Claims) != 1 || survey.Claims[0].Duplicate {
		t.Errorf("exactly one claim and no root duplicate after the triage claim: %+v %v", survey, err)
	}
}

func TestPlanFn_NeverReleasesALiveClaim(t *testing.T) {
	h := newHarness(t)
	runDir := h.ports.Workspace(1837)
	writeJSON(t, filepath.Join(runDir, "cycle-state.json"), map[string]any{"cycle_id": 1837, "phase": "build", "goal_hash": "g"})
	if err := runlease.Write(runDir, runlease.Lease{RunID: "r", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	startedCycle(t, h, 1838, "newer-goal")
	claim := filepath.Join(h.evolveDir, "inbox", "processing", "cycle-1837", "live.json")
	writeJSON(t, claim, map[string]any{"id": "live", "weight": 0.5})
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "a")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "b")
	if _, _, err := h.e.PlanFn(2)(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claim); err != nil {
		t.Errorf("a live lane keeps its claim: %v", err)
	}
	if strings.Contains(h.stderr.String(), "released the claim") {
		t.Errorf("no release line for a live claim: %s", h.stderr.String())
	}
}

func TestPlanFn_AReleaseFaultIsOneWarnAndThePlanGoesOn(t *testing.T) {
	h := newHarness(t)
	claim := filepath.Join(h.evolveDir, "inbox", "processing", "cycle-1828", "a.json")
	writeJSON(t, claim, map[string]any{"id": "a", "weight": 0.5})
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "a.json"), map[string]any{"id": "other", "weight": 0.5, "files": []string{"pkg/other.go"}})
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "b")

	if _, _, err := h.e.PlanFn(2)(context.Background(), 1); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(h.stderr.String(), `[loop] WARN: wave plan: a stale claim stays held: release a from cycle 1828:`) {
		t.Errorf("stderr = %q, want one WARN that names the claim", h.stderr.String())
	}
	if _, err := os.Stat(claim); err != nil {
		t.Errorf("a refused release leaves the claim: %v", err)
	}
}

func TestPlanFn_TheRunningGoalReleasesAPausedClaimAtTheFirstPlanning(t *testing.T) {
	h := newHarness(t)
	pausedHolder(t, h, 1836, "goal-wave-80")
	claim := claimAndRestore(t, h, 1836, cycle1838Item)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "beta")
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr, WithGoal("goal-wave-81"))

	wave81, _, err := e.PlanFn(2)(context.Background(), 81)

	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claim); !os.IsNotExist(err) {
		t.Fatalf("the first planning of a newer goal must release the paused claim: %v", err)
	}
	if !strings.Contains(h.stderr.String(), `released the claim of "`+cycle1838Item+`" held by cycle-1836 (duplicate-removed): paused (quota-likely), but the running loop has a newer goal`) {
		t.Errorf("stderr = %q", h.stderr.String())
	}
	if !topNIDs(t, wave81)[cycle1838Item] {
		t.Errorf("wave 81 plans the released item: %s", wave81)
	}
}

func TestPlanFn_TheRunningGoalKeepsAPauseOfTheSameGoal(t *testing.T) {
	h := newHarness(t)
	pausedHolder(t, h, 1836, "goal-wave-81")
	claim := claimAndRestore(t, h, 1836, cycle1838Item)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "beta")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "gamma")
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr, WithGoal("goal-wave-81"))

	if _, _, err := e.PlanFn(2)(context.Background(), 81); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claim); err != nil {
		t.Errorf("a pause of the running goal keeps its claim for the resume: %v", err)
	}
}
