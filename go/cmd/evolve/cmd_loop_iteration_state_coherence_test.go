package main

// cmd_loop_iteration_state_coherence_test.go — frozen RED contract for the
// iteration-state-coherence-sentinel lane (cycle-1693). DO NOT MODIFY: the
// Builder makes these pass with production code in cmd_loop_window.go /
// cmd_loop_blockerbreaker.go / cmd_loop_control.go only.
//
// THE GAP. unfinishedCycle (cmd_loop_control.go) is the only guard that reads
// canonical cycle-state as possibly stale, and it runs once per batch in
// prepareFreshBatch. prepareIteration (cmd_loop_window.go) — the chokepoint
// cmd_loop_batch.go runs before EVERY dispatch, sequential or fleet — never
// reads it. A fleet lane SIGKILLed by cmd_fleet.go's WaitDelay escalation dies
// before its abnormalEpilogue defer (cyclerun_epilogue.go) can apply the state
// floor (Phase="aborted", ActiveAgent=""), so the canonical record keeps
// claiming a live phase for a dead cycle. Once the batch has moved past that
// cycle (CycleID <= lastCycleNumber) unfinishedCycle cannot see it at all.
//
// THE CONTRACT these tests pin (inbox acceptance, verbatim scope):
//
//	stale in-flight record at the iteration top → WARN + Phase reconciled to
//	"aborted"; a genuinely resumable unfinished cycle (unfinishedCycle shape)
//	is NOT touched; wired at loopBatchCoordinator.prepareIteration for every
//	iteration, fleet waves included; go test -race.
//
// "Stale in-flight" is read as the conjunction the repo already has words for:
//   - the recorded phase is live: not fresh (CycleID 0), not a terminal phase
//     ("aborted", "end"), and not a phase the cycle already completed and
//     closed out (a cleanly finished lane's record is history, not residue);
//   - the owner is dead: no run lease, or a lease whose owner fails
//     runlease.OwnerLive (the SIGKILL shape is a FRESH heartbeat with a DEAD
//     pid — the lane was killed seconds before this iteration top);
//   - unfinishedCycle(cs, last) is false (the boot guard owns that shape).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : StaleCanonicalState (3 shapes, incl. the "phase=retro
//     residue" the epilogue's own comment names) + idempotence.
//   - Negative : ResumableCycleUntouched, OwnInFlightCycleUntouched,
//     CompletedCycleRecordUntouched, TerminalOrFreshRecordUntouched — every one
//     diffs the persisted record and the write log, never just "no error".
//   - Edge/OOD : CoherenceReadErrorSurfacesWithoutWrite,
//     ReconcileWriteErrorSurfaces (fail loudly, never clobber what was unread).
//   - Wiring   : WiredBeforeEveryIteration drives the production entrypoint
//     runLoop (sequential) and prepareIteration under wave AND pool fleet
//     configs (cmd_loop_batch.go:137 is the one call site for all three).

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	iscsLastCycle  = 1332 // the batch has completed through this cycle
	iscsStaleCycle = 1330 // a lane the batch already moved past
)

// iscsProject is a plain temp project (no .git, no policy.json): every other
// prepareIteration step no-ops cleanly on it, so batchProceed is reachable.
func iscsProject(t *testing.T) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", evolveDir, err)
	}
	return root, evolveDir
}

// iscsRunDir creates the run workspace a cycle's canonical record points at.
func iscsRunDir(t *testing.T, root string, cycle int) string {
	t.Helper()
	dir := cycleWorkspace(root, cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	return dir
}

// iscsExitedPID returns the pid of a process that has already exited and been
// reaped — a SIGKILLed lane's owner pid, discovered at runtime (never a
// literal pid).
func iscsExitedPID(t *testing.T) int {
	t.Helper()
	for attempt := 0; attempt < 3; attempt++ {
		cmd := exec.Command("true")
		if err := cmd.Run(); err != nil {
			t.Fatalf("spawn short-lived process: %v", err)
		}
		if pid := cmd.ProcessState.Pid(); !pidAlive(pid) {
			return pid
		}
	}
	t.Fatalf("could not obtain a dead pid in 3 attempts (pid reuse)")
	return 0
}

// iscsLease writes a run lease owned by pid with the given heartbeat.
func iscsLease(t *testing.T, runDir string, pid int, heartbeat time.Time) {
	t.Helper()
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01ISCSRUN", OwnerPID: pid}, heartbeat); err != nil {
		t.Fatalf("write lease in %s: %v", runDir, err)
	}
}

