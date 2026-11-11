package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func seedShip(t *testing.T, evolveDir string, cycle int) {
	t.Helper()
	row := fmt.Sprintf(`{"ts":%q,"cycle":%d,"role":"ship","kind":"phase","exit_code":0}`+"\n", time.Now().UTC().Format(time.RFC3339), cycle)
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(row), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPausedLaneScopes_ListsEachQuotaPausedScopeSortedByCycle(t *testing.T) {
	t.Parallel()
	evolveDir, worktree := t.TempDir(), t.TempDir()
	pathB := seedLaneRun(t, evolveDir, quotaPausedRun(12, worktree, "item-b"))
	pathA := seedLaneRun(t, evolveDir, quotaPausedRun(7, worktree, "item-a"))
	pathC := seedLaneRun(t, evolveDir, quotaPausedRun(9, worktree, "item-c", "item-d"))

	got, err := PausedLaneScopes(t.TempDir(), evolveDir, ResumeOptions{})
	if err != nil {
		t.Fatal(err)
	}

	want := []PausedScope{
		{Cycle: 7, Scope: []string{"item-a"}, Phase: "build", StatePath: pathA, AutoMaxAttempts: 3},
		{Cycle: 9, Scope: []string{"item-c", "item-d"}, Phase: "build", StatePath: pathC, AutoMaxAttempts: 3},
		{Cycle: 12, Scope: []string{"item-b"}, Phase: "build", StatePath: pathB, AutoMaxAttempts: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PausedLaneScopes = %+v, want %+v", got, want)
	}
}

func TestPausedLaneScopes_KeepsOnlyTheNewestAttemptOfEachScope(t *testing.T) {
	t.Parallel()
	evolveDir, worktree := t.TempDir(), t.TempDir()
	seedLaneRun(t, evolveDir, quotaPausedRun(5, worktree, "item-a"))
	path := seedLaneRun(t, evolveDir, quotaPausedRun(6, worktree, "item-a"))
	seedLaneRun(t, evolveDir, quotaPausedRun(8, worktree, "item-b"))
	seedLaneRun(t, evolveDir, laneRun{cycle: 9, scope: []string{"item-b"}, phase: "scout"})

	got, err := PausedLaneScopes(t.TempDir(), evolveDir, ResumeOptions{})
	if err != nil {
		t.Fatal(err)
	}

	want := []PausedScope{{Cycle: 6, Scope: []string{"item-a"}, Phase: "build", StatePath: path, AutoMaxAttempts: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PausedLaneScopes = %+v, want %+v", got, want)
	}
}

func TestPausedLaneScopes_SkipsEveryCycleThatALaneCannotResume(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()
	tests := []struct {
		name string
		run  laneRun
		ship bool
	}{
		{"a leased cycle", laneRun{cycle: 5, scope: []string{"x"}, phase: "build", reason: laneQuotaReason, enabled: true, worktree: worktree, leased: true}, false},
		{"an ended cycle", laneRun{cycle: 5, scope: []string{"x"}, phase: "end", reason: laneQuotaReason, enabled: true, worktree: worktree}, false},
		{"an aborted cycle", laneRun{cycle: 5, scope: []string{"x"}, phase: "aborted", reason: laneQuotaReason, enabled: true, worktree: worktree}, false},
		{"a stale cycle", laneRun{cycle: 5, scope: []string{"x"}, phase: "build", reason: laneQuotaReason, enabled: true, worktree: worktree, age: 24 * 365 * time.Hour}, false},
		{"a non-quota cycle", laneRun{cycle: 5, scope: []string{"x"}, phase: "build", reason: "batch-cap-near", enabled: true, worktree: worktree}, false},
		{"a shipped cycle", quotaPausedRun(5, worktree, "x"), true},
		{"a removed worktree", quotaPausedRun(5, filepath.Join(worktree, "gone"), "x"), false},
		{"a cycle without a lane scope", quotaPausedRun(5, worktree), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			evolveDir := t.TempDir()
			seedLaneRun(t, evolveDir, tc.run)
			if tc.ship {
				seedShip(t, evolveDir, tc.run.cycle)
			}

			if got, err := PausedLaneScopes(t.TempDir(), evolveDir, ResumeOptions{}); err != nil || len(got) != 0 {
				t.Fatalf("PausedLaneScopes = %+v, %v, want none", got, err)
			}
		})
	}
}

func TestPausedLaneScopes_NoRunsDirectoryIsNothingPaused(t *testing.T) {
	t.Parallel()
	if got, err := PausedLaneScopes(t.TempDir(), t.TempDir(), ResumeOptions{}); err != nil || got != nil {
		t.Fatalf("PausedLaneScopes = %+v, %v, want nil", got, err)
	}
}

func TestPausedLaneScopes_AnUnreadableRunsDirectoryIsAnError(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, "runs"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := PausedLaneScopes(t.TempDir(), evolveDir, ResumeOptions{}); err == nil {
		t.Fatalf("PausedLaneScopes = %+v, want an error for an unreadable runs directory", got)
	}
}

func TestPausedLaneScopes_ASiblingLaneThatStartedLaterDoesNotMakeAPausedLaneStale(t *testing.T) {
	t.Parallel()
	evolveDir, worktree := t.TempDir(), t.TempDir()
	path := seedLaneRun(t, evolveDir, quotaPausedRun(7, worktree, "item-a"))
	seedLaneRun(t, evolveDir, laneRun{cycle: 8, scope: []string{"item-b"}, phase: "end"})
	seedShip(t, evolveDir, 8)

	got, err := PausedLaneScopes(t.TempDir(), evolveDir, ResumeOptions{})

	if err != nil || len(got) != 1 || got[0].StatePath != path {
		t.Fatalf("PausedLaneScopes = %+v, %v, want cycle 7: a wave starts and ships sibling lanes after a pause, and that is no reason to drop it", got, err)
	}
}

func TestPausedLaneScopes_LogsACheckpointThatCannotBeLoaded(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	seedLaneRun(t, evolveDir, quotaPausedRun(5, filepath.Join(t.TempDir(), "gone"), "x"))
	var log strings.Builder

	got, err := PausedLaneScopes(t.TempDir(), evolveDir, ResumeOptions{Log: &log})

	if err != nil || len(got) != 0 || !strings.Contains(log.String(), "cycle 5 checkpoint is not resumable") {
		t.Fatalf("got %+v, %v, log %q, want the refused checkpoint logged", got, err, log.String())
	}
}

func TestPausedLaneScopes_AnUnreadableLedgerIsLoggedAndTheScanGoesOn(t *testing.T) {
	t.Parallel()
	evolveDir, worktree := t.TempDir(), t.TempDir()
	seedLaneRun(t, evolveDir, quotaPausedRun(7, worktree, "item-a"))
	if err := os.MkdirAll(filepath.Join(evolveDir, "ledger.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	var log strings.Builder

	got, err := PausedLaneScopes(t.TempDir(), evolveDir, ResumeOptions{Log: &log})

	if err != nil || len(got) != 1 || !strings.Contains(log.String(), "ledger scan stopped early") {
		t.Fatalf("got %+v, %v, log %q, want the paused cycle and the ledger warning", got, err, log.String())
	}
}

func TestLoadResumeStateFrom_AStateThatCannotBeReadOrParsedIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := loadResumeStateFrom(t.TempDir(), t.TempDir(), ResumeOptions{}.withDefaults()); err == nil || !strings.Contains(err.Error(), "read state") {
		t.Fatalf("err = %v, want a read-state error for a directory", err)
	}
	malformed := filepath.Join(t.TempDir(), "cycle-state.json")
	if err := os.WriteFile(malformed, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadResumeStateFrom(malformed, t.TempDir(), ResumeOptions{}.withDefaults()); err == nil || !strings.Contains(err.Error(), "parse state") {
		t.Fatalf("err = %v, want a parse-state error", err)
	}
}

func TestPausedLaneScopes_CarriesTheSpentAttemptsAndTheResetTimeOfTheCheckpoint(t *testing.T) {
	t.Parallel()
	evolveDir, worktree := t.TempDir(), t.TempDir()
	run := quotaPausedRun(7, worktree, "item-a")
	run.attempts = 2
	path := seedLaneRun(t, evolveDir, run)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	state["checkpoint"].(map[string]any)["quotaResetAt"] = "2999-01-01T00:00:00Z"
	raw, _ = json.Marshal(state)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := PausedLaneScopes(t.TempDir(), evolveDir, ResumeOptions{})

	if err != nil || len(got) != 1 || got[0].AutoAttempts != 2 || got[0].AutoMaxAttempts != 3 || got[0].QuotaResetAt != "2999-01-01T00:00:00Z" {
		t.Fatalf("PausedLaneScopes = %+v, %v, want attempts 2 of 3 and the reset time, which the pool reads to keep a spent or walled scope in its triage place", got, err)
	}
}

func TestFindLaneResume_RefusesACycleThatShippedAndPausedAgain(t *testing.T) {
	t.Parallel()
	evolveDir, worktree := t.TempDir(), t.TempDir()
	seedLaneRun(t, evolveDir, quotaPausedRun(5, worktree, "x"))
	if _, ok := FindLaneResume(t.TempDir(), evolveDir, []string{"x"}, ResumeOptions{}); !ok {
		t.Fatal("precondition: the quota-paused cycle is adoptable before it ships")
	}
	seedShip(t, evolveDir, 5)

	if rp, ok := FindLaneResume(t.TempDir(), evolveDir, []string{"x"}, ResumeOptions{}); ok {
		t.Fatalf("the lane adopted cycle %d after it shipped; the adopter and the grant list must share the shipped rule", rp.CycleID)
	}
}
