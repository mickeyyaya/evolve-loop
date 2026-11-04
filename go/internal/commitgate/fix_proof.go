package commitgate

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitprefixgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeFixProofMissing signalcenter.Code = "FIX_PROOF_MISSING"
	CodeFixProofUnknown signalcenter.Code = "FIX_PROOF_UNKNOWN"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleFixProof, CodeFixProofMissing, "evolve commit-gate run at regression_proof.stage shadow: the --message subject has the type fix and the message has no valid Fix-Journey line (Fix-Journey: <id> or Fix-Journey: none <reason>); WARN, blocks nothing; fields stage, subject (regression-proof-and-supervision.md §3)")
	signalcenter.RegisterCode(signalcenter.ModuleFixProof, CodeFixProofUnknown, "evolve commit-gate run at regression_proof.stage shadow: the Fix-Journey line of a fix commit names an id that is not an active row of "+journeys.RegistryPath+", or the registry cannot be read; WARN, blocks nothing; fields stage, subject (regression-proof-and-supervision.md §3)")
}

func (o Options) reportFixProof(res *Result) {
	if o.FixProofStage != config.StageShadow || commitprefixgate.SubjectType(o.Message) != "fix" {
		return
	}
	code, reason := o.fixProofFinding()
	if code == "" {
		return
	}
	subject, _, _ := strings.Cut(o.Message, "\n")
	res.log("%s (shadow, blocks nothing): %s", code, reason)
	o.Signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleFixProof, Origin: "commitgate.reportFixProof", Kind: signalcenter.KindFixProofWarning,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: reason,
		Fields: map[string]string{"stage": o.FixProofStage.String(), "subject": subject},
	})
}

func (o Options) fixProofFinding() (signalcenter.Code, string) {
	fj, err := evidence.ParseFixJourney(o.Message)
	switch {
	case err != nil:
		return CodeFixProofMissing, err.Error()
	case !fj.Present:
		return CodeFixProofMissing, "the fix commit has no Fix-Journey line"
	case fj.None():
		return "", ""
	}
	reg, err := journeys.Load(o.RepoRoot)
	if err != nil {
		return CodeFixProofUnknown, "the journey registry cannot be read: " + err.Error()
	}
	if !reg.Known(fj.ID) {
		return CodeFixProofUnknown, "the journey " + fj.ID + " is not an active row of " + journeys.RegistryPath
	}
	return "", ""
}