// iscsDossier writes the closeout dossier a cleanly completed cycle leaves.
func iscsDossier(t *testing.T, root string, cycle int) {
	t.Helper()
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	for _, ext := range []string{".json", ".md"} {
		p := filepath.Join(dir, fmt.Sprintf("cycle-%d%s", cycle, ext))
		body := fmt.Sprintf(`{"cycle":%d,"verdict":"PASS"}`, cycle)
		if ext == ".md" {
			body = fmt.Sprintf("# Cycle %d closeout\n", cycle)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}

// iscsCoordinator builds the minimal coordinator the existing prepareIteration
// tests use (TestPrepareIteration_PassesTheBatchCenterToTheRefresh), with the
// console captured so the WARN is observable.
func iscsCoordinator(root, evolveDir string, st core.Storage) (*loopBatchCoordinator, *bytes.Buffer) {
	var console bytes.Buffer
	b := &loopBatchCoordinator{
		ctx:    context.Background(),
		cfg:    loopConfig{ProjectRoot: root, EvolveDir: evolveDir},
		result: &loopResult{},
		stdout: &console,
		stderr: &console,
	}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, &console)
	b.deps.Storage = st
	return b, &console
}

// iscsPrepare runs one iteration top on the sequential (Count=1) config and
// requires it to reach batchProceed — a fixture that halts earlier proves
// nothing about the coherence check.
func iscsPrepare(t *testing.T, b *loopBatchCoordinator, console *bytes.Buffer, iteration int) {
	t.Helper()
	fc := policy.FleetConfig{Count: 1}
	bin := ""
	d := b.prepareIteration(iteration, &fc, &bin, iscsLastCycle)
	b.deps.Signals.Flush()
	if d.flow != batchProceed {
		t.Fatalf("fixture did not reach batchProceed (flow=%v exit=%d) — the coherence check must not halt the batch; console:\n%s",
			d.flow, d.exitCode, console.String())
	}
}

// iscsWarnLines returns the console lines that WARN about cycle n.
func iscsWarnLines(console string, cycle int) []string {
	id := fmt.Sprintf("%d", cycle)
	var out []string
	for _, line := range strings.Split(console, "\n") {
		if strings.Contains(line, "WARN") && strings.Contains(line, id) {
			out = append(out, line)
		}
	}
	return out
}

// iscsAssertUntouched is the byte-level negative check: the persisted record
// is deep-equal to the seed, nothing was written, nothing WARNed about it.
func iscsAssertUntouched(t *testing.T, st *fixtures.FakeStorage, seed core.CycleState, console string) {
	t.Helper()
	got, err := st.ReadCycleState(context.Background())
	if err != nil {
		t.Fatalf("read back cycle-state: %v", err)
	}
	if !reflect.DeepEqual(got, seed) {
		t.Errorf("canonical record was modified:\n got  %+v\n seed %+v", got, seed)
	}
	if n := len(st.CycleStateLog); n != 0 {
		t.Errorf("WriteCycleState called %d time(s) — this record must not be rewritten: %+v", n, st.CycleStateLog)
	}
	if lines := iscsWarnLines(console, seed.CycleID); seed.CycleID != 0 && len(lines) != 0 {
		t.Errorf("a WARN names cycle %d, which is not stale residue: %q", seed.CycleID, lines)
	}
}

// TestPrepareIteration_StaleCanonicalState — AC1 positive. A canonical record
// claiming a live phase for a dead cycle the batch already passed is WARNed
// and reconciled with the epilogue's state-floor semantics: Phase="aborted",
// ActiveAgent cleared, identity (CycleID, WorkspacePath, CompletedPhases)
// preserved — a reconcile, not a wipe. A second iteration top is quiet.
func TestPrepareIteration_StaleCanonicalState(t *testing.T) {
	cases := []struct {
		name      string
		phase     string
		completed []string
		lease     func(t *testing.T, runDir string)
	}{
		{
			name:      "sigkilled_lane_fresh_heartbeat_dead_owner",
			phase:     "build",
			completed: []string{"scout", "triage", "tdd"},
			lease: func(t *testing.T, runDir string) {
				iscsLease(t, runDir, iscsExitedPID(t), time.Now())
			},
		},
		{
			name:      "leaseless_record",
			phase:     "build",
			completed: []string{"scout", "triage", "tdd"},
			lease:     func(*testing.T, string) {},
		},
		{
			// The "two-hour stale phase=retro residue" cyclerun_epilogue.go's
			// state floor names: retro is live until it completes.
			name:      "killed_mid_retro",
			phase:     "retro",
			completed: []string{"scout", "triage", "tdd", "build", "audit", "ship"},
			lease: func(t *testing.T, runDir string) {
				iscsLease(t, runDir, iscsExitedPID(t), time.Now())
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, evolveDir := iscsProject(t)
			runDir := iscsRunDir(t, root, iscsStaleCycle)
			tc.lease(t, runDir)
			seed := core.CycleState{
				CycleID:         iscsStaleCycle,
				Phase:           tc.phase,
				ActiveAgent:     tc.phase,
				CompletedPhases: tc.completed,
				WorkspacePath:   runDir,
				StartedAt:       "2026-09-26T06:00:00Z",
			}
			st := &fixtures.FakeStorage{
				State:      core.State{LastCycleNumber: iscsLastCycle, LastAllocatedCycleNumber: iscsLastCycle},
				CycleState: seed,
			}
			if unfinishedCycle(seed, iscsLastCycle) {
				t.Fatalf("fixture error: seed is the unfinishedCycle shape, not stale residue")
			}
			b, console := iscsCoordinator(root, evolveDir, st)

			iscsPrepare(t, b, console, 1)

			got, err := st.ReadCycleState(context.Background())
			if err != nil {
				t.Fatalf("read back cycle-state: %v", err)
			}
			if got.Phase != "aborted" {
				t.Errorf("RED: stale record left at Phase=%q (want \"aborted\") — prepareIteration never reconciled canonical state; writes=%d",
					got.Phase, len(st.CycleStateLog))
			}
			if got.ActiveAgent != "" {
				t.Errorf("RED: ActiveAgent=%q after reconcile — the epilogue's state floor clears it", got.ActiveAgent)
			}
			if got.CycleID != seed.CycleID || got.WorkspacePath != seed.WorkspacePath ||
				!reflect.DeepEqual(got.CompletedPhases, seed.CompletedPhases) || got.StartedAt != seed.StartedAt {
				t.Errorf("reconcile must mark the phase, not wipe the record's identity:\n got  %+v\n seed %+v", got, seed)
			}
			warns := iscsWarnLines(console.String(), iscsStaleCycle)
			if len(warns) == 0 {
				t.Errorf("RED: no WARN on the loop console naming stale cycle %d; console:\n%s", iscsStaleCycle, console.String())
			} else if !strings.Contains(strings.Join(warns, "\n"), tc.phase) {
				t.Errorf("the WARN must name the stale phase %q it reconciled: %q", tc.phase, warns)
			}

			// Idempotence: the reconciled record is terminal — the next
			// iteration top neither rewrites it nor WARNs again.
			writes, warnCount := len(st.CycleStateLog), len(warns)
			iscsPrepare(t, b, console, 2)
			if n := len(st.CycleStateLog); n != writes {
				t.Errorf("second iteration top rewrote the already-reconciled record (%d → %d writes)", writes, n)
			}
			if n := len(iscsWarnLines(console.String(), iscsStaleCycle)); n != warnCount {
				t.Errorf("second iteration top re-WARNed about cycle %d (%d → %d lines) — the sentinel must be quiet once coherent", iscsStaleCycle, warnCount, n)
			}
		})
	}
}

// TestPrepareIteration_ResumableCycleUntouched — AC1 negative. The boot-guard
// shape (unfinishedCycle: CycleID > lastCycleNumber) belongs to
// `evolve loop --resume` / `evolve cycle reset`; the iteration sentinel must
// leave it byte-identical even when its owner is dead.
func TestPrepareIteration_ResumableCycleUntouched(t *testing.T) {
	const resumable = iscsLastCycle + 1
	for _, withLease := range []bool{true, false} {
		t.Run(fmt.Sprintf("dead_owner_lease=%v", withLease), func(t *testing.T) {
			root, evolveDir := iscsProject(t)
			runDir := iscsRunDir(t, root, resumable)
			if withLease {
				iscsLease(t, runDir, iscsExitedPID(t), time.Now())
			}
			seed := core.CycleState{
				CycleID:         resumable,
				Phase:           "build",
				ActiveAgent:     "build",
				CompletedPhases: []string{"scout", "triage", "tdd"},
				WorkspacePath:   runDir,
			}
			st := &fixtures.FakeStorage{
				State:      core.State{LastCycleNumber: iscsLastCycle, LastAllocatedCycleNumber: resumable + 1},
				CycleState: seed,
			}
			if !unfinishedCycle(seed, iscsLastCycle) {
				t.Fatalf("fixture error: seed must be the unfinishedCycle (resumable) shape")
			}
			b, console := iscsCoordinator(root, evolveDir, st)

			iscsPrepare(t, b, console, 1)

			iscsAssertUntouched(t, st, seed, console.String())
		})
	}
}

// TestPrepareIteration_OwnInFlightCycleUntouched — the "never touch a live
// lane" edge. The record's cycle is already <= lastCycleNumber, so ONLY owner
// liveness distinguishes it from residue: a fresh lease held by a live pid
// (this very process — the loop's own in-flight cycle) must stay untouched.
func TestPrepareIteration_OwnInFlightCycleUntouched(t *testing.T) {
	root, evolveDir := iscsProject(t)
	runDir := iscsRunDir(t, root, iscsStaleCycle)
	iscsLease(t, runDir, os.Getpid(), time.Now())
	seed := core.CycleState{
		CycleID:         iscsStaleCycle,
		Phase:           "build",
		ActiveAgent:     "build",
		CompletedPhases: []string{"scout", "triage", "tdd"},
		WorkspacePath:   runDir,
	}
	st := &fixtures.FakeStorage{
		State:      core.State{LastCycleNumber: iscsLastCycle, LastAllocatedCycleNumber: iscsLastCycle},
		CycleState: seed,
	}
	b, console := iscsCoordinator(root, evolveDir, st)

	iscsPrepare(t, b, console, 1)

	iscsAssertUntouched(t, st, seed, console.String())
}

// TestPrepareIteration_CompletedCycleRecordUntouched — over-correction guard.
// A cleanly finished fleet lane leaves its last phase recorded AND completed,
// a closeout dossier, and a lease whose owner exited normally. That is
// history, not an in-flight claim: WARNing (and relabelling a shipped cycle
// "aborted") on every healthy lane would bury the one real signal.
func TestPrepareIteration_CompletedCycleRecordUntouched(t *testing.T) {
	root, evolveDir := iscsProject(t)
	runDir := iscsRunDir(t, root, iscsLastCycle)
	iscsLease(t, runDir, iscsExitedPID(t), time.Now())
	iscsDossier(t, root, iscsLastCycle)
	seed := core.CycleState{
		CycleID:         iscsLastCycle,
		Phase:           "retro",
		ActiveAgent:     "retro",
		CompletedPhases: []string{"scout", "triage", "tdd", "build", "audit", "ship", "retro"},
		WorkspacePath:   runDir,
		Shipped:         true,
		FinalVerdict:    "PASS",
	}
	st := &fixtures.FakeStorage{
		State:      core.State{LastCycleNumber: iscsLastCycle, LastAllocatedCycleNumber: iscsLastCycle},
		CycleState: seed,
	}
	b, console := iscsCoordinator(root, evolveDir, st)

	iscsPrepare(t, b, console, 1)

	iscsAssertUntouched(t, st, seed, console.String())
}

// TestPrepareIteration_TerminalOrFreshRecordUntouched — records with nothing
// to reconcile: a fresh tree, an already-aborted record (the epilogue ran, or
// a previous iteration reconciled it), and the end marker.
func TestPrepareIteration_TerminalOrFreshRecordUntouched(t *testing.T) {
	cases := []struct {
		name  string
		cycle int
		phase string
	}{
		{"fresh_tree", 0, ""},
		{"already_aborted", iscsStaleCycle, "aborted"},
		{"end_marker", iscsStaleCycle, "end"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, evolveDir := iscsProject(t)
			seed := core.CycleState{CycleID: tc.cycle, Phase: tc.phase}
			if tc.cycle != 0 {
				runDir := iscsRunDir(t, root, tc.cycle)
				iscsLease(t, runDir, iscsExitedPID(t), time.Now())
				seed.WorkspacePath = runDir
				seed.CompletedPhases = []string{"scout", "triage"}
			}
			st := &fixtures.FakeStorage{
				State:      core.State{LastCycleNumber: iscsLastCycle, LastAllocatedCycleNumber: iscsLastCycle},
				CycleState: seed,
			}
			b, console := iscsCoordinator(root, evolveDir, st)

			iscsPrepare(t, b, console, 1)

			iscsAssertUntouched(t, st, seed, console.String())
		})
	}
}

