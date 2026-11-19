package runner

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner/verdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeSelfReviewMissing signalcenter.Code = "RUNNER_SELF_REVIEW_MISSING"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleRunner, CodeSelfReviewMissing, "a source-writing dispatch loaded code-review-simplify and handed off an accepted report without a Self-Review section. Advisory only; verdict stays unchanged. fields.skill, section")
}

func (b *BaseRunner) warnSelfReviewMissing(d verdict.Dispatch) {
	if b.signals == nil {
		return
	}
	b.signals().Emit(signalcenter.Event{
		Cycle: d.Cycle, RunID: d.RunID, Phase: d.Phase,
		Module: signalcenter.ModuleRunner, Origin: "BaseRunner.warnSelfReviewMissing", Kind: signalcenter.KindRunnerWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeSelfReviewMissing,
		Reason: "the " + d.Phase + " report carries no " + phasecontract.SelfReview.Canonical + " section although the dispatch loaded " + policy.SelfReviewSkill,
		Fields: map[string]string{"skill": policy.SelfReviewSkill, "section": phasecontract.SelfReview.Canonical},
	})
}
