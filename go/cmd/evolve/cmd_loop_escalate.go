// cmd_loop_escalate.go — the per-iteration ESCALATION BOUNDARY seam
// (failure-disposition-router S4 / chronicle-s6-escalation-boundary).
//
// S3 (internal/dispositionrouter) stages escalate/autofile intents while lanes
// are still running; it may never write .evolve/inbox/ because that races
// inboxmover.Claim's os.Rename. This file is the other half: it applies those
// staged intents at the ONE moment no lane is in flight — right after an
// iteration's dispatch returns and before the next one is planned.
//
// Best-effort by construction: an escalation failure WARNs and never breaks
// dispatch (the queue is an optimization; the loop is the product).
package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// applyEscalationBoundary applies every intent staged under
// <evolveDir>/escalations at the iteration boundary, writing its report to
// <evolveDir>/escalation-apply-report.json. Stage and escalation formula come
// from policy.json's failure_disposition block (compiled defaults when absent),
// so shadow-vs-enforce is config, not a flag.
func applyEscalationBoundary(evolveDir string, cycle int, stderr io.Writer, signals *signalcenter.Center) {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: escalation boundary: policy load: %v\n", err)
		return
	}
	cfg := pol.FailureDispositionConfig()
	res, err := recurrence.ApplyBoundary(recurrence.ApplyOptions{
		InboxDir:        filepath.Join(evolveDir, "inbox"),
		EscalationsPath: dispositionrouter.PendingActionsPath(filepath.Join(evolveDir, "escalations")),
		ReportPath:      filepath.Join(evolveDir, "escalation-apply-report.json"),
		Cycle:           cycle,
		Shadow:          !cfg.Enforce(),
		Policy: recurrence.EscalationPolicy{
			Threshold: cfg.Threshold,
			Step:      cfg.Step,
			Cap:       cfg.Cap,
		},
		Now: time.Now().UTC(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: escalation boundary: %v\n", err)
		return
	}
	if len(res.Bumped)+len(res.Filed)+len(res.Planned) == 0 {
		return // nothing staged — stay quiet
	}
	// ADR-0101 S4a: the boundary's outcome is a loop.escalation WARN.
	emitLoopEscalation(signals, cycle, "applyEscalationBoundary",
		fmt.Sprintf("escalation boundary staged %d item(s)", len(res.Bumped)+len(res.Filed)+len(res.Planned)),
		map[string]string{"stage": string(cfg.Stage), "bumped": strconv.Itoa(len(res.Bumped)), "filed": strconv.Itoa(len(res.Filed)),
			"skipped": strconv.Itoa(len(res.Skipped)), "planned": strconv.Itoa(len(res.Planned))})
}