// TestPrepareIteration_CoherenceReadErrorSurfacesWithoutWrite — an unreadable
// canonical record is reported on the loop console (never swallowed) and is
// never overwritten: the sentinel cannot reconcile what it could not read.
func TestPrepareIteration_CoherenceReadErrorSurfacesWithoutWrite(t *testing.T) {
	root, evolveDir := iscsProject(t)
	const marker = "iscs-injected: cycle-state unreadable"
	st := &fixtures.FakeStorage{
		State:             core.State{LastCycleNumber: iscsLastCycle, LastAllocatedCycleNumber: iscsLastCycle},
		ReadCycleStateErr: errors.New(marker),
	}
	b, console := iscsCoordinator(root, evolveDir, st)
	fc := policy.FleetConfig{Count: 1}
	bin := ""

	b.prepareIteration(1, &fc, &bin, iscsLastCycle)
	b.deps.Signals.Flush()

	if n := len(st.CycleStateLog); n != 0 {
		t.Errorf("WriteCycleState called %d time(s) after a failed read — never clobber an unread record", n)
	}
	if !strings.Contains(console.String(), marker) {
		t.Errorf("RED: the cycle-state read error was swallowed — it must surface on the loop console; console:\n%s", console.String())
	}
}

// TestPrepareIteration_ReconcileWriteErrorSurfaces — a failed reconcile write
// is reported on the loop console, not silently dropped.
func TestPrepareIteration_ReconcileWriteErrorSurfaces(t *testing.T) {
	root, evolveDir := iscsProject(t)
	runDir := iscsRunDir(t, root, iscsStaleCycle)
	iscsLease(t, runDir, iscsExitedPID(t), time.Now())
	const marker = "iscs-injected: disk full"
	st := &fixtures.FakeStorage{
		State: core.State{LastCycleNumber: iscsLastCycle, LastAllocatedCycleNumber: iscsLastCycle},
		CycleState: core.CycleState{
			CycleID: iscsStaleCycle, Phase: "build", ActiveAgent: "build",
			CompletedPhases: []string{"scout", "triage", "tdd"}, WorkspacePath: runDir,
		},
		WriteCycleStateErr: errors.New(marker),
	}
	b, console := iscsCoordinator(root, evolveDir, st)
	fc := policy.FleetConfig{Count: 1}
	bin := ""

	b.prepareIteration(1, &fc, &bin, iscsLastCycle)
	b.deps.Signals.Flush()

	if !strings.Contains(console.String(), marker) {
		t.Errorf("RED: the reconcile write error was not surfaced on the loop console; console:\n%s", console.String())
	}
}

