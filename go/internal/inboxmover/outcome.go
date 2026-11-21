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
	OnCeiling    string
	SystemLevel  bool
	Routed       bool
	RoutedIDs    []string
	FailureClass string
	Fingerprint  string
	FailureRung  string
	Reasons      []string
}

type OutcomeResult struct {
	Promoted        []string
	RetiredUnbacked []string
	Released        []string
	Quarantined     []string
}

const shipUnbackedReason = "ship-promote-processed: no inbox item backs the id"

func ApplyCycleOutcome(opts Options, oc CycleOutcome) (OutcomeResult, error) {
	opts, err := opts.resolved()
	if err != nil {
		return OutcomeResult{}, err
	}
	res := OutcomeResult{}
	committed := dedupeIDs(oc.CommittedIDs)

	if oc.Passed {
		return applyPassedOutcome(opts, oc, committed)
	}

	ClaimLaneScope(opts, oc.Cycle, committed)
	rr, err := releaseCycleProcessing(opts, oc.Cycle, oc.Reason, failurePolicy(oc, committed))
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

func applyPassedOutcome(opts Options, oc CycleOutcome, committed []string) (OutcomeResult, error) {
	res := OutcomeResult{}
	cycleStr := strconv.Itoa(oc.Cycle)
	var errs []error
	var notMoved []string
	for _, id := range committed {
		if oc.Cycle > 0 {
			if loc, lerr := lifecycle.Locate(opts.InboxDir, id); lerr == nil && loc.Cycle != 0 && loc.Cycle != oc.Cycle {
				opts.logf("WARN: ", "apply-cycle-outcome: refused %q for cycle %d (held by cycle %d)", id, oc.Cycle, loc.Cycle)
				continue
			}
		}
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

func FilterPromotionScope(opts Options, cid int, pinIDs, ids []string) (allowed, warnings []string) {
	if len(ids) == 0 {
		return nil, nil
	}
	opts.resolveOpts()
	pinned := len(pinIDs) > 0
	pinSet := idSet(pinIDs)
	for _, id := range ids {
		loc, err := lifecycle.Locate(opts.InboxDir, id)
		if err == nil && loc.Cycle != 0 && loc.Cycle != cid {
			warnings = append(warnings, fmt.Sprintf("refused inbox promotion of %q for cycle %d: held by cycle %d", id, cid, loc.Cycle))
			continue
		}
		if pinned && !pinSet[id] && (err != nil || loc.Cycle != cid) {
			warnings = append(warnings, fmt.Sprintf("refused inbox promotion of %q for cycle %d: outside lane-scope pin", id, cid))
			continue
		}
		allowed = append(allowed, id)
	}
	return allowed, warnings
}

func failurePolicy(oc CycleOutcome, committed []string) *quarantinePolicy {
	return &quarantinePolicy{
		Ceiling:      oc.Ceiling,
		OnCeiling:    oc.OnCeiling,
		SystemLevel:  oc.SystemLevel,
		Routed:       oc.Routed,
		RoutedIDs:    idSet(oc.RoutedIDs),
		Committed:    idSet(committed),
		FailureClass: oc.FailureClass,
		Fingerprint:  oc.Fingerprint,
		FailureRung:  oc.FailureRung,
		Reasons:      oc.Reasons,
	}
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

const (
	AlreadyShippedTag   = "already-shipped"
	ProtectedSurfaceTag = "protected-surface"
)

var closedDropReasons = []string{AlreadyShippedTag, "already-done", "already-landed", "duplicate", "superseded", "obsolete"}

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
	tag, _ := DropReasonToken(reason)
	return tag
}

func DropReasonToken(reason string) (tag, token string) {
	tag = reason
	if i := strings.IndexByte(reason, ':'); i >= 0 {
		tag = reason[:i]
		if words := strings.Fields(reason[i+1:]); len(words) > 0 {
			token = words[0]
		}
	}
	return strings.ToLower(strings.TrimSpace(tag)), token
}

func LandedClaims(body []byte) map[string]string {
	var d struct {
		committedset.DroppedBucket
		committedset.SkipShippedBucket
	}
	if json.Unmarshal(body, &d) != nil {
		return nil
	}
	claims := map[string]string{}
	for _, e := range d.Dropped {
		if tag, token := DropReasonToken(e.Reason); tag == AlreadyShippedTag && token != "" {
			claims[e.ID] = token
		}
	}
	for _, e := range d.SkipShipped {
		if sha := strings.TrimSpace(e.GitSHA); sha != "" {
			claims[e.TaskID] = sha
		}
	}
	return claims
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
