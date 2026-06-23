package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/phaseio"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// TestComparePhaseIOShadow_EquivalentNoMismatch: the typed Upstream assembled
// from a RoutingSignals digest must compare equal to that same digest — the
// shadow stage's "normal" outcome (zero mismatches across a soak ⇒ safe to
// advance to advisory). Independent re-derivation, so it catches a
// HandoffsFromSignals projection bug rather than being tautological.
func TestComparePhaseIOShadow_EquivalentNoMismatch(t *testing.T) {
	sig := router.RoutingSignals{
		Scout: router.ScoutSignals{CycleSizeEstimate: "medium", ItemCount: 3, BacklogSize: 7, Present: true},
		Build: router.BuildSignals{Verdict: "PASS", SeverityMax: router.SevHigh, FilesTouched: 3, ACSRed: 1, DiffLOC: 42, Present: true},
		Audit: router.AuditSignals{Verdict: "PASS", RedCount: 0, Confidence: 0.9, Present: true},
	}
	h := router.HandoffsFromSignals(sig)
	if ms := comparePhaseIOShadow(h, sig); len(ms) != 0 {
		t.Fatalf("equivalent assembly should yield no mismatch, got %+v", ms)
	}
}

// TestComparePhaseIOShadow_DivergenceDetected: a Handoffs that does NOT match
// the digest (here: build present in the digest but absent in the assembly)
// must surface a mismatch.
func TestComparePhaseIOShadow_DivergenceDetected(t *testing.T) {
	sig := router.RoutingSignals{Build: router.BuildSignals{Verdict: "PASS", SeverityMax: router.SevHigh, Present: true}}
	h := phaseio.NewHandoffs(phaseio.HandoffsInit{}) // build absent → diverges from sig
	ms := comparePhaseIOShadow(h, sig)
	if len(ms) == 0 {
		t.Fatal("divergent assembly should yield at least one mismatch")
	}
	// the build.present field must be among the reported mismatches
	found := false
	for _, m := range ms {
		if m.Field == "build.present" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected build.present mismatch, got %+v", ms)
	}
}

// TestComparePhaseIOShadow_CoversAllProjectedFields pins the comparator's
// completeness contract: every field HandoffsFromSignals projects must be
// compared, so a projection bug surfaces as a mismatch. Perturbs exactly the
// fields the first cut omitted (phase_skip, the ACS siblings, DefectsBySeverity
// incl. the .String() key conversion) and asserts each divergence is caught.
func TestComparePhaseIOShadow_CoversAllProjectedFields(t *testing.T) {
	sig := router.RoutingSignals{
		Triage: router.TriageSignals{CycleSize: "medium", PhaseSkip: []string{"tdd"}, Present: true},
		Build:  router.BuildSignals{Verdict: "PASS", ACSGreen: 10, ACSTotal: 12, ACSThisCycle: 4, ACSRegression: 8, Present: true},
		Audit:  router.AuditSignals{Verdict: "PASS", DefectsBySeverity: map[router.Severity]int{router.SevHigh: 2}, Present: true},
	}
	// An assembled view diverging in exactly the previously-uncompared fields.
	h := phaseio.NewHandoffs(phaseio.HandoffsInit{
		Triage: &phaseio.TriageView{CycleSize: "medium", PhaseSkip: []string{"retro"}},
		Build:  &phaseio.BuildView{Verdict: "PASS", ACSGreen: 999, ACSTotal: 12, ACSThisCycle: 4, ACSRegression: 8},
		Audit:  &phaseio.AuditView{Verdict: "PASS", DefectsBySeverity: map[string]int{"HIGH": 99}},
	})
	got := map[string]bool{}
	for _, m := range comparePhaseIOShadow(h, sig) {
		got[m.Field] = true
	}
	for _, want := range []string{"triage.phase_skip", "build.acs_green", "audit.defects.HIGH"} {
		if !got[want] {
			t.Errorf("comparator missed divergence in %q", want)
		}
	}
}

