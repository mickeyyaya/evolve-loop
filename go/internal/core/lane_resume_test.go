package core

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type laneRun struct {
	cycle    int
	scope    []string
	phase    string
	reason   string
	enabled  bool
	attempts int
	worktree string
	leased   bool
	age      time.Duration
}

func seedLaneRun(t *testing.T, evolveDir string, r laneRun) string {
	t.Helper()
	dir := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(r.cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if r.scope != nil {
		scope, _ := json.Marshal(LaneScope{TodoIDs: r.scope, GoalHash: "wave-goal"})
		if err := os.WriteFile(filepath.Join(dir, LaneScopeFile), scope, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	state := map[string]any{"cycle_id": r.cycle, "phase": r.phase}
	if r.reason != "" {
		state["checkpoint"] = map[string]any{
			"enabled": r.enabled, "reason": r.reason, "resumeFromPhase": r.phase, "worktreePath": r.worktree,
			"savedAt": time.Now().Add(-r.age).UTC().Format(time.RFC3339), "autoResumeAttempts": r.attempts, "autoResumeMaxAttempts": 3,
		}
	}
	if r.leased {
		if err := runlease.Write(dir, runlease.Lease{RunID: "live-run", OwnerPID: os.Getpid()}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(state)
	path := filepath.Join(dir, CycleStateFile)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func quotaPausedRun(cycle int, worktree string, scope ...string) laneRun {
	return laneRun{cycle: cycle, scope: scope, phase: "build", reason: laneQuotaReason, enabled: true, worktree: worktree}
}

func TestFindLaneResume_TheLastAttemptOfTheTodoPausedOnQuotaResumesAtItsPhase(t *testing.T) {
	t.Parallel()
	evolveDir, worktree := t.TempDir(), t.TempDir()
	seedLaneRun(t, evolveDir, laneRun{cycle: 10, scope: []string{"item-a"}, phase: "end"})
	path := seedLaneRun(t, evolveDir, quotaPausedRun(11, worktree, "item-a"))
	seedLaneRun(t, evolveDir, quotaPausedRun(12, worktree, "item-b"))

	rp, ok := FindLaneResume(t.TempDir(), evolveDir, []string{"item-a"}, ResumeOptions{})

	if !ok || rp.CycleID != 11 || rp.Phase != "build" || rp.StatePath != path || rp.WorktreePath != worktree || rp.AutoMaxAttempts != 3 {
		t.Fatalf("FindLaneResume = %+v, %v; want cycle 11 at build from %s", rp, ok, path)
	}
}

func TestFindLaneResume_StartsFreshWhenNoPausedLastAttemptIsResumable(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()
	tests := []struct {
		name  string
		scope string
		runs  []laneRun
		log   string
	}{
		{"an empty lane scope", " , ", []laneRun{quotaPausedRun(5, worktree)}, ""},
		{"a checkpoint older than the age bound", "item-a", []laneRun{{cycle: 5, scope: []string{"item-a"}, phase: "build", reason: laneQuotaReason, enabled: true, worktree: worktree, age: 24 * 365 * time.Hour}}, "cycle 5 checkpoint is not resumable"},
		{"no run of the todo", "item-a", []laneRun{quotaPausedRun(5, worktree, "item-b")}, ""},
		{"a fresh attempt after the pause", "item-a", []laneRun{quotaPausedRun(5, worktree, "item-a"), {cycle: 6, scope: []string{"item-a"}, phase: "scout"}}, ""},
		{"a pause for another reason", "item-a", []laneRun{{cycle: 5, scope: []string{"item-a"}, phase: "build", reason: "operator-requested", enabled: true, worktree: worktree}}, ""},
		{"a disabled checkpoint", "item-a", []laneRun{{cycle: 5, scope: []string{"item-a"}, phase: "build", reason: laneQuotaReason, worktree: worktree}}, ""},
		{"a completed cycle", "item-a", []laneRun{{cycle: 5, scope: []string{"item-a"}, phase: "end", reason: laneQuotaReason, enabled: true, worktree: worktree}}, ""},
		{"an aborted cycle", "item-a", []laneRun{{cycle: 5, scope: []string{"item-a"}, phase: "aborted", reason: laneQuotaReason, enabled: true, worktree: worktree}}, ""},
		{"a removed worktree", "item-a", []laneRun{quotaPausedRun(5, filepath.Join(worktree, "gone"), "item-a")}, "cycle 5 checkpoint is not resumable"},
		{"a cycle that a live run holds", "item-a", []laneRun{{cycle: 5, scope: []string{"item-a"}, phase: "build", reason: laneQuotaReason, enabled: true, worktree: worktree, leased: true}}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			evolveDir := t.TempDir()
			for _, r := range tc.runs {
				seedLaneRun(t, evolveDir, r)
			}
			var log bytes.Buffer

			rp, ok := FindLaneResume(t.TempDir(), evolveDir, ScopeIDs(tc.scope), ResumeOptions{Log: &log})

			if ok || rp != nil {
				t.Fatalf("FindLaneResume = %+v, %v; want a fresh cycle", rp, ok)
			}
			if !strings.Contains(log.String(), tc.log) {
				t.Errorf("log = %q, want it to name %q", log.String(), tc.log)
			}
		})
	}
}

func TestFindLaneResume_AMissingRunsDirectoryStartsFresh(t *testing.T) {
	t.Parallel()
	if rp, ok := FindLaneResume(t.TempDir(), filepath.Join(t.TempDir(), "absent"), []string{"item-a"}, ResumeOptions{}); ok || rp != nil {
		t.Fatalf("FindLaneResume = %+v, %v; want a fresh cycle", rp, ok)
	}
}

func TestFindLaneResume_MatchesAMultiTodoScopeInAnyOrder(t *testing.T) {
	t.Parallel()
	evolveDir, worktree := t.TempDir(), t.TempDir()
	seedLaneRun(t, evolveDir, quotaPausedRun(8, worktree, "item-b", "item-a"))

	rp, ok := FindLaneResume(t.TempDir(), evolveDir, ScopeIDs("item-a, item-b,item-a"), ResumeOptions{})

	if !ok || rp.CycleID != 8 {
		t.Fatalf("FindLaneResume = %+v, %v; want cycle 8", rp, ok)
	}
}

func TestScopeIDs_TrimsSortsAndDropsEmptyAndRepeatedIDs(t *testing.T) {
	t.Parallel()
	got := ScopeIDs(" b,a,, b ,  ")
	if want := []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("normalizedIDs = %v, want %v", got, want)
	}
}

func TestEmitLaneResumed_PublishesAnInfoEventWithTheResumeFacts(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()

	EmitLaneResumed(c, &ResumePoint{CycleID: 2107, Phase: "triage", AutoAttempts: 1, AutoMaxAttempts: 3}, []string{"outcome-churn-revert-recovery"})

	if len(*got) != 1 {
		t.Fatalf("events = %d, want 1", len(*got))
	}
	e := (*got)[0]
	want := map[string]string{"scope": "outcome-churn-revert-recovery", "phase": "triage", "attempts": "1", "max_attempts": "3"}
	if e.Code != CodeLaneResumed || e.Severity != signalcenter.SeverityInfo || e.Kind != signalcenter.KindLoopWave || e.Cycle != 2107 || e.Phase != "triage" || !reflect.DeepEqual(e.Fields, want) {
		t.Errorf("event = %+v, want an INFO %s for cycle 2107 with fields %v", e, CodeLaneResumed, want)
	}
	if m, ok := signalcenter.IsRegistered(CodeLaneResumed); !ok || m != signalcenter.ModuleOrchestrator {
		t.Errorf("IsRegistered(%s) = %q, %v; want the orchestrator module", CodeLaneResumed, m, ok)
	}
}
