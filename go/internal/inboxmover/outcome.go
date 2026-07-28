package inboxmover

// outcome.go — the SINGLE cycle-outcome lifecycle seam.
//
// Before this file the inbox lifecycle had two half-implementations and a hole
// in the middle:
//
//   - PASS side: promotion was agent-driven prose, so cycle-1147 shipped three
//     menu items in ONE commit and promoted none of them — processed/cycle-1147/
//     was empty and all three re-entered the very next triage
//     (menu-pass-promotes-committed-ids).
//   - FAIL side: ReleaseCycleProcessingWithQuarantine is the only
//     bumpFailureCount caller and it walks ONLY processing/cycle-N/. Nothing
//     ever put a wave lane's worked ids there, so the ADR-0072 S5 retry ceiling
//     was structurally unreachable for fleet work — batch-14 burned four FAILs
//     on the same items with failure_count never leaving 0
//     (wave-lane-task-quarantine-dead).
//
// ApplyCycleOutcome is the one entry point both closeout paths now call, so the
// PASS-promote and FAIL-bump halves cannot drift apart again
// (never_duplicate_centralize).

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// CycleOutcome is the verdict-shaped input to the lifecycle seam: what the
// cycle was scoped to, what it actually committed to working, and how it ended.
type CycleOutcome struct {
	Cycle        int      // cycle number
	Passed       bool     // true = PASS (promote committed ids), false = FAIL (bump/quarantine)
	CommittedIDs []string // triage-decision.json top_n/skip_shipped — the WORKED set
	LaneIDs      []string // full lane/menu scope (superset; may be nil)
	CommitSHA    string   // ship SHA, PASS only ("" = no SHA prefix)
	Reason       string   // ledger reason ("" = default)
	Ceiling      int      // FailureThresholds.TaskRetryCeiling (FAIL only; <=0 disables quarantine)
	SystemLevel  bool     // ADR-0072 S3 system failure: NEVER quarantines (AC4)
}

// OutcomeResult reports what the seam moved, by task id or destination path.
type OutcomeResult struct {
	Promoted    []string // committed ids moved to processed/cycle-N/ (PASS)
	Released    []string // paths released back to the inbox root
	Quarantined []string // paths parked in quarantine/ (FAIL at ceiling)
}

// ApplyCycleOutcome applies one cycle's verdict to the inbox lifecycle.
//
// PASS: every committed id is promoted to processed/cycle-<N>/ (resolvable from
// processing/cycle-*/ OR the inbox root — promotion never depends on a prior
// claim), then the residual claims for the cycle drain back to the inbox root.
// Uncommitted menu ids are left exactly where they are, pending for the next
// triage. Re-applying the same PASS is an idempotent no-op: an already-promoted
// id resolves nowhere and takes Promote's NoOp path.
//
// FAIL: the committed ids are claimed into processing/cycle-<N>/ if they are
// not already there (see ClaimLaneScope for why the claim happens HERE and not
// at dispatch), then the drain bumps the durable failure_count on those ids
// ONLY and quarantines any that reach Ceiling. Uncommitted menu ids release
// untouched — no phase worked them, so they must not accrue task-level
// failures. Nothing is ever promoted to processed/ on a FAIL.
func ApplyCycleOutcome(opts Options, oc CycleOutcome) (OutcomeResult, error) {
	opts.resolveOpts()
	res := OutcomeResult{}
	committed := dedupeIDs(oc.CommittedIDs)

	if oc.Passed {
		cycleStr := strconv.Itoa(oc.Cycle)
		// Promote failures are COLLECTED, never early-returned: the residual
		// drain below is the invariant that keeps claimed items from rotting in
		// processing/cycle-N/ across cycles (orphans in 124/265/294/295/308),
		// and returning on the first failed promote would skip it entirely —
		// trading one loud non-delivery for a silent multi-item strand. Every
		// error still reaches the caller, joined, after the drain has run.
		var errs []error
		for _, id := range committed {
			pr, err := Promote(opts, id, "processed", PromoteOpts{Cycle: cycleStr, CommitSHA: oc.CommitSHA})
			if err != nil {
				errs = append(errs, fmt.Errorf("promote %q: %w", id, err))
				continue
			}
			if !pr.NoOp {
				res.Promoted = append(res.Promoted, id)
			}
		}
		rr, err := releaseCycleProcessing(opts, oc.Cycle, oc.Reason, nil)
		res.Released = rr.Paths
		if err != nil {
			errs = append(errs, fmt.Errorf("residual drain: %w", err))
		}
		if len(errs) > 0 {
			return res, fmt.Errorf("apply-cycle-outcome: %w", errors.Join(errs...))
		}
		return res, nil
	}

	// FAIL. Claim first so a committed id that never reached processing/ still
	// gets its failure_count bumped — the exact gap that made the S5 ceiling
	// unreachable for wave lanes.
	if _, err := ClaimLaneScope(opts, oc.Cycle, committed); err != nil {
		return res, fmt.Errorf("apply-cycle-outcome: claim committed ids: %w", err)
	}
	var committedSet map[string]bool
	if len(committed) > 0 {
		committedSet = make(map[string]bool, len(committed))
		for _, id := range committed {
			committedSet[id] = true
		}
	}
	rr, err := releaseCycleProcessing(opts, oc.Cycle, oc.Reason, &quarantinePolicy{
		ceiling:     oc.Ceiling,
		systemLevel: oc.SystemLevel,
		committed:   committedSet,
	})
	for _, p := range rr.Paths {
		if strings.Contains(p, filepath.Join(opts.InboxDir, "quarantine")+string(filepath.Separator)) {
			res.Quarantined = append(res.Quarantined, p)
			continue
		}
		res.Released = append(res.Released, p)
	}
	if err != nil {
		return res, fmt.Errorf("apply-cycle-outcome: failure drain: %w", err)
	}
	return res, nil
}

