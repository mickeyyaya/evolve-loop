package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// phaseIOMismatch records one field where the assembled typed Upstream view
// (phaseio.Handoffs) diverged from the legacy routing digest during the
// EVOLVE_PHASE_IO shadow stage (ADR-0050 Phase 3.4).
type phaseIOMismatch struct {
	Field string `json:"field"`
	Want  string `json:"want"` // the legacy router.RoutingSignals value
	Got   string `json:"got"`  // the assembled phaseio.Handoffs value
}

// phaseIOShadowDoc is the on-disk shadow artifact (phaseio-shadow-<phase>.json):
// the assembled upstream presence + any divergences, for soak inspection.
type phaseIOShadowDoc struct {
	Phase         string            `json:"phase"`
	Cycle         int               `json:"cycle"`
	ScoutPresent  bool              `json:"scout_present"`
	TriagePresent bool              `json:"triage_present"`
	BuildPresent  bool              `json:"build_present"`
	AuditPresent  bool              `json:"audit_present"`
	Mismatches    []phaseIOMismatch `json:"mismatches,omitempty"`
}

// comparePhaseIOShadow compares the assembled typed Upstream view against the
// legacy routing digest field-by-field, returning every divergence. An empty
// result means the typed Handoffs faithfully reproduces what the legacy router
// consumes. It re-derives the expected values DIRECTLY from sig (not via
// HandoffsFromSignals) so it is an independent check — a projection bug in
// HandoffsFromSignals shows up as a mismatch rather than being masked. Pure and
// total.
func comparePhaseIOShadow(h phaseio.Handoffs, sig router.RoutingSignals) []phaseIOMismatch {
	var ms []phaseIOMismatch
	add := func(field, want, got string) {
		if want != got {
			ms = append(ms, phaseIOMismatch{Field: field, Want: want, Got: got})
		}
	}

	sc, scOK := h.Scout()
	add("scout.present", strconv.FormatBool(sig.Scout.Present), strconv.FormatBool(scOK))
	if sig.Scout.Present && scOK {
		add("scout.cycle_size_estimate", sig.Scout.CycleSizeEstimate, sc.CycleSizeEstimate)
		add("scout.item_count", strconv.Itoa(sig.Scout.ItemCount), strconv.Itoa(sc.ItemCount))
		add("scout.carryover_count", strconv.Itoa(sig.Scout.CarryoverCount), strconv.Itoa(sc.CarryoverCount))
		add("scout.backlog_size", strconv.Itoa(sig.Scout.BacklogSize), strconv.Itoa(sc.BacklogSize))
	}

	tr, trOK := h.Triage()
	add("triage.present", strconv.FormatBool(sig.Triage.Present), strconv.FormatBool(trOK))
	if sig.Triage.Present && trOK {
		add("triage.cycle_size", sig.Triage.CycleSize, tr.CycleSize)
		add("triage.phase_skip", strings.Join(sig.Triage.PhaseSkip, ","), strings.Join(tr.PhaseSkip, ","))
	}

	b, bOK := h.Build()
	add("build.present", strconv.FormatBool(sig.Build.Present), strconv.FormatBool(bOK))
	if sig.Build.Present && bOK {
		add("build.verdict", sig.Build.Verdict, b.Verdict)
		add("build.severity_max", sig.Build.SeverityMax.String(), b.SeverityMax)
		add("build.files_touched", strconv.Itoa(sig.Build.FilesTouched), strconv.Itoa(b.FilesTouched))
		add("build.diff_loc", strconv.Itoa(sig.Build.DiffLOC), strconv.Itoa(b.DiffLOC))
		add("build.acs_green", strconv.Itoa(sig.Build.ACSGreen), strconv.Itoa(b.ACSGreen))
		add("build.acs_red", strconv.Itoa(sig.Build.ACSRed), strconv.Itoa(b.ACSRed))
		add("build.acs_total", strconv.Itoa(sig.Build.ACSTotal), strconv.Itoa(b.ACSTotal))
		add("build.acs_this_cycle", strconv.Itoa(sig.Build.ACSThisCycle), strconv.Itoa(b.ACSThisCycle))
		add("build.acs_regression", strconv.Itoa(sig.Build.ACSRegression), strconv.Itoa(b.ACSRegression))
	}

	a, aOK := h.Audit()
	add("audit.present", strconv.FormatBool(sig.Audit.Present), strconv.FormatBool(aOK))
	if sig.Audit.Present && aOK {
		add("audit.verdict", sig.Audit.Verdict, a.Verdict)
		add("audit.red_count", strconv.Itoa(sig.Audit.RedCount), strconv.Itoa(a.RedCount))
		add("audit.confidence",
			strconv.FormatFloat(sig.Audit.Confidence, 'g', -1, 64),
			strconv.FormatFloat(a.Confidence, 'g', -1, 64))
		// DefectsBySeverity: legacy keys by the Severity ordinal, the assembled
		// view by the canonical word — compare bucket count + each per-severity
		// count through the same .String() conversion HandoffsFromSignals applies
		// (the only non-trivial projection, so the highest-value field to check).
		add("audit.defect_buckets", strconv.Itoa(len(sig.Audit.DefectsBySeverity)), strconv.Itoa(len(a.DefectsBySeverity)))
		for sev, n := range sig.Audit.DefectsBySeverity {
			add("audit.defects."+sev.String(), strconv.Itoa(n), strconv.Itoa(a.DefectsBySeverity[sev.String()]))
		}
	}
	return ms
}

