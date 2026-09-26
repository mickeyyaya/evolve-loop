package inboxmover

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
)

// CycleOutcome is one cycle's verdict as the inbox lifecycle consumes it.
type CycleOutcome struct {
	Cycle        int
	Passed       bool     // PASS promotes the committed ids; FAIL bumps and may quarantine them
	CommittedIDs []string // the worked set (top_n + skip_shipped); on FAIL, nil bumps the whole cycle dir
	CommitSHA    string   // PASS only; "" means no SHA prefix
	Reason       string   // ledger reason; "" means the default
	Ceiling      int      // FAIL only; <= 0 disables quarantine
	SystemLevel  bool     // a system-level failure, which never quarantines
	Routed       bool     // FAIL only: the closeout already routed the refused item console-manual, so no bump or park
}

// OutcomeResult reports what the seam moved, by task id or destination path.
type OutcomeResult struct {
	Promoted    []string // committed ids moved to processed/cycle-N/
	Released    []string
	Quarantined []string
}

// ApplyCycleOutcome applies one cycle's verdict to the inbox lifecycle.
// See ADR-0079.
func ApplyCycleOutcome(opts Options, oc CycleOutcome) (OutcomeResult, error) {
	opts.resolveOpts()
	res := OutcomeResult{}
	committed := dedupeIDs(oc.CommittedIDs)

	if oc.Passed {
		cycleStr := strconv.Itoa(oc.Cycle)
		// Promote errors are collected, not returned early: the residual drain
		// below must always run, or claimed items strand in processing/.
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

	// Claim first: the drain bumps only what sits in processing/cycle-N/.
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
		Ceiling:     oc.Ceiling,
		SystemLevel: oc.SystemLevel,
		Routed:      oc.Routed,
		Committed:   committedSet,
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

// ClaimLaneScope claims each resolvable id into processing/cycle-<cycle>/, skipping the rest; the error is always nil.
func ClaimLaneScope(opts Options, cycle int, ids []string) ([]string, error) {
	opts.resolveOpts()
	cycleStr := strconv.Itoa(cycle)
	var claimed []string
	for _, id := range dedupeIDs(ids) {
		// Already claimed by this cycle: re-claiming from the root would raise a false not-found.
		if loc, lerr := lifecycle.Locate(opts.InboxDir, id); lerr == nil && loc.Cycle == cycle {
			claimed = append(claimed, id)
			continue
		}
		if _, err := Claim(opts, id, cycleStr); err != nil {
			opts.logf("WARN: ", "claim-lane-scope: '%s' not claimed (%v) — lane continues", id, err)
			continue
		}
		claimed = append(claimed, id)
	}
	return claimed, nil
}

// CommittedIDs returns the deduped union of a triage decision's top_n ids and skip_shipped task ids, or nil on bad JSON.
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

// DeferredIDs returns the ids triage explicitly deferred, which consumption must never retire.
func DeferredIDs(body []byte) []string {
	var d struct {
		Deferred []struct {
			ID string `json:"id"`
		} `json:"deferred"`
	}
	if json.Unmarshal(body, &d) != nil {
		return nil
	}
	out := []string{}
	for _, e := range d.Deferred {
		out = append(out, e.ID)
	}
	return dedupeIDs(out)
}

// closedDropReasons are the only drop reasons consumption may retire; an unknown
// reason, or "stale", keeps the item pickable because a live todo outranks a stale one.
var closedDropReasons = []string{"already-shipped", "already-done", "already-landed", "duplicate", "superseded", "obsolete"}

// ClosedDroppedIDs returns the ids triage dropped with a close-class reason.
func ClosedDroppedIDs(body []byte) []string {
	// dropped[] has two sibling readers with other policies (carryover's triageDroppedIDs,
	// committedset.DispositionsFrom); a schema change must land in all three.
	var d struct {
		Dropped []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"dropped"`
	}
	if json.Unmarshal(body, &d) != nil {
		return nil
	}
	out := []string{}
	for _, e := range d.Dropped {
		tag := dropReasonTag(e.Reason)
		for _, tok := range closedDropReasons {
			if strings.HasPrefix(tag, tok) {
				out = append(out, e.ID)
				break
			}
		}
	}
	return dedupeIDs(out)
}

// dropReasonTag returns the lowercased text before a reason's first ':'. A reason
// is classified by what it leads with, so "stale: superseded by X" stays stale.
func dropReasonTag(reason string) string {
	if i := strings.IndexByte(reason, ':'); i >= 0 {
		reason = reason[:i]
	}
	return strings.ToLower(strings.TrimSpace(reason))
}

// dedupeIDs drops empties and duplicates, preserving first-seen order; it returns nil when none remain.
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
