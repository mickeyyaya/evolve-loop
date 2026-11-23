package codereview

import (
	"maps"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
)

const (
	DecisionLedger   = "ledger"
	DecisionHistory  = "history"
	DecisionResolved = "resolved"
	DecisionGapOpen  = "gap_open"
)

type RoundInput struct {
	Round     int
	Rows      []defectledger.Entry
	Snapshots []cyclestate.RoundSnapshot
	LedgerErr error
	Config    policy.ConvergenceConfig
}

type RoundDecision struct {
	Decision    string
	FindingMass float64
	GapMass     float64
	DecideMass  float64
	Rung        int
	BlockingBar string
	Reasons     []string
}

func GapMass(scores qualityindex.Scores, thresholds qualityindex.Thresholds) float64 {
	_, gaps := qualityindex.Qualifies(scores, thresholds)
	total := 0.0
	for _, g := range gaps {
		total += float64(g.Threshold - g.Score)
	}
	return total
}

func TakeSnapshot(scores qualityindex.Scores, thresholds qualityindex.Thresholds, rows []defectledger.Entry) cyclestate.RoundSnapshot {
	statuses := map[string]string{}
	for _, r := range rows {
		if r.Source != PhaseName {
			continue
		}
		statuses[r.ID] = statusOf(r)
	}
	return cyclestate.RoundSnapshot{Observed: true, GapMass: GapMass(scores, thresholds), Statuses: statuses}
}

func statusOf(r defectledger.Entry) string {
	if r.Reason == ShadowReason {
		return defectledger.StatusOpen
	}
	return r.Status
}

func DecideAfterReview(in RoundInput) RoundDecision {
	if in.LedgerErr != nil {
		return RoundDecision{Decision: DecisionLedger}
	}
	if in.Round < 1 || len(in.Snapshots) != in.Round || slices.ContainsFunc(in.Snapshots, unobserved) {
		return RoundDecision{Decision: DecisionHistory}
	}
	rounds := judgments(severityByID(in.Rows), in.Snapshots)
	gap := in.Snapshots[in.Round-1].GapMass
	out := RoundDecision{FindingMass: convergence.FindingMass(openOf(rounds[in.Round-1].Findings)), GapMass: gap}
	if out.FindingMass == 0 && gap == 0 {
		out.Decision = DecisionResolved
		return out
	}
	input := convergence.Input{
		Loop: convergence.LoopCodeReview, Round: in.Round - 1, Rounds: rounds, Config: in.Config,
		Mass: func(k int, atBar []convergence.Finding) float64 {
			return convergence.FindingMass(atBar) + in.Snapshots[k].GapMass
		},
	}
	d := convergence.Decide(input)
	out.Decision, out.Rung, out.Reasons, out.BlockingBar = string(d.Action), d.Rung, d.Reasons, string(d.BlockingBar)
	out.DecideMass = convergence.MassAt(input, in.Round-1, d.BlockingBar)
	if d.Action == convergence.ActionLand && gap > 0 {
		out.Decision = DecisionGapOpen
	}
	return out
}

func unobserved(s cyclestate.RoundSnapshot) bool { return !s.Observed }

func severityByID(rows []defectledger.Entry) map[string]convergence.Severity {
	out := map[string]convergence.Severity{}
	for _, r := range rows {
		if r.Source == PhaseName {
			out[r.ID] = convergence.Severity(r.Severity)
		}
	}
	return out
}

func openOf(findings []convergence.Finding) []convergence.Finding {
	var open []convergence.Finding
	for _, f := range findings {
		if f.Status == convergence.StatusOpen {
			open = append(open, f)
		}
	}
	return open
}

func judgments(severities map[string]convergence.Severity, snapshots []cyclestate.RoundSnapshot) []convergence.Judgment {
	out := make([]convergence.Judgment, len(snapshots))
	for k, snap := range snapshots {
		out[k] = convergence.Judgment{Index: k, Findings: findingsOf(severities, snap)}
	}
	return out
}

func findingsOf(severities map[string]convergence.Severity, snap cyclestate.RoundSnapshot) []convergence.Finding {
	var out []convergence.Finding
	for _, id := range slices.Sorted(maps.Keys(snap.Statuses)) {
		out = append(out, convergence.Finding{ID: id, Severity: severities[id], Status: convergence.Status(snap.Statuses[id])})
	}
	return out
}
