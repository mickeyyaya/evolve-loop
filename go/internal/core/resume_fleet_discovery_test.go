package core

// resume_fleet_discovery_test.go — a fleet-written checkpoint must be findable
// by a host-global resume.
//
// THE INCIDENT (2026-08-29 → 08-31, cycles 1580-1582): all three lanes hit the
// all-families quota wall. Each one kept its promise — "checkpoint written —
// resume with `evolve loop --resume` after quota reset" — via
// QuotaBoundaryCheckpointer, which resolves through ResolveCycleStatePath and
// therefore honored the fleet lane's EVOLVE_CYCLE_STATE_FILE override: the
// checkpoint landed in the lane's PER-RUN cycle-state file
// (.evolve/runs/cycle-N/cycle-state.json). The lane teardown then unset the
// override. When the operator ran `evolve loop --resume` after the quota reset,
// the fresh process resolved the HOST-GLOBAL .evolve/cycle-state.json — absent —
// and reported "no live checkpoint" while THREE live quota-likely checkpoints
// sat on disk. Cycle-1580 had completed build and reached audit; all of that
// progress was abandoned and the next wave re-did the work from scratch.
//
// The writer learned fleet isolation (2026-07-03); the reader never did. This
// is also the root cause behind the long-standing operator note "fleet -resume
// broken (relaunch FRESH)" — it was never a checkpointing defect, it is a
// DISCOVERY defect.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// fleetCheckpointState builds a realistic per-run state body in the exact
// shape QuotaBoundaryCheckpointer writes.
func fleetCheckpointState(cycle int, savedAt, phase, worktree string) map[string]any {
	return map[string]any{
		"cycle_id": cycle,
		"phase":    phase,
		"checkpoint": map[string]any{
			"enabled":         true,
			"reason":          "quota-likely",
			"savedAt":         savedAt,
			"resumeFromPhase": phase,
			"worktreePath":    worktree,
			"gitHead":         "unknown", // capture failed — validation skipped, as live
			"completedPhases": []string{"scout", "triage", "tdd", "build"},
		},
	}
}

// TestLoadResumeState_DiscoversFleetPerRunCheckpoint reproduces the incident:
// host-global cycle-state.json ABSENT, three per-run quota-likely checkpoints
// present. Resume must find the NEWEST one (by savedAt) instead of reporting
// "no live checkpoint" over live checkpoints.
func TestLoadResumeState_DiscoversFleetPerRunCheckpoint(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}
	// The three orphans, exactly as the incident left them.
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1580"), fleetCheckpointState(1580, "2026-08-29T02:10:00Z", "audit", wt))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1581"), fleetCheckpointState(1581, "2026-08-29T03:40:00Z", "tdd", wt))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1582"), fleetCheckpointState(1582, "2026-08-29T04:55:00Z", "triage", wt))

	rp, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if err != nil {
		t.Fatalf("resume over three live per-run checkpoints must not fail: %v", err)
	}
	if rp.CycleID != 1582 {
		t.Errorf("CycleID = %d, want 1582 (the newest savedAt)", rp.CycleID)
	}
	if rp.Reason != "quota-likely" {
		t.Errorf("Reason = %q, want quota-likely", rp.Reason)
	}
	// The found path must be DISCLOSED so the caller can route the resumed
	// run's own state writes back to the same per-run file. Without this the
	// resumed cycle would write its state to the host-global singleton and the
	// next quota pause would orphan a checkpoint all over again.
	if !strings.Contains(rp.StatePath, filepath.Join("runs", "cycle-1582")) {
		t.Errorf("StatePath = %q, want the per-run file the checkpoint came from", rp.StatePath)
	}
	if len(rp.CompletedPhases) != 4 {
		t.Errorf("CompletedPhases = %v — the preserved progress is the whole point", rp.CompletedPhases)
	}
}

// The host-global checkpoint must still WIN when it exists: discovery is a
// fallback, not a re-ranking. A sequential (non-fleet) loop writes host-global
// and must resume exactly as before this change.
func TestLoadResumeState_HostGlobalWinsOverPerRun(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}
	writeStateFile(t, evolveDir, fleetCheckpointState(900, "2026-08-29T01:00:00Z", "build", wt))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-901"), fleetCheckpointState(901, "2026-08-29T09:00:00Z", "audit", wt))

	rp, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if err != nil {
		t.Fatalf("LoadResumeState: %v", err)
	}
	if rp.CycleID != 900 {
		t.Errorf("CycleID = %d, want 900 — a live host-global checkpoint must not be outranked by discovery", rp.CycleID)
	}
}

