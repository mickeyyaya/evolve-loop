package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type HandoffChecker func(in ReviewInput, pair string) handoff.Check

type HandoffReviewer struct {
	config policy.HandoffConfig
	check  HandoffChecker
}

var errHandoffLedgerNotObject = errors.New("the ledger is not a JSON object, so it is kept as written")

var handoffPairEndingAt = map[string]string{
	string(PhaseTDD):   request.PairTriageTDD,
	string(PhaseBuild): request.PairTDDBuild,
}

func NewHandoffReviewer(config policy.HandoffConfig, check HandoffChecker) HandoffReviewer {
	return HandoffReviewer{config: config, check: check}
}

func (r HandoffReviewer) Review(_ context.Context, in ReviewInput) ReviewResult {
	pair, ok := handoffPairEndingAt[in.Phase]
	if !ok {
		return ReviewResult{Approve: true}
	}
	check := r.check(in, pair)
	check.Stage = r.config[pair].Stage
	if err := recordHandoffCheck(in.Workspace, pair, check); err != nil {
		fmt.Fprintf(in.errOut(), "[orchestrator] WARN handoff %s: the check record was not written: %v\n", pair, err)
	}
	if check.Stage != config.StageEnforce.String() || check.Outcome == string(handoff.ExitPass) {
		return ReviewResult{Approve: true}
	}
	return enforcedHandoffRejection(in.Phase, pair, check)
}

func enforcedHandoffRejection(phase, pair string, check handoff.Check) ReviewResult {
	if check.Outcome == string(handoff.ExitBlocked) {
		return ReviewResult{
			Reason:      fmt.Sprintf("handoff %s: blocked: asks %s", pair, strings.Join(check.Blocked, ", ")),
			Remediation: fmt.Sprintf("The check of pair %s proved that asks %s cannot be met (proof: %s). Name each conflict for the console in handoff-%s.json.", pair, strings.Join(check.Blocked, ", "), strings.Join(check.Paths, ", "), pair),
			ProvenBlocker: fmt.Sprintf("blocked: needs console — phase %s stopped on the handoff check of pair %s: the request cannot be met for asks %s (proof: %s)",
				phase, pair, strings.Join(check.Blocked, ", "), strings.Join(check.Paths, ", ")),
		}
	}
	reason := fmt.Sprintf("handoff %s: %s; unmet asks: %s", pair, check.Outcome, strings.Join(check.Unmet, ", "))
	var remedy strings.Builder
	fmt.Fprintf(&remedy, "Open the handoff-asks entry of inputs/index.json. Meet each unmet ask of the %s request, by its ask id, in handoff-%s.json, with evidence that resolves: %s.", pair, pair, strings.Join(check.Unmet, ", "))
	for _, line := range slices.Concat(check.FailedEvidence, check.Errors) {
		fmt.Fprintf(&remedy, "\n- %s", line)
	}
	return ReviewResult{Retry: true, Reason: reason, Remediation: remedy.String()}
}

func recordHandoffCheck(workspace, pair string, check handoff.Check) error {
	path := filepath.Join(workspace, "handoff-"+pair+".json")
	doc := map[string]any{}
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		doc["schema_version"] = handoff.SchemaVersion
		doc["pair"] = pair
	case err != nil:
		return err
	default:
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return errHandoffLedgerNotObject
		}
		for key, value := range fields {
			doc[key] = value
		}
	}
	doc["check"] = check
	return atomicwrite.JSON(path, doc)
}

func (o *Orchestrator) HandoffReviewerWired() bool {
	return reviewerChainHas(o.reviewer, func(r DeliverableReviewer) bool {
		_, ok := r.(HandoffReviewer)
		return ok
	})
}