// ClaimLaneScope moves each resolvable id from the inbox root into
// processing/cycle-<cycle>/ and returns the ids actually claimed. An id it
// cannot resolve — absent, already claimed by another wave, or console-routed
// (ADR-0074) — is logged and skipped: a partial claim must never abort a lane.
// The error return is reserved for a future whole-operation failure and is
// currently always nil, so callers can wire it without a behavior change.
//
// Placement note (deliberate, load-bearing): this is called from
// ApplyCycleOutcome's FAIL path rather than at wave dispatch. Triage builds its
// menu from inboxbatch.LoadDir on the inbox ROOT only (triage.go:113), so
// claiming a lane's scope BEFORE triage runs would hand triage an empty inbox
// and starve the very cycle the claim exists to track. Claiming at outcome time
// puts the worked ids in processing/cycle-N/ exactly when the drain needs them
// there, with no starvation window.
func ClaimLaneScope(opts Options, cycle int, ids []string) ([]string, error) {
	opts.resolveOpts()
	cycleStr := strconv.Itoa(cycle)
	var claimed []string
	for _, id := range dedupeIDs(ids) {
		if _, err := Claim(opts, id, cycleStr); err != nil {
			opts.logf("WARN: ", "claim-lane-scope: '%s' not claimed (%v) — lane continues", id, err)
			continue
		}
		claimed = append(claimed, id)
	}
	return claimed, nil
}

// CommittedIDs walks a triage-decision.json body and returns the union of
// .top_n[].id and .skip_shipped[].task_id, deduped and order-preserving: the
// set of ids the cycle actually committed to working. Both closeout paths key
// their lifecycle transition off this ONE reader so PASS-promote and FAIL-bump
// can never disagree about what "the worked set" means. Invalid JSON returns
// nil (which callers read as "no committed set known").
func CommittedIDs(body []byte) []string {
	var d struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
		SkipShipped []struct {
			TaskID string `json:"task_id"`
		} `json:"skip_shipped"`
	}
	if json.Unmarshal(body, &d) != nil {
		return nil
	}
	out := []string{}
	for _, e := range d.TopN {
		out = append(out, e.ID)
	}
	for _, e := range d.SkipShipped {
		out = append(out, e.TaskID)
	}
	return dedupeIDs(out)
}

// dedupeIDs drops empties and duplicates, preserving first-seen order.
func dedupeIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