// TestPhaseIOShadow_MismatchEmitsLedgerEntry: a non-empty mismatch list appends
// exactly one phaseio_shadow_mismatch ledger entry; an empty list appends none.
func TestPhaseIOShadow_MismatchEmitsLedgerEntry(t *testing.T) {
	fl := &fakeLedger{}

	// Equivalent (no mismatch) → no ledger entry.
	appendPhaseIOShadowMismatch(context.Background(), fl, "2026-06-15T00:00:00Z", 7, "run1", PhaseBuild, nil)
	if len(fl.entries) != 0 {
		t.Fatalf("no mismatch must emit no entry, got %d", len(fl.entries))
	}

	// Mismatch → exactly one entry of the right kind/identity.
	ms := []phaseIOMismatch{{Field: "build.present", Want: "true", Got: "false"}}
	appendPhaseIOShadowMismatch(context.Background(), fl, "2026-06-15T00:00:00Z", 7, "run1", PhaseBuild, ms)
	if len(fl.entries) != 1 {
		t.Fatalf("mismatch must emit one entry, got %d", len(fl.entries))
	}
	e := fl.entries[0]
	if e.Kind != "phaseio_shadow_mismatch" {
		t.Errorf("Kind = %q, want phaseio_shadow_mismatch", e.Kind)
	}
	if e.Cycle != 7 || e.Role != "build" || e.RunID != "run1" {
		t.Errorf("entry identity = {cycle:%d role:%q run:%q}, want {7 build run1}", e.Cycle, e.Role, e.RunID)
	}
	if e.Message == "" {
		t.Error("mismatch entry must carry a human-readable Message")
	}
}

// TestAssembleCycleInputs_FromContext: the typed CycleInputs is populated from
// the exact legacy ctxSnap keys the phases read today (incl. the camelCase
// challengeToken key, not snake_case).
func TestAssembleCycleInputs_FromContext(t *testing.T) {
	ctx := map[string]string{
		"goal":              "cut latency",
		"strategy":          "profile-first",
		"commit_message":    "perf: cache",
		"fleet_scope":       "core",
		"challengeToken":    "tok-9",
		"previous_verdict":  "FAIL",
		"carryover_summary": "carried: tighten the digest fallback",
	}
	ci := assembleCycleInputs(ctx)
	if ci.Goal() != "cut latency" || ci.Strategy() != "profile-first" || ci.CommitMessage() != "perf: cache" ||
		ci.FleetScope() != "core" || ci.ChallengeToken() != "tok-9" || ci.PreviousVerdict() != "FAIL" ||
		ci.Carryover() != "carried: tighten the digest fallback" {
		t.Fatalf("assembleCycleInputs mismapped: %+v", ci)
	}
}

// TestAssembleErrorContext_PresentAndAbsent: the ship_error_* keys assemble into
// a typed ErrorContext, or nil when none are set.
func TestAssembleErrorContext_PresentAndAbsent(t *testing.T) {
	if ec := assembleErrorContext(map[string]string{}); ec != nil {
		t.Fatalf("no ship_error_* keys → want nil, got %+v", ec)
	}
	ec := assembleErrorContext(map[string]string{
		"ship_error_code": "E_PUSH", "ship_error_class": "transient",
		"ship_error_stage": "ship", "ship_error_debug": "non-ff",
	})
	if ec == nil || ec.Code != "E_PUSH" || ec.Class != "transient" || ec.Stage != "ship" || ec.Debug != "non-ff" {
		t.Fatalf("assembleErrorContext mismapped: %+v", ec)
	}
}

// TestRetro_PreviousVerdict_FromCycleInputs_MatchesContext is the named 3.5
// anchor: for the retro phase, the typed CycleInputs.PreviousVerdict() must
// equal the legacy req.Context["previous_verdict"] the retro phase reads — and
// the shadow comparator must report ZERO mismatch when they agree.
func TestRetro_PreviousVerdict_FromCycleInputs_MatchesContext(t *testing.T) {
	// Mirrors the dispatch retro-clone: phaseCtx carries previous_verdict.
	phaseCtx := map[string]string{"goal": "g", "previous_verdict": VerdictFAIL}
	ci := assembleCycleInputs(phaseCtx)
	if ci.PreviousVerdict() != phaseCtx["previous_verdict"] {
		t.Fatalf("typed PreviousVerdict()=%q != Context[previous_verdict]=%q", ci.PreviousVerdict(), phaseCtx["previous_verdict"])
	}
	if ms := compareCycleInputsShadow(ci, assembleErrorContext(phaseCtx), phaseCtx); len(ms) != 0 {
		t.Fatalf("typed == legacy must yield no mismatch, got %+v", ms)
	}
}

// ── ADR-0050 Phase 3.6: named per-reader equivalence anchors ──────────────────
// One named test per remaining Context reader (scout/triage/intent/ship/debugger),
// each pinning that the typed CycleInputs/ErrorContext getter reproduces the exact
// legacy req.Context key the live phase still reads — the per-field key-drift guard
// the soak relies on. Each test asserts ONLY its own field so a failure localizes
// to the reader it names (the comparator's drift behaviour, incl. clean and
// diverging paths, is proven once over ALL fields in
// TestCompareCycleInputsShadow_KeyDrift). The phase code keeps reading req.Context
// until the 3.10 enforce cutover; these prove the typed envelope is a faithful
// shadow of every reader before that cutover is permitted.
//
// These tests run unconditionally — they exercise the assembler directly. At
// EVOLVE_PHASE_IO=off the production shadow hook (emitPhaseIOShadow, gated at
// cyclerun_dispatch.go:112) is never called, but the unit tests remain always-on.

