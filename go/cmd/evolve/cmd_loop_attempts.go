package main

// cmd_loop_attempts.go — ADR-0080 P2 loop wiring: after a task-level graded
// FAIL, bump the durable failure_count of each triage-COMMITTED id where the
// item lives (the inbox root — wave lanes never claim into processing/, so
// the ADR-0072 S5 release-path accounting was structurally unreachable and
// the retry ceiling never fired: 12-lane grinds with failure_count 0).
// Menu semantics: only ids the cycle's triage COMMITTED bump; unworked menu
// ids never do. System-level failures (infra, guards) are not the task's
// fault and are excluded by the caller, mirroring the release path's AC4.

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// recordCommittedFailures applies the FAIL-side accounting for one graded
// task-level FAIL: one bump per committed id, quarantine at the ceiling.
// Best-effort and loud — accounting must never turn a FAIL into an abort.
func recordCommittedFailures(projectRoot, workspace string, cycle, ceiling int, warn io.Writer) {
	// failedCycleCommittedIDs is the same-package SSOT (delegates to
	// inboxmover.CommittedIDs): top_n AND skip_shipped, DEDUPED — a local
	// re-parse double-bumped duplicate ids and missed skip_shipped entirely
	// (review HIGH: two failure shapes yielded two different committed sets).
	for _, id := range failedCycleCommittedIDs(workspace) {
		count, quarantined, err := inboxmover.RecordRootTaskFailure(
			inboxmover.Options{ProjectRoot: projectRoot, Stderr: warn}, id, cycle, "graded cycle FAIL", ceiling)
		switch {
		case err != nil:
			fmt.Fprintf(warn, "[loop] WARN: failure accounting for %q: %v\n", id, err)
		case quarantined:
			fmt.Fprintf(warn, "[loop] task %q QUARANTINED after %d failed attempts (ceiling %d) — release with: evolve inbox quarantine release %s\n", id, count, ceiling, id)
		case count > 0:
			fmt.Fprintf(warn, "[loop] task %q failure_count=%d/%d\n", id, count, ceiling)
		}
	}
}
