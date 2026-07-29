// Package cycleoutcome is the importable FAIL-closeout seam: the symmetric
// sibling of the PASS half that already lives inside the cycle process
// (internal/phases/ship/postship.go).
//
// Why a package and not more inline `main` code: the FAIL closeout used to be
// an inline block in cmd_loop.go's SEQUENTIAL branch only. `package main` is
// not importable, so (a) no predicate could ever pin the behavior, and (b) the
// wave path — whose lanes run `evolve cycle run` as a subprocess and whose
// fleet.Result{Index,ExitCode,Err} carries neither cycle number nor workspace —
// had no way to reuse it. Fleet-dispatched work therefore never bumped
// failure_count and the ADR-0072 S5 retry ceiling was structurally unreachable
// (wave-lane-task-quarantine-dead; batch-14 burned four FAILs on the same ids
// with failure_count stuck at 0).
//
// Both closeout paths now call ApplyFailure, so the two halves cannot drift
// apart again (never_duplicate_centralize).
package cycleoutcome

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// FailureInputs is one failed cycle's closeout context. Workspace is the
// cycle's .evolve/runs/cycle-N dir — the sole source of the committed set
// (triage-decision.json), so the seam bumps exactly the ids triage worked and
// never a lane's whole claimed menu (PR #366 menu semantics).
type FailureInputs struct {
	ProjectRoot string    // repo root containing .evolve/
	Workspace   string    // .evolve/runs/cycle-N
	Cycle       int       // cycle number
	Ceiling     int       // FailureThresholds.TaskRetryCeiling; <=0 disables quarantine
	SystemLevel bool      // ADR-0072 S3: releases, never bumps, never quarantines
	Reason      string    // ledger reason ("" = default)
	Stderr      io.Writer // nil = discard
}

// ApplyFailure walks a failed cycle's committed ids through the inbox failure
// lifecycle: claim (if the lane never did) → bump failure_count → quarantine at
// Ceiling. Uncommitted menu ids release untouched.
//
// An absent or unreadable triage-decision.json yields a nil committed set,
// which is exactly the legacy whole-dir drain behavior — a missing decision
// must never turn into a silent no-op that strands claims in processing/.
func ApplyFailure(in FailureInputs) (inboxmover.OutcomeResult, error) {
	stderr := in.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	res, err := inboxmover.ApplyCycleOutcome(inboxmover.Options{
		ProjectRoot: in.ProjectRoot,
		Stderr:      stderr,
	}, inboxmover.CycleOutcome{
		Cycle:        in.Cycle,
		Passed:       false,
		CommittedIDs: CommittedIDsFor(in.Workspace),
		Reason:       in.Reason,
		Ceiling:      in.Ceiling,
		SystemLevel:  in.SystemLevel,
	})
	if err != nil {
		return res, fmt.Errorf("apply cycle %d failure outcome: %w", in.Cycle, err)
	}
	return res, nil
}

// FailureInputsFor derives the closeout context a caller cannot state on its
// own: the S5 retry ceiling (from .evolve/policy.json, falling back to the
// compiled default) and whether the failure was task-level. Both call sites go
// through it so the classification rule has ONE definition.
func FailureInputsFor(projectRoot, evolveDir, workspace string, cycle int, stderr io.Writer) FailureInputs {
	failPol := policy.DefaultSystemFailurePolicy()
	if pol, err := policy.Load(filepath.Join(evolveDir, "policy.json")); err == nil {
		if fp, fpErr := pol.FailurePolicyConfig(); fpErr == nil {
			failPol = fp
		}
	}
	return FailureInputs{
		ProjectRoot: projectRoot,
		Workspace:   workspace,
		Cycle:       cycle,
		Ceiling:     failPol.Thresholds.TaskRetryCeiling,
		SystemLevel: !IsTaskLevelFailure(cycleclassify.Classify(workspace).Class),
		Reason:      "cycle-failure-release",
		Stderr:      stderr,
	}
}

// IsTaskLevelFailure reports whether the classification blames the TASK (and so
// may accrue failure_count toward the S5 ceiling). Transient infra, quota storms
// and kernel breaches are the pipeline's fault, not the todo's (ADR-0072 AC4).
// Exported because the loop's quarantine surface asks the same question — one
// definition, so "what counts as the task's fault" cannot fork.
func IsTaskLevelFailure(c cycleclassify.Classification) bool {
	switch c {
	case cycleclassify.ClassBuildFail, cycleclassify.ClassAuditFail, cycleclassify.ClassShipGateConfig:
		return true
	default:
		return false
	}
}

// CommittedIDsFor reads a cycle workspace's triage-decision.json (written by the
// triage phase, so it exists even when the cycle never reached ship) and returns
// the ids triage committed to working. nil on any absent/unreadable/malformed
// decision. Exported as the ONE workspace-level reader of the committed set, so
// the failure seam and the loop's attempt accounting cannot disagree about what
// "the worked set" means.
func CommittedIDsFor(workspace string) []string {
	body, err := os.ReadFile(filepath.Join(workspace, "triage-decision.json"))
	if err != nil {
		return nil
	}
	return inboxmover.CommittedIDs(body)
}