// assembleCycleInputs builds the typed CycleInputs from the legacy per-phase
// Context map (ctxSnap/phaseCtx), reading the SAME keys the phases read today
// (ADR-0050 Phase 3.5/3.6). Note challengeToken is camelCase (the live Context
// key), not the snake_case wire-JSON field name; carryover reads the legacy
// carryover_summary key (triage), not carryover.
func assembleCycleInputs(ctx map[string]string) phaseio.CycleInputs {
	return phaseio.NewCycleInputs(phaseio.CycleInputsInit{
		Goal:            ctx["goal"],
		Strategy:        ctx["strategy"],
		CommitMessage:   ctx["commit_message"],
		FleetScope:      ctx["fleet_scope"],
		ChallengeToken:  ctx["challengeToken"],
		PreviousVerdict: ctx["previous_verdict"],
		Carryover:       ctx["carryover_summary"],
	})
}

// assembleErrorContext builds the typed ErrorContext from the ship_error_* keys
// the recovery path injects, or nil when none are present (no upstream error).
func assembleErrorContext(ctx map[string]string) *phaseio.ErrorContext {
	code, class := ctx["ship_error_code"], ctx["ship_error_class"]
	stage, debug := ctx["ship_error_stage"], ctx["ship_error_debug"]
	if code == "" && class == "" && stage == "" && debug == "" {
		return nil
	}
	return &phaseio.ErrorContext{Code: code, Class: class, Stage: stage, Debug: debug}
}

// compareCycleInputsShadow compares the typed CycleInputs + ErrorContext against
// the legacy Context map, field-by-field. "want" is the legacy value keyed by
// what the phases ACTUALLY read (the ground truth) and "got" is the typed
// getter — so an assembler that drifts to a wrong key (e.g. snake_case
// challenge_token vs the live camelCase challengeToken) surfaces as a mismatch.
func compareCycleInputsShadow(ci phaseio.CycleInputs, ec *phaseio.ErrorContext, ctx map[string]string) []phaseIOMismatch {
	var ms []phaseIOMismatch
	add := func(field, want, got string) {
		if want != got {
			ms = append(ms, phaseIOMismatch{Field: field, Want: want, Got: got})
		}
	}
	add("cycle_inputs.goal", ctx["goal"], ci.Goal())
	add("cycle_inputs.strategy", ctx["strategy"], ci.Strategy())
	add("cycle_inputs.commit_message", ctx["commit_message"], ci.CommitMessage())
	add("cycle_inputs.fleet_scope", ctx["fleet_scope"], ci.FleetScope())
	add("cycle_inputs.challenge_token", ctx["challengeToken"], ci.ChallengeToken())
	add("cycle_inputs.previous_verdict", ctx["previous_verdict"], ci.PreviousVerdict())
	add("cycle_inputs.carryover", ctx["carryover_summary"], ci.Carryover())

	var gotCode, gotClass, gotStage, gotDebug string
	if ec != nil {
		gotCode, gotClass, gotStage, gotDebug = ec.Code, ec.Class, ec.Stage, ec.Debug
	}
	add("error_context.code", ctx["ship_error_code"], gotCode)
	add("error_context.class", ctx["ship_error_class"], gotClass)
	add("error_context.stage", ctx["ship_error_stage"], gotStage)
	add("error_context.debug", ctx["ship_error_debug"], gotDebug)
	return ms
}