// A fleet lane's own resolution (env override set) must NOT trigger discovery:
// the override IS the authoritative path for that process, and scanning
// siblings from inside a lane would let one lane resume another's cycle.
func TestLoadResumeState_EnvOverrideDisablesDiscovery(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}
	// Sibling lane has a live checkpoint; OUR override points at a file with none.
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-951"), fleetCheckpointState(951, "2026-08-29T09:00:00Z", "audit", wt))
	own := writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-950"), map[string]any{"cycle_id": 950})
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", own)

	_, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if !errors.Is(err, ErrNoCheckpoint) {
		t.Fatalf("err = %v, want ErrNoCheckpoint — an env-overridden lane must never resume a SIBLING's cycle", err)
	}
}

// When per-run checkpoints exist but the newest is STALE (worktree gone), the
// error must be the honest ErrStaleCheckpoint — never "no live checkpoint",
// which is the lie the incident produced. An operator told "stale" knows to
// pass the override or reset; an operator told "nothing to resume" relaunches
// fresh and burns the preserved progress.
func TestLoadResumeState_StalePerRunCheckpointReportsStaleNotMissing(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	gone := filepath.Join(tmp, "deleted-worktree")
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-970"), fleetCheckpointState(970, "2026-08-29T05:00:00Z", "audit", gone))

	_, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if !errors.Is(err, ErrStaleCheckpoint) {
		t.Fatalf("err = %v, want ErrStaleCheckpoint (honest) — a stale checkpoint is not an absent one", err)
	}
}

// Nothing anywhere: the original error stands, and it now names where it
// looked so the next operator does not have to rediscover the two locations.
func TestLoadResumeState_NoCheckpointsAnywhereStillErrNoCheckpoint(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "runs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if !errors.Is(err, ErrNoCheckpoint) {
		t.Fatalf("err = %v, want ErrNoCheckpoint", err)
	}
}

// --- write-back routing ---