// TestScout_Strategy_FromCycleInputs_MatchesContext: scout reads
// req.Context["strategy"] (scout.go ComposePrompt + Classify).
func TestScout_Strategy_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"strategy": "profile-first", "goal": "cut latency", "challengeToken": "tok-7"}
	if got := assembleCycleInputs(phaseCtx).Strategy(); got != phaseCtx["strategy"] {
		t.Fatalf("typed Strategy()=%q != Context[strategy]=%q", got, phaseCtx["strategy"])
	}
}

// TestScout_Goal_FromCycleInputs_MatchesContext: scout reads req.Context["goal"]
// (scout.go ComposePrompt — the operator --goal-text constraint).
func TestScout_Goal_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"strategy": "profile-first", "goal": "cut latency", "challengeToken": "tok-7"}
	if got := assembleCycleInputs(phaseCtx).Goal(); got != phaseCtx["goal"] {
		t.Fatalf("typed Goal()=%q != Context[goal]=%q", got, phaseCtx["goal"])
	}
}

// TestScout_ChallengeToken_FromCycleInputs_MatchesContext: scout reads the
// camelCase req.Context["challengeToken"] (scout.go ComposePrompt). The typed
// getter must read the SAME camelCase key, not the snake_case wire-JSON name.
func TestScout_ChallengeToken_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"strategy": "profile-first", "goal": "cut latency", "challengeToken": "tok-7"}
	if got := assembleCycleInputs(phaseCtx).ChallengeToken(); got != phaseCtx["challengeToken"] {
		t.Fatalf("typed ChallengeToken()=%q != Context[challengeToken]=%q", got, phaseCtx["challengeToken"])
	}
}

// TestTriage_FleetScope_FromCycleInputs_MatchesContext: triage reads
// req.Context["fleet_scope"] (triage.go ComposePrompt).
func TestTriage_FleetScope_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"fleet_scope": "core,bridge", "carryover_summary": "carried: x"}
	if got := assembleCycleInputs(phaseCtx).FleetScope(); got != phaseCtx["fleet_scope"] {
		t.Fatalf("typed FleetScope()=%q != Context[fleet_scope]=%q", got, phaseCtx["fleet_scope"])
	}
}

// TestTriage_Carryover_FromCycleInputs_MatchesContext: triage reads
// req.Context["carryover_summary"] (triage.go:63). The typed getter is
// Carryover() — note the legacy key is carryover_summary, not carryover.
// (3.6's genuinely-new field; this is the RED that drives the leaf addition.)
func TestTriage_Carryover_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"fleet_scope": "core", "carryover_summary": "carried: finish the digest fallback"}
	if got := assembleCycleInputs(phaseCtx).Carryover(); got != phaseCtx["carryover_summary"] {
		t.Fatalf("typed Carryover()=%q != Context[carryover_summary]=%q", got, phaseCtx["carryover_summary"])
	}
}

// TestIntent_Goal_FromCycleInputs_MatchesContext: intent reads req.Context["goal"]
// (intent.go:54).
func TestIntent_Goal_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"goal": "add a typed envelope"}
	if got := assembleCycleInputs(phaseCtx).Goal(); got != phaseCtx["goal"] {
		t.Fatalf("typed Goal()=%q != Context[goal]=%q", got, phaseCtx["goal"])
	}
}

// TestShip_CommitMessage_FromCycleInputs_MatchesContext: ship reads
// req.Context["commit_message"] (ship.go:72).
func TestShip_CommitMessage_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"commit_message": "feat(core): unified phase I/O"}
	if got := assembleCycleInputs(phaseCtx).CommitMessage(); got != phaseCtx["commit_message"] {
		t.Fatalf("typed CommitMessage()=%q != Context[commit_message]=%q", got, phaseCtx["commit_message"])
	}
}

// TestDebugger_ErrorContext_FromCycleInputs_MatchesContext: the debugger reads the
// ship_error_* keys (debugger.go ComposePrompt) carried in by the recovery path;
// the typed ErrorContext must reproduce all four.
func TestDebugger_ErrorContext_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{
		"ship_error_code": "E_PUSH", "ship_error_class": "transient",
		"ship_error_stage": "ship", "ship_error_debug": "non-ff",
	}
	ec := assembleErrorContext(phaseCtx)
	if ec == nil || ec.Code != phaseCtx["ship_error_code"] || ec.Class != phaseCtx["ship_error_class"] ||
		ec.Stage != phaseCtx["ship_error_stage"] || ec.Debug != phaseCtx["ship_error_debug"] {
		t.Fatalf("typed ErrorContext != Context ship_error_*: %+v", ec)
	}
}

