package audit

import (
	"context"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/site"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	ledgerGate         = "audit chain ledgers"
	ledgerStagePair    = "build-audit"
	ledgerSkippedNote  = "tdd did not run"
	ledgerNoSetNote    = "no committed set"
	ledgerStageWarning = "handoff stage unreadable: the build-audit gate is in shadow"
)

func (h hooks) outputStale(req core.PhaseRequest, ledgerPath string) string {
	if h.buildStale != nil {
		return h.buildStale(req, ledgerPath)
	}
	return core.OutputDigestStale(context.Background(), req.ProjectRoot, req.Worktree, req.Workspace, ledgerPath)
}

func (a *auditClassification) ledgerInputs() auditchain.LedgerInputs {
	var in auditchain.LedgerInputs
	for _, pair := range []string{request.PairTriageTDD, request.PairTDDBuild} {
		s := site.ForPhase(a.req.Workspace, a.req.ProjectRoot, a.req.Worktree)
		if pair == request.PairTDDBuild {
			s.Stale = a.hooks.outputStale(a.req, filepath.Join(a.req.Workspace, "handoff-"+pair+".json"))
		}
		ev := site.Evaluate(s, a.req.Cycle, pair)
		in.Pairs = append(in.Pairs, auditchain.PairEval{
			Pair: ev.Pair, RequestAbsent: ev.RequestAbsent, LedgerAbsent: ev.LedgerAbsent, Outcome: ev.Outcome,
			Unmet: ev.Unmet, Blocked: ev.Blocked, Errors: ev.Errors, Request: ev.Request, Ledger: ev.Ledger,
		})
	}
	return in
}

func (a *auditClassification) walkLedgers() {
	in := a.ledgerInputs()
	a.ledgerFindings = auditchain.WalkLedgers(in)
	switch {
	case auditchain.TDDSkipped(in):
		a.ledgerNote = ledgerSkippedNote
	case auditchain.TriageSkipped(in):
		a.ledgerNote = ledgerNoSetNote
	}
	stage, err := policy.HandoffStageFor(a.req.ProjectRoot, ledgerStagePair)
	if err != nil {
		a.warn(ledgerStageWarning)
	}
	if stage != config.StageEnforce.String() || len(a.ledgerFindings) == 0 {
		return
	}
	for _, f := range a.ledgerFindings {
		a.diagnostics = append(a.diagnostics, core.Diagnostic{Severity: "error", Message: f.Message})
	}
	a.override(ledgerGate)
}
