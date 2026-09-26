package router

import "github.com/mickeyyaya/evolve-loop/go/internal/phaseio"

// AssembleHandoffs digests workspace and projects the result into phaseio.Handoffs. It lives here
// so phaseio stays a leaf and Digest stays the only on-disk reader.
func AssembleHandoffs(workspace string, completed []string) (phaseio.Handoffs, error) {
	sig, err := Digest(workspace, completed)
	if err != nil {
		return phaseio.Handoffs{}, err
	}
	return HandoffsFromSignals(sig), nil
}

// HandoffsFromSignals projects an existing digest into phaseio views without re-reading disk;
// severities become their canonical words.
func HandoffsFromSignals(sig RoutingSignals) phaseio.Handoffs {
	init := phaseio.HandoffsInit{Generic: sig.Generic, Degraded: sig.DigestDegraded}
	if sig.Scout.Present {
		init.Scout = &phaseio.ScoutView{
			CycleSizeEstimate: sig.Scout.CycleSizeEstimate,
			GoalType:          sig.Scout.GoalType,
			DeliverableKind:   sig.Scout.DeliverableKind,
			ItemCount:         sig.Scout.ItemCount,
			CarryoverCount:    sig.Scout.CarryoverCount,
			BacklogSize:       sig.Scout.BacklogSize,
		}
	}
	if sig.Triage.Present {
		init.Triage = &phaseio.TriageView{
			CycleSize:       sig.Triage.CycleSize,
			PhaseSkip:       sig.Triage.PhaseSkip,
			DeliverableKind: sig.Triage.DeliverableKind,
		}
	}
	if sig.Build.Present {
		init.Build = &phaseio.BuildView{
			Verdict:       sig.Build.Verdict,
			ACSGreen:      sig.Build.ACSGreen,
			ACSRed:        sig.Build.ACSRed,
			ACSTotal:      sig.Build.ACSTotal,
			ACSThisCycle:  sig.Build.ACSThisCycle,
			ACSRegression: sig.Build.ACSRegression,
			SeverityMax:   sig.Build.SeverityMax.String(),
			FilesTouched:  sig.Build.FilesTouched,
			DiffLOC:       sig.Build.DiffLOC,
		}
	}
	if sig.Audit.Present {
		defects := make(map[string]int, len(sig.Audit.DefectsBySeverity))
		for sev, n := range sig.Audit.DefectsBySeverity {
			defects[sev.String()] = n
		}
		init.Audit = &phaseio.AuditView{
			Verdict:           sig.Audit.Verdict,
			Confidence:        sig.Audit.Confidence,
			RedCount:          sig.Audit.RedCount,
			DefectsBySeverity: defects,
		}
	}
	return phaseio.NewHandoffs(init)
}
