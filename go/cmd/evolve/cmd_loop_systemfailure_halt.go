package main

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

// systemFailureHaltExitCode is the process exit code a cycle run returns when an
// ADR-0072 SYSTEM-level failure (a forged/incoherent verdict, not a task-code
// failure) mandates a loop halt. It is distinct from the ordinary FAIL mapping
// (rc=2) and the soft batch-complete rc=3, so the parent wave/fleet loop can
// tell a forged-verdict halt apart from an ordinary task-level lane failure and
// stop the batch instead of continuing to the next wave.
const systemFailureHaltExitCode = 4

// cycleRunExitCode is the single, side-effect-free exit-code mapping every
// cycle-run boundary consults — runCycleRun (the fleet-lane subprocess
// entrypoint) directly, and the sequential single-cycle path via
// haltOnSystemFailure. A halting SystemFailure takes priority over the recorded
// verdict: the pipeline is untrustworthy, so the halt code is returned
// regardless of FinalVerdict (even a forged PASS). Without a halting
// SystemFailure the historical mapping is preserved exactly: FAIL → 2,
// everything else → 0.
func cycleRunExitCode(res cyclestate.CycleResult) int {
	if sf := res.SystemFailure; sf != nil && sf.Halt {
		return systemFailureHaltExitCode
	}
	if res.FinalVerdict == cyclestate.VerdictFAIL {
		return 2
	}
	return 0
}

// haltOnSystemFailure is the ONE shared halt+escalate action (ADR-0072 AC2)
// invoked by BOTH the sequential single-cycle path (cmd_loop.go) and each fleet
// lane subprocess (runCycleRun) — so the escalation dossier, the P0 inbox item,
// and the halt exit code are produced identically on every code path instead of
// the logic being duplicated inline per call site. It writes the escalation
// dossier + P0 pipeline-repair inbox item (writePipelineEscalation), prints the
// operator-facing halt message, and returns systemFailureHaltExitCode so the
// caller propagates the halt via its exit code.
func haltOnSystemFailure(evolveDir, projectRoot string, cycle int, workspace string, sf *cyclestate.SystemFailureSignal, w io.Writer) int {
	writePipelineEscalation(evolveDir, projectRoot, cycle, workspace, sf, w)
	fmt.Fprintf(w, "[loop] SYSTEM-FAILURE HALT: cycle=%d category=%s level=%s\n[loop]   %s\n[loop]   The pipeline (not the task) is the cause — diagnose + fix before resuming; a P0 pipeline-repair item was filed to .evolve/inbox/. Escalation: .evolve/pipeline-escalation.json\n",
		cycle, sf.Category, sf.Level, sf.Evidence)
	return systemFailureHaltExitCode
}

// anyLaneHaltedForSystemFailure reports whether any fleet lane result exited
// with the ADR-0072 system-failure halt code. The wave/fleet dispatch loop
// consults it to stop dispatching further waves: a forged verdict makes the
// pipeline untrustworthy fleet-wide, so one halting lane stops the whole batch.
// An ordinary lane FAIL (rc=2) or launch error (rc=-1/1) is deliberately NOT
// conflated with a halt — those keep the never-stop retry semantics ADR-0072
// draws the line at.
func anyLaneHaltedForSystemFailure(results []fleet.Result) bool {
	for _, r := range results {
		if r.ExitCode == systemFailureHaltExitCode {
			return true
		}
	}
	return false
}
