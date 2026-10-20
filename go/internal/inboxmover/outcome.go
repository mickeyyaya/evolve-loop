package inboxmover

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
)

type CycleOutcome struct {
	Cycle        int
	Passed       bool
	CommittedIDs []string
	CommitSHA    string
	Reason       string
	Ceiling      int
	SystemLevel  bool
	Routed       bool
	RoutedIDs    []string
	FailureClass string
	Fingerprint  string
}

type OutcomeResult struct {
	Promoted        []string
	RetiredUnbacked []string
	Released        []string
	Quarantined     []string
}

const shipUnbackedReason = "ship-promote-processed: no inbox item backs the id"

func ApplyCycleOutcome(opts Options, oc CycleOutcome) (OutcomeResult, error) {
	opts.resolveOpts()
	res := OutcomeResult{}
	committed := dedupeIDs(oc.CommittedIDs)

	if oc.Passed {
		cycleStr := strconv.Itoa(oc.Cycle)
		var errs []error
		var notMoved []string
		for _, id := range committed {
			pr, err := Promote(opts, id, "processed", PromoteOpts{Cycle: cycleStr, CommitSHA: oc.CommitSHA})
			if err != nil {
				errs = append(errs, fmt.Errorf("promote %q: %w", id, err))
				continue
			}
			if pr.NoOp {
				notMoved = append(notMoved, id)
				continue
			}
			res.Promoted = append(res.Promoted, id)
		}
		retired, err := RetireUnbacked(opts, oc.Cycle, StateProcessed, shipUnbackedReason, oc.CommitSHA, notMoved)
		res.RetiredUnbacked = retired
		if err != nil {
			errs = append(errs, err)
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

	ClaimLaneScope(opts, oc.Cycle, committed)
	rr, err := releaseCycleProcessing(opts, oc.Cycle, oc.Reason, &quarantinePolicy{
		Ceiling:      oc.Ceiling,
		SystemLevel:  oc.SystemLevel,
		Routed:       oc.Routed,
		RoutedIDs:    idSet(oc.RoutedIDs),
		Committed:    idSet(committed),
		FailureClass: oc.FailureClass,
		Fingerprint:  oc.Fingerprint,
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

func ClaimLaneScope(opts Options, cycle int, ids []string) []string {
	opts.resolveOpts()
	cycleStr := strconv.Itoa(cycle)
	var claimed []string
	for _, id := range dedupeIDs(ids) {
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
	return claimed
}

func CommittedIDs(body []byte) []string {
	var d struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
		committedset.SkipShippedBucket
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

var closedDropReasons = []string{"already-shipped", "already-done", "already-landed", "duplicate", "superseded", "obsolete"}

func ClosedDroppedIDs(body []byte) []string {
	var d committedset.DroppedBucket
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

func dropReasonTag(reason string) string {
	if i := strings.IndexByte(reason, ':'); i >= 0 {
		reason = reason[:i]
	}
	return strings.ToLower(strings.TrimSpace(reason))
}

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

func idSet(ids []string) map[string]bool {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
