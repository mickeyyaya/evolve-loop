package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/plainfile"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type HandoffChecker func(in ReviewInput, pair string) handoff.Check

type HandoffReviewer struct {
	config policy.HandoffConfig
	check  HandoffChecker
	tree   func(ReviewInput) string

	judgeRun HandoffJudgeRunner
}

var errHandoffLedgerNotObject = errors.New("the ledger is not a JSON object, so it is kept as written")

var errHandoffLedgerNotRegular = plainfile.ErrNotPlain

const maxHandoffLedgerBytes = 4 << 20

var errHandoffLedgerTooLarge = plainfile.ErrTooLarge

func ReadHandoffLedger(path string) ([]byte, error) {
	return plainfile.Read(path, maxHandoffLedgerBytes)
}

var handoffPairEndingAt = map[string]string{
	string(PhaseTDD):   request.PairTriageTDD,
	string(PhaseBuild): request.PairTDDBuild,
}

func NewHandoffReviewer(config policy.HandoffConfig, check HandoffChecker) HandoffReviewer {
	return HandoffReviewer{config: config, check: check}
}

const handoffOutcomeInvalid = "invalid"

func (r HandoffReviewer) outputTree(ctx context.Context, in ReviewInput) string {
	if r.tree != nil {
		return r.tree(in)
	}
	return worktreeContentSHA(ctx, in.ProjectRoot, in.Worktree, in.Workspace)
}

func staleOutputCheck(check handoff.Check, stale string) handoff.Check {
	if stale == "" {
		return check
	}
	check.Outcome = handoffOutcomeInvalid
	check.Errors = append(slices.Clone(check.Errors), fmt.Sprintf("output_digest %s is stale: the ledger names an earlier output than the current tree", stale))
	return check
}

func (r HandoffReviewer) Review(ctx context.Context, in ReviewInput) ReviewResult {
	tree := ""
	if _, paired := handoffLedgerPairOf[in.Phase]; paired {
		tree = r.outputTree(ctx, in)
	}
	stale := stampHandoffAuthor(in, tree)
	pair, ok := handoffPairEndingAt[in.Phase]
	if !ok {
		return ReviewResult{Approve: true}
	}
	check := staleOutputCheck(r.check(in, pair), stale)
	check.Stage = r.config[pair].Stage
	in.recordHandoff(pair, check, nil)
	judgement, asks := r.judge(ctx, in, pair, check)
	if judgement != nil {
		in.recordHandoff(pair, check, judgement)
	}
	switch {
	case check.Stage != config.StageEnforce.String():
		return ReviewResult{Approve: true}
	case check.Outcome != string(handoff.ExitPass):
		return enforcedHandoffRejection(in.Phase, pair, check)
	}
	return judgeRejection(pair, judgement, asks)
}

func (in ReviewInput) recordHandoff(pair string, check handoff.Check, judgement *handoff.Judgement) {
	if err := recordHandoffCheck(in.Workspace, pair, check, judgement); err != nil {
		fmt.Fprintf(in.errOut(), "[orchestrator] WARN handoff %s: the check record was not written: %v\n", pair, err)
	}
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

func handoffLedgerPath(workspace, pair string) string {
	return filepath.Join(workspace, "handoff-"+pair+".json")
}

func recordHandoffCheck(workspace, pair string, check handoff.Check, judgement *handoff.Judgement) error {
	path := handoffLedgerPath(workspace, pair)
	doc := map[string]any{}
	raw, err := ReadHandoffLedger(path)
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
	delete(doc, "judge")
	if judgement != nil {
		doc["judge"] = judgement
	}
	return atomicwrite.JSON(path, doc)
}

func (o *Orchestrator) HandoffReviewerWired() bool {
	return reviewerChainHas(o.reviewer, func(r DeliverableReviewer) bool {
		_, ok := r.(HandoffReviewer)
		return ok
	})
}
