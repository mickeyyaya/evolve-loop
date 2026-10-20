package core

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sealedRunState(cycle int, startedAt string) map[string]any {
	return map[string]any{"cycle_id": cycle, "phase": string(PhaseEnd), "started_at": startedAt, "goal_hash": "later-launch"}
}

func stoppedLaneState(cycle int, startedAt string) map[string]any {
	return map[string]any{"cycle_id": cycle, "phase": "build", "started_at": startedAt}
}

func wave27Fixture(t *testing.T) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	wt := filepath.Join(root, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1709"), fleetCheckpointState(1709, "2026-09-27T10:00:00Z", "build", wt))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1738"), sealedRunState(1738, "2026-09-28T09:00:00Z"))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1739"), stoppedLaneState(1739, "2026-09-28T20:00:00Z"))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1740"), stoppedLaneState(1740, "2026-09-28T20:00:01Z"))
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
		"cycle 1709: stale: cycle 1738 started at 2026-09-28T09:00:00Z, after this checkpoint (2026-09-27T10:00:00Z), and sealed",
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
	refill := sealedRunState(1803, "2026-10-01T10:45:00Z")
	refill["goal_hash"] = "this-launch"
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1801"), paused)
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1802"), sealedRunState(1802, "2026-10-01T10:00:00Z"))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1800"), sealedRunState(1800, "2026-10-01T11:00:00Z"))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1803"), refill)

	rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{})

	if err != nil || rp.CycleID != 1801 {
		t.Fatalf("rp=%+v err=%v — a sibling that started before the pause, a lower cycle, or a pool refill of the same launch does not make the checkpoint stale", rp, err)
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

	if err == nil || !strings.Contains(err.Error(), `cycle 1900: checkpoint reason "signal" is not resumable`) {
		t.Fatalf("err = %v, want the non-resumable reason named", err)
	}
}

func TestLoadResumeState_OnlySealedRunsLeaveNothingToConsider(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-2000"), sealedRunState(2000, "2026-10-03T10:00:00Z"))

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
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-2101"), sealedRunState(2101, "2026-10-04T10:00:00Z"))

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
