package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

type JudgeCall struct {
	Input  ReviewInput
	Pair   string
	Author handoff.Author
	Tier   string
	Asks   []string
	Ledger handoff.Ledger
}

type JudgeResult struct {
	Seat     handoff.Seat
	Verdicts []handoff.Verdict
	Basis    handoff.Basis
}

type HandoffJudgeRunner func(ctx context.Context, call JudgeCall) (JudgeResult, error)

func (r HandoffReviewer) WithHandoffJudge(run HandoffJudgeRunner) HandoffReviewer {
	r.judgeRun = run
	return r
}

func (r HandoffReviewer) JudgeWired() bool { return r.judgeRun != nil }

func (o *Orchestrator) HandoffJudgeWired() bool {
	return reviewerChainHas(o.reviewer, func(r DeliverableReviewer) bool {
		hr, ok := r.(HandoffReviewer)
		return ok && hr.JudgeWired()
	})
}

func (r HandoffReviewer) judge(ctx context.Context, in ReviewInput, pair string, check handoff.Check) (*handoff.Judgement, []string) {
	tier := r.config[pair].JudgeTier
	if !r.JudgeWired() || tier == "" || check.Outcome != string(handoff.ExitPass) {
		return nil, nil
	}
	ledger, ok := readJudgedLedger(in.Workspace, pair)
	if !ok {
		return nil, nil
	}
	asks := ledger.MetAsks()
	if len(asks) == 0 {
		return nil, nil
	}
	result, err := r.judgeRun(ctx, JudgeCall{Input: in, Pair: pair, Author: ledger.Author, Tier: tier, Asks: asks, Ledger: ledger})
	return judgementOf(result, err), asks
}

func judgementOf(result JudgeResult, err error) *handoff.Judgement {
	judgement := &handoff.Judgement{Seat: result.Seat, Basis: result.Basis}
	switch {
	case errors.Is(err, llmroute.ErrNoPermittedAttempt):
		judgement.Refused = err.Error()
	case err != nil:
		judgement.Error = err.Error()
	default:
		judgement.Verdicts = result.Verdicts
	}
	return judgement
}

func readJudgedLedger(workspace, pair string) (handoff.Ledger, bool) {
	raw, err := ReadHandoffLedger(handoffLedgerPath(workspace, pair))
	if err != nil || handoff.IsCheckOnly(raw) {
		return handoff.Ledger{}, false
	}
	ledger, err := handoff.Parse(raw)
	return ledger, err == nil
}

func judgeRejection(pair string, judgement *handoff.Judgement, asks []string) ReviewResult {
	if judgement == nil || judgement.Refused != "" || judgement.Error != "" || judgement.Agrees(asks) {
		return ReviewResult{Approve: true}
	}
	disagreed := judgement.Disagreements(asks)
	ids := make([]string, len(disagreed))
	for i, line := range disagreed {
		ids[i], _, _ = strings.Cut(line, ":")
	}
	var remedy strings.Builder
	fmt.Fprintf(&remedy, "The independent judge does not accept the evidence of asks %s in handoff-%s.json. Meet the meaning of each ask, then raise `round`.", strings.Join(ids, ", "), pair)
	for _, line := range disagreed {
		fmt.Fprintf(&remedy, "\n- %s", line)
	}
	return ReviewResult{Retry: true, Reason: fmt.Sprintf("handoff %s: judge disagreed; asks: %s", pair, strings.Join(ids, ", ")), Remediation: remedy.String()}
}
