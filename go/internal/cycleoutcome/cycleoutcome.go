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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// FailureInputs is one failed cycle's closeout context. Workspace is the
// cycle's .evolve/runs/cycle-N dir — the sole source of the committed set
// (triage-decision.json), so the seam bumps exactly the ids triage worked and
// never a lane's whole claimed menu (PR #366 menu semantics).
type FailureInputs struct {
	ProjectRoot string // repo root containing .evolve/
	Workspace   string // .evolve/runs/cycle-N
	Cycle       int    // cycle number
	Ceiling     int    // FailureThresholds.TaskRetryCeiling; <=0 disables quarantine
	SystemLevel bool   // ADR-0072 S3: releases, never bumps, never quarantines
	// Refusal is the C1 record's diagnostic code when the cycle stopped on a
	// phase gate's own refusal (cycleclassify.ClassPhaseRefusal), with the
	// refusing message and the item it named; empty otherwise. A
	// TRIAGE_PROTECTED_SURFACE refusal routes its Subject console-manual.
	Refusal        string
	RefusalDetail  string
	RefusalSubject string
	Reason         string    // ledger reason ("" = default)
	Stderr         io.Writer // nil = discard
	// Ledger is the chained-append seam the lifecycle lines go through — the
	// root's Signal-Center-observed ledger (ADR-0101 S4a). nil lets the inbox
	// mover fall back to its own, unobserved file ledger over the same file.
	Ledger inboxmover.LedgerAppender
	// Signals is the root's Signal Center the inbox mover's inbox.warning
	// events go to (ADR-0103 unit 06). nil = unwired: the mover prints its
	// legacy [inbox-mover] line instead.
	Signals *signalcenter.Center
}

// WithLedger returns the inputs with the lifecycle ledger set (the receiver
// is left untouched).
func (in FailureInputs) WithLedger(l inboxmover.LedgerAppender) FailureInputs {
	in.Ledger = l
	return in
}

// WithSignals returns the inputs with the Signal Center set (the receiver is
// left untouched).
func (in FailureInputs) WithSignals(c *signalcenter.Center) FailureInputs {
	in.Signals = c
	return in
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
	// Continuation/retry cycles carry NO triage-decision.json (the
	// continuation path binds the task directly), so the committed set
	// resolved nil and the durable failure_count was never bumped — after 15
	// FAILs every live inbox item sat at 0 and the TaskRetryCeiling
	// quarantine + deep-escalation governors were unreachable (2026-08-10
	// investigation). The lane-scope pin is the worked set on those cycles.
	// File-ABSENT, not merely empty (diff-review MEDIUM): a fresh cycle whose
	// triage legitimately committed zero ids must not have its lane-scope menu
	// blamed for the failure — the pinned items were explicitly declined.
	committed := CommittedIDsFor(in.Workspace)
	if len(committed) == 0 {
		if _, statErr := os.Stat(filepath.Join(in.Workspace, "triage-decision.json")); os.IsNotExist(statErr) {
			committed = LaneScopeIDs(in.Workspace)
		}
	}
	opts := inboxmover.Options{
		ProjectRoot: in.ProjectRoot,
		Ledger:      in.Ledger,
		Stderr:      stderr,
		Signals:     in.Signals,
	}
	// The per-item breaker (docs/incidents/2026-09-14-triage-refusal-poison-loop.md):
	// a protected-surface refusal is deterministic AND operator-owned, so the
	// refused card is routed console-manual in place — wherever the lane's
	// claim left it — BEFORE the drain releases it, and the claim floor refuses
	// every later lane. The route is that item's disposition and the cycle's
	// other committed ids were never worked (triage stopped the cycle), so the
	// drain then runs with Routed set (no bump, no park — SystemLevel stays the
	// fact it is). A route that cannot happen has said so on the stream and
	// falls back to the task-level bump — the S5 ceiling is the second breaker.
	routed := false
	if cyclestate.RefusalDisposition(in.Refusal).RouteConsole && in.RefusalSubject != "" {
		if !containsID(committed, in.RefusalSubject) {
			// An agent-authored id never widens what an agent may do (ADR-0073):
			// a Subject outside this cycle's committed set is a mis-copy, not a
			// disposition — say so and let the task-level bump run.
			fmt.Fprintf(stderr, "[cycleoutcome] WARN: route-console skipped: refused subject %q is not in the committed set %v — task-level bump instead\n", in.RefusalSubject, committed)
		} else if _, rerr := inboxmover.RouteConsole(opts, in.RefusalSubject, in.RefusalDetail, in.Cycle); rerr == nil {
			routed = true
		}
	}
	res, err := inboxmover.ApplyCycleOutcome(opts, inboxmover.CycleOutcome{
		Cycle:        in.Cycle,
		Passed:       false,
		CommittedIDs: committed,
		Reason:       in.Reason,
		Ceiling:      in.Ceiling,
		SystemLevel:  in.SystemLevel,
		Routed:       routed,
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
	cls := cycleclassify.Classify(workspace)
	in := FailureInputs{
		ProjectRoot: projectRoot,
		Workspace:   workspace,
		Cycle:       cycle,
		Ceiling:     failPol.Thresholds.TaskRetryCeiling,
		SystemLevel: !IsTaskLevelResult(cls),
		Reason:      "cycle-failure-release",
		Stderr:      stderr,
	}
	if cls.Class == cycleclassify.ClassPhaseRefusal {
		in.Refusal, in.RefusalDetail, in.RefusalSubject = cls.Marker, cls.Detail, cls.Subject
	}
	return in
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

// LaneScopeIDs reads the lane-scope pin's todo ids from the workspace — the
// triage-less (continuation/lane) committed set, shared by BOTH outcome
// halves: the FAIL closeout here (failure_count accounting, PR #439) and the
// PASS promotion in phases/ship/postship.go (consumption-rides-landing-ship)
// — one reader, so the two sides can never disagree about what a triage-less
// cycle worked.
// Delegates to the same projection used by the TDD and Build contracts.
func LaneScopeIDs(workspace string) []string {
	return core.LaneScopeIDs(workspace)
}

// IsTaskLevelResult is IsTaskLevelFailure over the whole classification: a
// phase-refusal blames the task only per its code's disposition
// (cyclestate.RefusalDisposition — the table beside the vocabulary), so an
// I/O fault stamped TRIAGE_COMMITMENT_INVALID never charges the queue while a
// card that named a protected surface does. Callers holding only the class
// keep IsTaskLevelFailure, where a refusal is not task-level by default.
func IsTaskLevelResult(cls cycleclassify.Result) bool {
	if cls.Class == cycleclassify.ClassPhaseRefusal {
		return cyclestate.RefusalDisposition(cls.Marker).TaskLevel
	}
	return IsTaskLevelFailure(cls.Class)
}

// containsID reports whether id is one of the committed ids.
func containsID(ids []string, id string) bool {
	for _, c := range ids {
		if c == id {
			return true
		}
	}
	return false
}
