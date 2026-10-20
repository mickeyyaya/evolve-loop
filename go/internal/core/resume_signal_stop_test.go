package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func shippedRunState(cycle int, goal string) map[string]any {
	return map[string]any{"cycle_id": cycle, "phase": "ship", "final_verdict": "PASS", "goal_hash": goal, "started_at": "2026-09-28T08:00:00Z"}
}

func stoppedLaneState(cycle int, startedAt string) map[string]any {
	return map[string]any{"cycle_id": cycle, "phase": "build", "final_verdict": "PASS", "goal_hash": "wave-27", "started_at": startedAt}
}

func writeShipRows(t *testing.T, evolveDir string, rows map[int]string) {
	t.Helper()
	var b strings.Builder
	for cycle, ts := range rows {
		fmt.Fprintf(&b, `{"ts":%q,"cycle":%d,"role":"ship","kind":"phase","exit_code":0}`+"\n", ts, cycle)
		fmt.Fprintf(&b, `{"ts":%q,"cycle":%d,"role":"ship","kind":"agent_subprocess","exit_code":0}`+"\n", ts, cycle)
	}
	b.WriteString("{not json\n")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wave27Fixture(t *testing.T) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	wt := filepath.Join(root, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	old := fleetCheckpointState(1709, "2026-09-27T10:00:00Z", "build", wt)
	old["goal_hash"] = "wave-20"
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1709"), old)
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1738"), shippedRunState(1738, "wave-26"))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1739"), stoppedLaneState(1739, "2026-09-28T20:00:00Z"))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1740"), stoppedLaneState(1740, "2026-09-28T20:00:01Z"))
	writeShipRows(t, evolveDir, map[int]string{1738: "2026-09-28T09:30:00Z"})
	return root, evolveDir
}

func TestLoadResumeState_RefusesACheckpointBehindALaterSealedCycleAndNamesTheStoppedLanes(t *testing.T) {
	t.Parallel()
	root, evolveDir := wave27Fixture(t)
	var log bytes.Buffer

	rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{Log: &log})

	if err == nil {
		t.Fatalf("resume picked cycle %d; a checkpoint behind a later sealed cycle must be refused", rp.CycleID)
	}
	if !errors.Is(err, ErrNoCheckpoint) {
		t.Errorf("err = %v, want ErrNoCheckpoint", err)
	}
	for _, want := range []string{
		"cycle 1709: stale: cycle 1738 of another launch shipped at 2026-09-28T09:30:00Z, after this checkpoint (2026-09-27T10:00:00Z)",
		"EVOLVE_CYCLE_STATE_FILE=" + filepath.Join(evolveDir, "runs", "cycle-1709", CycleStateFile) + " evolve cycle reset",
		"cycle 1739: no resumable checkpoint (the cycle stopped without one)",
		"cycle 1740: no resumable checkpoint (the cycle stopped without one)",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q:\n%v", want, err)
		}
	}
	if !strings.Contains(log.String(), "[resume] considered") || !strings.Contains(log.String(), "cycle 1740") {
		t.Errorf("discovery must list every checkpoint it considered, got %q", log.String())
	}
}

func TestLoadResumeState_ASiblingThatSealedBeforeTheCheckpointLeavesItResumable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	wt := filepath.Join(root, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	paused := fleetCheckpointState(1801, "2026-10-01T10:30:00Z", "build", wt)
	paused["goal_hash"] = "this-launch"
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1801"), paused)
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1802"), shippedRunState(1802, "other-launch"))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1800"), shippedRunState(1800, "other-launch"))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1803"), shippedRunState(1803, "this-launch"))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1804"), map[string]any{"cycle_id": 1804, "phase": "aborted", "final_verdict": "FAIL", "goal_hash": "other-launch"})
	writeShipRows(t, evolveDir, map[int]string{1802: "2026-10-01T10:00:00Z", 1800: "2026-10-01T11:00:00Z", 1803: "2026-10-01T10:45:00Z"})

	rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{})

	if err != nil || rp.CycleID != 1801 {
		t.Fatalf("rp=%+v err=%v — a ship before the pause, a lower cycle, a pool refill of the same launch, or a later launch that did not ship does not make the checkpoint stale", rp, err)
	}
}

func TestLoadResumeState_ANonResumableReasonIsNamed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	st := fleetCheckpointState(1900, "2026-10-02T10:00:00Z", "build", "")
	st["checkpoint"].(map[string]any)["reason"] = "signal"
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1900"), st)

	_, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{})

	if err == nil || !strings.Contains(err.Error(), "1 other cycle(s) hold a checkpoint that is not resumable (signal 1)") {
		t.Fatalf("err = %v, want the non-resumable reason counted", err)
	}
}

func TestLoadResumeState_OnlySealedRunsLeaveNothingToConsider(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-2000"), shippedRunState(2000, "g"))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-2001"), map[string]any{"cycle_id": 2001, "phase": "aborted"})
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-2002"), map[string]any{"cycle_id": 2002, "phase": string(PhaseEnd)})
	writeShipRows(t, evolveDir, map[int]string{2000: "2026-10-03T10:00:00Z"})

	_, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{})

	if err == nil || !strings.Contains(err.Error(), "none live)") || strings.Contains(err.Error(), "considered") {
		t.Fatalf("err = %v, want a plain none-live refusal with nothing considered", err)
	}
}

