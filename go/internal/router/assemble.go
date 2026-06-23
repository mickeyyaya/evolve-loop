package router

import "github.com/mickeyyaya/evolveloop/go/internal/phaseio"

// AssembleHandoffs reads the completed phases' handoff artifacts (via Digest)
// and projects the resulting RoutingSignals into a dependency-free
// phaseio.Handoffs — the typed Upstream view a phase consumes (ADR-0050
// Phase 3.3). It is the single router→phaseio bridge: it lives in router
// (which already owns Digest + RoutingSignals and may import the phaseio leaf,
// router→phaseio→phasespec→config, no cycle) precisely so phaseio stays a pure
// leaf with no router import. Built on Digest so there is one on-disk-shape
// authority, never a second reader.
func AssembleHandoffs(workspace string, completed []string) (phaseio.Handoffs, error) {
	sig, err := Digest(workspace, completed)
	if err != nil {
		return phaseio.Handoffs{}, err
	}
	return HandoffsFromSignals(sig), nil
}

// HandoffsFromSignals maps an already-computed RoutingSignals digest into the
// typed phaseio views, without re-reading disk. Exported so the orchestrator's
// EVOLVE_PHASE_IO shadow stage (Phase 3.4) can Digest once and project, rather
// than calling AssembleHandoffs (a second Digest read) alongside its own Digest.
// The only non-trivial conversion is severity: RoutingSignals encodes it as the
// ordinal router.Severity, while the dependency-free phaseio views use the
// canonical severity word (Severity.String()).
func HandoffsFromSignals(sig RoutingSignals) phaseio.Handoffs {
	init := phaseio.HandoffsInit{Generic: sig.Generic, Degraded: sig.DigestDegraded}
	if sig.Scout.Present {
		init.Scout = &phaseio.ScoutView{
			CycleSizeEstimate: sig.Scout.CycleSizeEstimate,
			ItemCount:         sig.Scout.ItemCount,
			CarryoverCount:    sig.Scout.CarryoverCount,
			BacklogSize:       sig.Scout.BacklogSize,
		}
	}
	if sig.Triage.Present {
		init.Triage = &phaseio.TriageView{
			CycleSize: sig.Triage.CycleSize,
			PhaseSkip: sig.Triage.PhaseSkip,
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