// summarizePhaseIOMismatches renders mismatches into a human-readable ledger
// Message (e.g. `build.present(want="true" got="false"); ...`).
func summarizePhaseIOMismatches(ms []phaseIOMismatch) string {
	parts := make([]string, 0, len(ms))
	for _, m := range ms {
		parts = append(parts, fmt.Sprintf("%s(want=%q got=%q)", m.Field, m.Want, m.Got))
	}
	return strings.Join(parts, "; ")
}

// appendPhaseIOShadowMismatch appends a single phaseio_shadow_mismatch ledger
// entry iff ms is non-empty. Best-effort: a shadow-stage ledger failure must
// never affect the cycle, so the append error is intentionally dropped.
func appendPhaseIOShadowMismatch(ctx context.Context, l Ledger, ts string, cycle int, runID string, phase Phase, ms []phaseIOMismatch) {
	if len(ms) == 0 {
		return
	}
	if err := l.Append(ctx, LedgerEntry{
		TS:      ts,
		Cycle:   cycle,
		Role:    string(phase),
		Kind:    "phaseio_shadow_mismatch",
		Message: summarizePhaseIOMismatches(ms),
		RunID:   runID,
	}); err != nil {
		// No-abort shadow contract: surface the failure (matching the core
		// best-effort-append idiom) but never propagate it into the cycle.
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phaseio shadow ledger append failed for %s: %v\n", phase, err)
	}
}

// writePhaseIOShadowFile writes the shadow artifact to the workspace.
func writePhaseIOShadowFile(workspace, phase string, h phaseio.Handoffs, cycle int, ms []phaseIOMismatch) error {
	_, scOK := h.Scout()
	_, trOK := h.Triage()
	_, bOK := h.Build()
	_, aOK := h.Audit()
	doc := phaseIOShadowDoc{
		Phase: phase, Cycle: cycle,
		ScoutPresent: scOK, TriagePresent: trOK, BuildPresent: bOK, AuditPresent: aOK,
		Mismatches: ms,
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workspace, "phaseio-shadow-"+phase+".json"), data, 0o644)
}

// emitPhaseIOShadowWithSig is the dispatch-time shadow comparison (active only at
// EVOLVE_PHASE_IO>=shadow). Given the already-computed routing digest, it projects
// the typed Upstream view, assembles the typed CycleInputs/ErrorContext from
// phaseCtx, compares BOTH against the legacy sources, writes the shadow artifact,
// and on any divergence emits a WARN + a phaseio_shadow_mismatch ledger entry. It
// NEVER returns an error or affects dispatchResult — at EVOLVE_PHASE_IO=off it is
// not called at all, so the live loop stays byte-identical. The caller
// (assemblePhaseIO) owns the single router.Digest so the enforce stage can reuse
// the same sig to assemble the authoritative PhaseInput without a second
// workspace read.
func (cr *cycleRun) emitPhaseIOShadowWithSig(phase Phase, phaseCtx map[string]string, sig router.RoutingSignals) {
	h := router.HandoffsFromSignals(sig)
	ms := comparePhaseIOShadow(h, sig)
	// Phase 3.5: also assemble + compare the typed CycleInputs/ErrorContext
	// against the legacy Context map (the key-drift guard).
	ci := assembleCycleInputs(phaseCtx)
	ec := assembleErrorContext(phaseCtx)
	ms = append(ms, compareCycleInputsShadow(ci, ec, phaseCtx)...)
	if werr := writePhaseIOShadowFile(cr.cs.WorkspacePath, string(phase), h, cr.cycle, ms); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phaseio shadow write failed for %s: %v\n", phase, werr)
	}
	if len(ms) > 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phaseio shadow mismatch for %s: %s\n", phase, summarizePhaseIOMismatches(ms))
		appendPhaseIOShadowMismatch(cr.ctx, cr.o.ledger, cr.o.now().UTC().Format(time.RFC3339), cr.cycle, cr.cs.RunID, phase, ms)
	}
}