func TestLoadResumeState_ACheckpointWithoutASaveTimeIsNotJudgedStale(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-2100"), fleetCheckpointState(2100, "", "build", ""))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-2101"), shippedRunState(2101, "later-launch"))
	writeShipRows(t, evolveDir, map[int]string{2101: "2026-10-04T10:00:00Z"})

	rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{})

	if err != nil || rp.CycleID != 2100 {
		t.Fatalf("rp=%+v err=%v — without a save time the stale rule cannot order the checkpoint, so it stays resumable", rp, err)
	}
}

func TestNoPerRunCandidates_TheErrorNamesEachRefusedCycle(t *testing.T) {
	t.Parallel()
	err := noPerRunCandidates{refused: []string{"cycle 1: stale", "cycle 2: no resumable checkpoint"}}

	if !errors.Is(err, errNoPerRunCandidates) || err.Error() != "resume: no per-run checkpoint candidates; considered: cycle 1: stale; cycle 2: no resumable checkpoint" {
		t.Fatalf("err = %q", err.Error())
	}
}

func TestLoadResumeState_AResetArchiveIsNotConsideredAgain(t *testing.T) {
	t.Parallel()
	root, evolveDir := wave27Fixture(t)
	if err := os.Rename(filepath.Join(evolveDir, "runs", "cycle-1709"), filepath.Join(evolveDir, "runs", "cycle-1709"+resetArchiveMarker+"20260929T000000")); err != nil {
		t.Fatal(err)
	}

	_, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{})

	if err == nil || strings.Contains(err.Error(), "cycle 1709") {
		t.Fatalf("err = %v — after `evolve cycle reset` archives the run dir, discovery must not judge the archive again", err)
	}
}

func TestLoadResumeState_AnUnparseableSaveTimeIsSaidInTheLog(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-2200"), fleetCheckpointState(2200, "yesterday", "build", ""))
	var log bytes.Buffer

	if _, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{Log: &log}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(log.String(), `the stale rule did not apply, because savedAt "yesterday" is not RFC3339`) {
		t.Fatalf("log = %q", log.String())
	}
}

func TestLoadResumeState_ARefusalNamesTheStoppedLanesAndCountsTheRest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	for c := 3000; c < 3025; c++ {
		writeStateFile(t, filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", c)), stoppedLaneState(c, "2026-10-05T10:00:00Z"))
	}
	for c := 3100; c < 3103; c++ {
		st := fleetCheckpointState(c, "2026-10-05T10:00:00Z", "build", "")
		st["checkpoint"].(map[string]any)["reason"] = "phase-complete"
		writeStateFile(t, filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", c)), st)
	}

	_, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{})

	msg := err.Error()
	if strings.Count(msg, ": no resumable checkpoint") != maxNamedRefusals || !strings.Contains(msg, "5 more refused cycle(s)") || !strings.Contains(msg, "3 other cycle(s) hold a checkpoint that is not resumable (phase-complete 3)") {
		t.Fatalf("err = %v; want %d stopped lanes named, the rest counted, and the plain finished checkpoints summed", err, maxNamedRefusals)
	}
}

func TestLandedShips_ReadsOnlyALandedShipPhaseRow(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	rows := `{"ts":"2026-10-01T00:00:00Z","cycle":1,"role":"ship","kind":"phase","exit_code":0}
{"ts":"2026-10-01T01:00:00Z","cycle":1,"role":"ship","kind":"phase","exit_code":0}
{"ts":"2026-10-01T02:00:00Z","cycle":2,"role":"ship","kind":"phase","exit_code":1}
{"ts":"2026-10-01T03:00:00Z","cycle":3,"role":"audit","kind":"phase","exit_code":0}
{"ts":"yesterday","cycle":4,"role":"ship","kind":"phase","exit_code":0}
`
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}

	ships, err := landedShips(evolveDir)

	if err != nil || len(ships) != 1 || !ships[1].Equal(time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("ships = %v, want cycle 1 at its latest landed ship row only", ships)
	}
	if got, err := landedShips(filepath.Join(evolveDir, "missing")); len(got) != 0 || err != nil {
		t.Fatalf("no ledger gives no ships and no error, got %v %v", got, err)
	}
	if _, err := landedShips(filepath.Join(evolveDir, "ledger.jsonl")); err == nil {
		t.Fatal("a ledger path that is not a directory of the evolve dir must be an error")
	}
}

func TestLoadResumeState_ACheckpointWithoutAGoalHashIsStaleAfterAnyLaterShip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1709"), fleetCheckpointState(1709, "2026-09-27T10:00:00Z", "build", ""))
	later := shippedRunState(1738, "")
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1738"), later)
	writeShipRows(t, evolveDir, map[int]string{1738: "2026-09-28T09:30:00Z"})

	_, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{})

	if err == nil || !strings.Contains(err.Error(), "cycle 1709: stale") {
		t.Fatalf("err = %v; a checkpoint with no goal hash cannot claim a later ship as its own launch", err)
	}
}

func TestLoadResumeState_AnUnreadableLedgerWarnsInTheLog(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1801"), fleetCheckpointState(1801, "2026-10-01T10:30:00Z", "build", ""))
	if err := os.MkdirAll(filepath.Join(evolveDir, "ledger.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer

	if _, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{Log: &log}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(log.String(), "[resume] WARN: the ledger scan stopped early") {
		t.Fatalf("log = %q, want the failed ledger scan said", log.String())
	}
}
