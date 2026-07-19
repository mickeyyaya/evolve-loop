package core

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// errorSeverityMessages extracts the error-severity diagnostic messages — the
// single definition shared by floorVerdictError (the FailedRecord reason) and
// persistFloorFailReasons (the coherence-floor + forensic signal), so the two
// can never drift on what counts as "explained".
func errorSeverityMessages(diags []Diagnostic) []string {
	var msgs []string
	for _, d := range diags {
		if d.Severity == "error" {
			msgs = append(msgs, d.Message)
		}
	}
	return msgs
}

// floorFailReason is the FORENSIC workspace artifact recording WHY a floor
// phase's verdict was recorded FAIL when the phase's own report said otherwise
// — the error-severity diagnostics of the runner-side downgrade (audit's
// CI-parity gates: integration tier, go vet, apicover, EGPS…). state.json
// truncates the defect string; this file is the untruncated on-disk "why" for
// retros/operators (one grep instead of a forensic dig).
//
// TRUST BOUNDARY (go-review HIGH): the workspace is agent-writable — the audit
// agent already writes audit-report.md there — so this file is NEVER read by
// the ADR-0072 coherence floor. The floor's authoritative signal is
// CycleState.AuditFailReasons, set in orchestrator memory at the same
// chokepoint; a file dropped by any workspace writer cannot talk the floor out
// of halting.
type floorFailReason struct {
	SchemaVersion int      `json:"schema_version"`
	Phase         string   `json:"phase"`
	Reasons       []string `json:"reasons"`
}

func floorFailReasonPath(workspace string, phase Phase) string {
	return filepath.Join(workspace, string(phase)+"-fail-reason.json")
}

// persistFloorFailReasons records the downgrade reasons behind a floor phase's
// FAIL verdict at the verdict-record chokepoint (recordFloorVerdictFailure):
//   - authoritative: CycleState.AuditFailReasons (orchestrator memory; the
//     coherence floor's only source) — CLOBBERED on every call, so a superseding
//     attempt with no error-severity diagnostics erases a prior attempt's
//     explanation rather than letting it ratchet (go-review MEDIUM-HIGH);
//   - forensic: <workspace>/<phase>-fail-reason.json, written/removed in
//     lockstep (best-effort; a write failure only costs observability).
//
// Warning-only diagnostics explain nothing (fail-open gate skips), so they
// clear both carriers — an empty explanation must never suppress the
// forged-verdict halt.
func persistFloorFailReasons(cs *CycleState, phase Phase, diags []Diagnostic) {
	if cs == nil {
		return
	}
	reasons := errorSeverityMessages(diags)
	if phase == PhaseAudit {
		cs.AuditFailReasons = reasons
	}
	if cs.WorkspacePath == "" {
		return
	}
	path := floorFailReasonPath(cs.WorkspacePath, phase)
	if len(reasons) == 0 {
		_ = os.Remove(path)
		return
	}
	b, err := json.MarshalIndent(floorFailReason{SchemaVersion: 1, Phase: string(phase), Reasons: reasons}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

// resetFloorFailReason clears a phase's recorded downgrade explanation — called
// at every dispatch of the phase, so a re-dispatch (ship-error recovery
// re-audit, debugger RERUN_PHASE) can never inherit a stale explanation from a
// superseded attempt: an attempt that ends via the error path never reaches the
// record chokepoint, and without this clear the prior attempt's reasons would
// incorrectly mark a differently-caused, unexplained FAIL as diagnosed.
func resetFloorFailReason(cs *CycleState, phase Phase) {
	if cs == nil {
		return
	}
	if phase == PhaseAudit {
		cs.AuditFailReasons = nil
	}
	if cs.WorkspacePath != "" {
		_ = os.Remove(floorFailReasonPath(cs.WorkspacePath, phase))
	}
}

// readFloorFailReasons reads the FORENSIC reason file (retro/operator/test
// tooling). Deliberately NOT consulted by detectVerdictIncoherence — see the
// trust boundary on floorFailReason.
func readFloorFailReasons(workspace string, phase Phase) []string {
	b, err := os.ReadFile(floorFailReasonPath(workspace, phase))
	if err != nil {
		return nil
	}
	var r floorFailReason
	if json.Unmarshal(b, &r) != nil {
		return nil
	}
	return r.Reasons
}

// detectVerdictIncoherence is the ADR-0072 Go floor for the verdict-incoherence
// category: the deterministic, non-negotiable check that catches a pipeline
// forging a verdict. It reads the cycle's own on-disk phase artifacts
// (audit-report evolve-verdict + acs-verdict.json) and, if the recorded verdict
// is FAIL/WARN while both artifacts are green, returns a system-failure signal.
//
// This floor fires regardless of orchestrator judgment or strict_audit: a
// broken pipeline cannot be talked out of halting. It is gated on the
// failure_policy IsFloor(verdict-incoherence) predicate, so an operator can
// never narrow it below the compiled floor.
//
// Scope (deliberate for the deterministic slice): it fires ONLY on a recorded
// FAIL/WARN with green artifacts — the exact cycle 862→899 forgery signature.
// A RED artifact is coherent (a genuine task-code failure → never-stop). The
// "silent no-ship" (CycleOutcomeSkippedUnknown) case is intentionally NOT
// hard-halted here — a benign no-op cycle can also produce it, so its
// disambiguation is left to the orchestrator's judgment layer. The other floor
// category, infra-systemic (all CLI families exhausted), is enforced by the
// pre-existing resumable quota-pause path (cmd_loop) — NOT this function; the
// two floor categories have distinct, deliberate detection sites.
//
// A DIAGNOSED downgrade is NOT forgery: cs.AuditFailReasons carries the
// error-severity gate diagnostics behind the FAIL, set in ORCHESTRATOR MEMORY
// at the verdict-record chokepoint (recordFloorVerdictFailure) and cleared on
// every audit re-dispatch — the runner's own CI-parity gate overrode a
// narrative PASS, a coherent task-level outcome that routes retro→continue.
// Halting on it was the cycles-930/931/932 batch-killer: the whole loop stopped
// for a flaky integration tier while the diagnostics naming the cause sat in
// the response. The signal is deliberately NOT the workspace reason file
// (agent-writable — see floorFailReason's trust boundary): only the
// orchestrator's own in-process record can mark a FAIL as explained.
func (o *Orchestrator) detectVerdictIncoherence(cs CycleState, finalVerdict string) *SystemFailureSignal {
	audit, acs, auditRan := coherence.ReadCycleVerdicts(cs.WorkspacePath)
	coh := coherence.CheckVerdictCoherence(coherence.VerdictInputs{
		Recorded:         finalVerdict,
		Audit:            audit,
		ACS:              acs,
		AuditRan:         auditRan,
		SubstantiveError: len(cs.AuditFailReasons) > 0,
	})
	if !coh.Incoherent || !o.failurePolicy.IsFloor(coh.Category) {
		return nil
	}
	return &SystemFailureSignal{
		Category: coh.Category,
		Level:    policy.LevelSystem,
		Evidence: coh.Evidence,
		Halt:     true,
	}
}