// TestCompareCycleInputsShadow_KeyDrift: the comparator catches a typed view
// whose value diverges from the legacy Context key (the key-drift bug class —
// e.g. an assembler reading the wrong key name). Table-driven over EVERY
// cycle_inputs field so each comparator line — incl. the 3.6 carryover line —
// has genuine drift coverage: each row supplies the ground-truth Context key the
// live phase reads + a CycleInputs whose corresponding getter is drifted, and
// asserts the comparator reports exactly that field with want/got in the right
// orientation (want=legacy ground truth, got=typed getter). The empty want/got
// guards against a swapped mismatch struct.
func TestCompareCycleInputsShadow_KeyDrift(t *testing.T) {
	const real, wrong = "real-value", "wrong-value"
	cases := []struct {
		name      string
		ctxKey    string                  // the legacy Context key the phase reads
		field     string                  // the comparator field name
		driftInit phaseio.CycleInputsInit // a CycleInputs whose getter is drifted to `wrong`
	}{
		{"goal", "goal", "cycle_inputs.goal", phaseio.CycleInputsInit{Goal: wrong}},
		{"strategy", "strategy", "cycle_inputs.strategy", phaseio.CycleInputsInit{Strategy: wrong}},
		{"commit_message", "commit_message", "cycle_inputs.commit_message", phaseio.CycleInputsInit{CommitMessage: wrong}},
		{"fleet_scope", "fleet_scope", "cycle_inputs.fleet_scope", phaseio.CycleInputsInit{FleetScope: wrong}},
		// challenge_token: the comparator field name is snake_case, the live
		// Context key is camelCase challengeToken — the canonical key-drift trap.
		{"challenge_token", "challengeToken", "cycle_inputs.challenge_token", phaseio.CycleInputsInit{ChallengeToken: wrong}},
		{"previous_verdict", "previous_verdict", "cycle_inputs.previous_verdict", phaseio.CycleInputsInit{PreviousVerdict: wrong}},
		// carryover: the comparator field name is carryover, the live Context key
		// is carryover_summary (triage) — the 3.6 key-drift trap.
		{"carryover", "carryover_summary", "cycle_inputs.carryover", phaseio.CycleInputsInit{Carryover: wrong}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := map[string]string{tc.ctxKey: real}
			ms := compareCycleInputsShadow(phaseio.NewCycleInputs(tc.driftInit), nil, ctx)
			var got *phaseIOMismatch
			for i := range ms {
				if ms[i].Field == tc.field {
					got = &ms[i]
				}
			}
			if got == nil {
				t.Fatalf("expected %s drift, got %+v", tc.field, ms)
			}
			if got.Want != real || got.Got != wrong {
				t.Fatalf("want/got orientation wrong for %s: %+v", tc.field, *got)
			}
		})
	}
}

// TestCompareCycleInputsShadow_ErrorContext exercises the comparator's typed
// ErrorContext path (the ship-failure recovery case): a matching ErrorContext
// yields no mismatch; a diverging one surfaces the offending field with correct
// want/got.
func TestCompareCycleInputsShadow_ErrorContext(t *testing.T) {
	ctx := map[string]string{
		"ship_error_code": "E_PUSH", "ship_error_class": "transient",
		"ship_error_stage": "ship", "ship_error_debug": "non-ff",
	}
	if ms := compareCycleInputsShadow(assembleCycleInputs(ctx), assembleErrorContext(ctx), ctx); len(ms) != 0 {
		t.Fatalf("matching ErrorContext should yield no mismatch, got %+v", ms)
	}
	diverge := &phaseio.ErrorContext{Code: "WRONG", Class: "transient", Stage: "ship", Debug: "non-ff"}
	ms := compareCycleInputsShadow(assembleCycleInputs(ctx), diverge, ctx)
	found := false
	for _, m := range ms {
		if m.Field == "error_context.code" && m.Want == "E_PUSH" && m.Got == "WRONG" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error_context.code divergence (want E_PUSH got WRONG), got %+v", ms)
	}
}

// TestWritePhaseIOShadowFile_Parseable: the shadow artifact is written as
// parseable JSON capturing the assembled upstream presence + any mismatches.
func TestWritePhaseIOShadowFile_Parseable(t *testing.T) {
	ws := t.TempDir()
	h := router.HandoffsFromSignals(router.RoutingSignals{Build: router.BuildSignals{Verdict: "PASS", Present: true}})
	if err := writePhaseIOShadowFile(ws, "build", h, 5, nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(ws, "phaseio-shadow-build.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var doc phaseIOShadowDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Phase != "build" || doc.Cycle != 5 || !doc.BuildPresent || doc.ScoutPresent {
		t.Fatalf("unexpected shadow doc: %+v", doc)
	}
}