// A checkpoint discovered in a per-run file must make the RESUMED run write
// its state back to that same file. Without this, the resumed cycle writes to
// the host-global singleton: its next quota pause checkpoints THERE, the run
// dir and singleton disagree, and the orphaning starts over.
func TestActivateResumeStatePath_RoutesResolutionToTheDiscoveredFile(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	perRun := filepath.Join(evolveDir, "runs", "cycle-1582", CycleStateFile)
	if err := os.MkdirAll(filepath.Dir(perRun), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Ensure a clean slate for the env-mutating helper.
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", "")
	if err := os.Unsetenv("EVOLVE_CYCLE_STATE_FILE"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}

	restore := ActivateResumeStatePath(&ResumePoint{StatePath: perRun}, evolveDir)
	if got := ResolveCycleStatePath(evolveDir); got != perRun {
		t.Errorf("after activation, ResolveCycleStatePath = %q, want the discovered per-run file %q", got, perRun)
	}
	restore()
	if got := ResolveCycleStatePath(evolveDir); got == perRun {
		t.Error("after restore, resolution still points at the per-run file — the override leaked past the resume")
	}
}

// When the checkpoint came from the file the process would resolve ANYWAY
// (host-global resume of a host-global checkpoint, or a fleet lane reading its
// own override), activation must be a no-op — re-setting the same value is
// harmless, but claiming an override that changes nothing muddies debugging.
func TestActivateResumeStatePath_NoOpWhenAlreadyResolved(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", "")
	if err := os.Unsetenv("EVOLVE_CYCLE_STATE_FILE"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	hostGlobal := ResolveCycleStatePath(evolveDir)

	restore := ActivateResumeStatePath(&ResumePoint{StatePath: hostGlobal}, evolveDir)
	if got := os.Getenv("EVOLVE_CYCLE_STATE_FILE"); got != "" {
		t.Errorf("env override set to %q for an already-resolved path — want no-op", got)
	}
	restore()
	// Empty StatePath (older checkpoint shape) must also be a safe no-op.
	restore2 := ActivateResumeStatePath(&ResumePoint{}, evolveDir)
	restore2()
}

// --- adversarial-review round: liveness + reason filtering ---

// CRITICAL (review Q5): PhaseBoundaryCheckpointer writes enabled:true with
// reason "phase-complete" after EVERY phase of a HEALTHY run — a crash
// breadcrumb for the same process, never a cross-process resume target. Its
// gitHead is "" (HEAD validation dead) and a live lane's worktree exists, so
// without a reason filter a `--resume` during an active wave discovers the
// live lane's newest breadcrumb and DOUBLE-DRIVES the running cycle: two
// processes dispatching agents into one worktree. Observed in this session's
// own live probe, which found reason=phase-complete and called it a success.
func TestLoadResumeState_PhaseCompleteBreadcrumbIsNotDiscoverable(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}
	st := fleetCheckpointState(1584, "2026-08-31T09:00:00Z", "audit", wt)
	st["checkpoint"].(map[string]any)["reason"] = "phase-complete"
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1584"), st)

	_, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if !errors.Is(err, ErrNoCheckpoint) {
		t.Fatalf("err = %v, want ErrNoCheckpoint — a phase-complete breadcrumb is not a pause and must never be resumed by another process", err)
	}
}

// CRITICAL (review Q5, second half): even an escalation-reason checkpoint must
// be skipped while its lane is ALIVE — the lease heartbeat is the liveness
// signal gc already trusts, and discovery reuses it rather than inventing one.
func TestLoadResumeState_FreshLeaseExcludesCandidate(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}
	runDir := filepath.Join(evolveDir, "runs", "cycle-1590")
	writeStateFile(t, runDir, fleetCheckpointState(1590, "2026-08-31T09:00:00Z", "audit", wt))
	if err := runlease.Write(runDir, runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatalf("write lease: %v", err)
	}

	_, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if !errors.Is(err, ErrNoCheckpoint) {
		t.Fatalf("err = %v, want ErrNoCheckpoint — a FRESH lease means the lane is alive; resuming it would double-drive the cycle", err)
	}
}

// The converse must hold or quota recovery breaks all over again: a quota-paused
// lane's process EXITS (rc=5), its heartbeat goes stale, and that checkpoint is
// exactly the one discovery exists to find. A stale lease must not exclude.
func TestLoadResumeState_StaleLeaseDoesNotExclude(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}
	runDir := filepath.Join(evolveDir, "runs", "cycle-1591")
	writeStateFile(t, runDir, fleetCheckpointState(1591, "2026-08-31T09:00:00Z", "audit", wt))
	if err := runlease.Write(runDir, runlease.Lease{OwnerPID: 1}, time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatalf("write lease: %v", err)
	}

	rp, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if err != nil {
		t.Fatalf("a dead lane's quota checkpoint must be discoverable: %v", err)
	}
	if rp.CycleID != 1591 {
		t.Errorf("CycleID = %d, want 1591", rp.CycleID)
	}
}

// MEDIUM (review Q1) + simplifier: when the newest candidate is stale and an
// older one validates, the older wins (independent lanes, no supersession) —
// but the skip must be SAID, or the operator never learns the newest is
// sitting orphaned until a later resume trips over it.
func TestLoadResumeState_NewestStaleFallsBackToOlderValidAndSaysSo(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}
	gone := filepath.Join(tmp, "deleted-worktree")
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1592"), fleetCheckpointState(1592, "2026-08-31T02:00:00Z", "build", wt))
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1593"), fleetCheckpointState(1593, "2026-08-31T03:00:00Z", "audit", gone))

	var log strings.Builder
	rp, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{Log: &log})
	if err != nil {
		t.Fatalf("older valid candidate must win when newest is stale: %v", err)
	}
	if rp.CycleID != 1592 {
		t.Errorf("CycleID = %d, want 1592 (older but valid)", rp.CycleID)
	}
	if !strings.Contains(log.String(), "cycle-1593") {
		t.Errorf("skip breadcrumb missing — log %q must name the stale newest so the operator knows it needs attention", log.String())
	}
}