// TestPrepareIteration_WiredBeforeEveryIteration — AC2 wiring proof. The one
// call site (cmd_loop_batch.go:137) serves the sequential path and both fleet
// schedulers, so the proof drives:
//   - the production entrypoint runLoop (sequential): the reconcile write for
//     the stale cycle lands BEFORE the batch's own cycle writes any state;
//   - prepareIteration under a wave and a pool fleet config (iteration 1, i.e.
//     NOT the batch's first dispatch — prepareFreshBatch is not in play), so a
//     check gated on the sequential config, or run once per batch, fails.
func TestPrepareIteration_WiredBeforeEveryIteration(t *testing.T) {
	staleSeed := func(t *testing.T, root string) core.CycleState {
		runDir := iscsRunDir(t, root, iscsStaleCycle)
		iscsLease(t, runDir, iscsExitedPID(t), time.Now())
		return core.CycleState{
			CycleID: iscsStaleCycle, Phase: "build", ActiveAgent: "build",
			CompletedPhases: []string{"scout", "triage", "tdd"}, WorkspacePath: runDir,
		}
	}

	t.Run("sequential_runLoop_entrypoint", func(t *testing.T) {
		root, evolveDir := iscsProject(t)
		writeDispatchPolicy(t, evolveDir, "off")
		st := &fixtures.FakeStorage{
			State:      core.State{LastCycleNumber: iscsLastCycle, LastAllocatedCycleNumber: iscsLastCycle},
			CycleState: staleSeed(t, root),
		}
		defer installStubDeps(t, st, newFakeLedger())()

		var stdout, stderr bytes.Buffer
		runLoop([]string{
			"--project-root", root,
			"--evolve-dir", evolveDir,
			"--goal-text", "x",
			"--cycles", "1",
		}, nil, &stdout, &stderr)

		reconciled, ownWrite := -1, -1
		var trail []string
		for i, cs := range st.CycleStateLog {
			trail = append(trail, fmt.Sprintf("%d/%s", cs.CycleID, cs.Phase))
			if reconciled < 0 && cs.CycleID == iscsStaleCycle && cs.Phase == "aborted" {
				reconciled = i
			}
			if ownWrite < 0 && cs.CycleID != iscsStaleCycle {
				ownWrite = i
			}
		}
		if ownWrite < 0 {
			t.Fatalf("fixture error: runLoop dispatched no cycle of its own (no foreign cycle-state write); stderr:\n%s", stderr.String())
		}
		if reconciled < 0 {
			t.Fatalf("RED: runLoop never reconciled stale cycle %d to \"aborted\" — the iteration top is not wired; write trail (cycle/phase): %v", iscsStaleCycle, trail)
		}
		if reconciled > ownWrite {
			t.Errorf("stale cycle %d reconciled at write #%d, AFTER the batch's own cycle began writing (#%d) — it must run at the iteration top, before dispatch",
				iscsStaleCycle, reconciled, ownWrite)
		}
		if len(iscsWarnLines(stderr.String(), iscsStaleCycle)) == 0 {
			t.Errorf("RED: runLoop's console carries no WARN naming stale cycle %d; stderr:\n%s", iscsStaleCycle, stderr.String())
		}
	})

	for _, fleet := range []struct {
		name    string
		policy  string
		enabled func(policy.FleetConfig) bool
	}{
		{"fleet_wave_config", `{"fleet":{"count":3,"scheduling":"wave"}}`, shouldRunWave},
		{"fleet_pool_config", `{"fleet":{"count":3,"scheduling":"pool"}}`, shouldRunPool},
	} {
		t.Run(fleet.name, func(t *testing.T) {
			root, evolveDir := iscsProject(t)
			if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(fleet.policy), 0o644); err != nil {
				t.Fatalf("write policy.json: %v", err)
			}
			st := &fixtures.FakeStorage{
				State:      core.State{LastCycleNumber: iscsLastCycle, LastAllocatedCycleNumber: iscsLastCycle},
				CycleState: staleSeed(t, root),
			}
			b, console := iscsCoordinator(root, evolveDir, st)
			fc := loadFleetConfig(evolveDir)
			if !fleet.enabled(fc) {
				t.Fatalf("fixture error: %s does not select the fleet path: %+v", fleet.policy, fc)
			}
			bin := ""

			d := b.prepareIteration(1, &fc, &bin, iscsLastCycle)
			b.deps.Signals.Flush()

			if d.flow != batchProceed {
				t.Fatalf("fleet iteration top did not reach batchProceed (flow=%v exit=%d); console:\n%s", d.flow, d.exitCode, console.String())
			}
			if !fleet.enabled(fc) {
				t.Fatalf("fixture error: the fleet config did not survive the wave-boundary reload: %+v", fc)
			}
			got, err := st.ReadCycleState(context.Background())
			if err != nil {
				t.Fatalf("read back cycle-state: %v", err)
			}
			if got.Phase != "aborted" || got.CycleID != iscsStaleCycle {
				t.Errorf("RED: %s iteration top left stale record at cycle=%d phase=%q (want %d/\"aborted\")",
					fleet.name, got.CycleID, got.Phase, iscsStaleCycle)
			}
			if len(iscsWarnLines(console.String(), iscsStaleCycle)) == 0 {
				t.Errorf("RED: %s iteration top emitted no WARN naming stale cycle %d; console:\n%s", fleet.name, iscsStaleCycle, console.String())
			}
		})
	}
}
