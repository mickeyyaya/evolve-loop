package codereview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeRound signalcenter.Code = "REVIEW_ROUND"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleReview, CodeRound, "the shadow decision after a code-review round, computed as if enforced and recorded only, so live routing is unchanged; INFO; fields.decision (resolved, gap_open, ledger, history, or the convergence action), stage, round, finding_mass (all OPEN review rows), gap_mass (sum of threshold minus score), decide_mass (the mass convergence uses for the round at its blocking bar, gap included), blocking_bar, rung, reasons; in shadow every row is read as OPEN in every round, so no row goes from FIXED to OPEN and the reopen counts stay zero; zero oscillation stops is no evidence for that rule")
}

func RoundEvent(req Request, stage string, d RoundDecision) signalcenter.Event {
	return signalcenter.Event{
		Cycle: req.Cycle, Phase: PhaseName, Module: signalcenter.ModuleReview, Origin: "codereview.RoundEvent",
		Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo, Code: CodeRound,
		Reason: fmt.Sprintf("code-review round %d: shadow decision %s (stage %s)", req.Round, d.Decision, stage),
		Fields: map[string]string{
			"decision": d.Decision, "stage": stage, "round": strconv.Itoa(req.Round),
			"finding_mass": formatMass(d.FindingMass), "gap_mass": formatMass(d.GapMass), "decide_mass": formatMass(d.DecideMass),
			"rung": strconv.Itoa(d.Rung), "blocking_bar": d.BlockingBar, "reasons": strings.Join(d.Reasons, "; "),
		},
	}
}

func formatMass(m float64) string { return strconv.FormatFloat(m, 'f', -1, 64) }
