package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

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

func TestComparePhaseIOShadow_DivergenceDetected(t *testing.T) {
	sig := router.RoutingSignals{Build: router.BuildSignals{Verdict: "PASS", SeverityMax: router.SevHigh, Present: true}}
	h := phaseio.NewHandoffs(phaseio.HandoffsInit{}) // build absent → diverges from sig
	ms := comparePhaseIOShadow(h, sig)
	if len(ms) == 0 {
		t.Fatal("divergent assembly should yield at least one mismatch")
	}
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

func TestPhaseIOShadow_MismatchEmitsLedgerEntry(t *testing.T) {
	fl := &fakeLedger{}

	appendPhaseIOShadowMismatch(context.Background(), fl, "2026-06-15T00:00:00Z", 7, "run1", PhaseBuild, nil)
	if len(fl.entries) != 0 {
		t.Fatalf("no mismatch must emit no entry, got %d", len(fl.entries))
	}

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

func TestRetro_PreviousVerdict_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"goal": "g", "previous_verdict": VerdictFAIL}
	ci := assembleCycleInputs(phaseCtx)
	if ci.PreviousVerdict() != phaseCtx["previous_verdict"] {
		t.Fatalf("typed PreviousVerdict()=%q != Context[previous_verdict]=%q", ci.PreviousVerdict(), phaseCtx["previous_verdict"])
	}
	if ms := compareCycleInputsShadow(ci, assembleErrorContext(phaseCtx), phaseCtx); len(ms) != 0 {
		t.Fatalf("typed == legacy must yield no mismatch, got %+v", ms)
	}
}

func TestScout_Strategy_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"strategy": "profile-first", "goal": "cut latency", "challengeToken": "tok-7"}
	if got := assembleCycleInputs(phaseCtx).Strategy(); got != phaseCtx["strategy"] {
		t.Fatalf("typed Strategy()=%q != Context[strategy]=%q", got, phaseCtx["strategy"])
	}
}

func TestScout_Goal_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"strategy": "profile-first", "goal": "cut latency", "challengeToken": "tok-7"}
	if got := assembleCycleInputs(phaseCtx).Goal(); got != phaseCtx["goal"] {
		t.Fatalf("typed Goal()=%q != Context[goal]=%q", got, phaseCtx["goal"])
	}
}

func TestScout_ChallengeToken_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"strategy": "profile-first", "goal": "cut latency", "challengeToken": "tok-7"}
	if got := assembleCycleInputs(phaseCtx).ChallengeToken(); got != phaseCtx["challengeToken"] {
		t.Fatalf("typed ChallengeToken()=%q != Context[challengeToken]=%q", got, phaseCtx["challengeToken"])
	}
}

func TestTriage_FleetScope_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"fleet_scope": "core,bridge", "carryover_summary": "carried: x"}
	if got := assembleCycleInputs(phaseCtx).FleetScope(); got != phaseCtx["fleet_scope"] {
		t.Fatalf("typed FleetScope()=%q != Context[fleet_scope]=%q", got, phaseCtx["fleet_scope"])
	}
}

func TestTriage_Carryover_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"fleet_scope": "core", "carryover_summary": "carried: finish the digest fallback"}
	if got := assembleCycleInputs(phaseCtx).Carryover(); got != phaseCtx["carryover_summary"] {
		t.Fatalf("typed Carryover()=%q != Context[carryover_summary]=%q", got, phaseCtx["carryover_summary"])
	}
}

func TestIntent_Goal_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"goal": "add a typed envelope"}
	if got := assembleCycleInputs(phaseCtx).Goal(); got != phaseCtx["goal"] {
		t.Fatalf("typed Goal()=%q != Context[goal]=%q", got, phaseCtx["goal"])
	}
}

func TestShip_CommitMessage_FromCycleInputs_MatchesContext(t *testing.T) {
	phaseCtx := map[string]string{"commit_message": "feat(core): unified phase I/O"}
	if got := assembleCycleInputs(phaseCtx).CommitMessage(); got != phaseCtx["commit_message"] {
		t.Fatalf("typed CommitMessage()=%q != Context[commit_message]=%q", got, phaseCtx["commit_message"])
	}
}

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
		// is carryover_summary (triage) — the same key-drift trap.
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

// overCapCtxValue is a valid-UTF-8 value strictly larger than the cap whose
// first byte is `lead` so two values can differ INSIDE the retained prefix.
func overCapCtxValue(lead byte) string {
	return string(lead) + strings.Repeat("x", phaseio.MaxFieldBytes+64)
}

func mismatchFor(ms []phaseIOMismatch, field string) *phaseIOMismatch {
	for i := range ms {
		if ms[i].Field == field {
			return &ms[i]
		}
	}
	return nil
}

func TestCompareCycleInputsShadow_CapAware_NoFalseMismatch(t *testing.T) {
	ctx := map[string]string{
		"carryover_summary": overCapCtxValue('a'),
		"previous_verdict":  overCapCtxValue('b'),
	}
	if ms := compareCycleInputsShadow(assembleCycleInputs(ctx), nil, ctx); len(ms) != 0 {
		t.Fatalf("over-cap legacy values assembled via assembleCycleInputs must not mismatch, got %+v", summarizePhaseIOMismatches(ms))
	}
	// Cap-equivalent: identical inside the cap, different only past it.
	beyond := map[string]string{
		"carryover_summary": overCapCtxValue('a') + "TAIL-ONLY-DIFFERENCE",
		"previous_verdict":  overCapCtxValue('b') + "TAIL-ONLY-DIFFERENCE",
	}
	ms := compareCycleInputsShadow(assembleCycleInputs(ctx), nil, beyond)
	for _, f := range []string{"cycle_inputs.carryover", "cycle_inputs.previous_verdict"} {
		if m := mismatchFor(ms, f); m != nil {
			t.Errorf("%s: cap-equivalent values (differ only beyond the cap) reported as drift: want=%d bytes got=%d bytes", f, len(m.Want), len(m.Got))
		}
	}
}

func TestCompareCycleInputsShadow_DetectsRealDrift(t *testing.T) {
	cases := []struct {
		name, ctxKey, field string
		typed               phaseio.CycleInputsInit
	}{
		{"carryover", "carryover_summary", "cycle_inputs.carryover", phaseio.CycleInputsInit{Carryover: overCapCtxValue('B')}},
		{"previous_verdict", "previous_verdict", "cycle_inputs.previous_verdict", phaseio.CycleInputsInit{PreviousVerdict: overCapCtxValue('B')}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			legacy := overCapCtxValue('A')
			ctx := map[string]string{tc.ctxKey: legacy}
			ms := compareCycleInputsShadow(phaseio.NewCycleInputs(tc.typed), nil, ctx)
			m := mismatchFor(ms, tc.field)
			if m == nil {
				t.Fatalf("genuine over-cap drift on %s was swallowed (comparator is cap-BLIND): %+v", tc.field, ms)
			}
			if m.Want != phaseio.CapField(legacy) {
				t.Errorf("want must be the bounded legacy value (len %d), got len %d — raw text would leak into the ledger", len(phaseio.CapField(legacy)), len(m.Want))
			}
			if m.Got != phaseio.CapField(overCapCtxValue('B')) {
				t.Errorf("got must be the typed getter's bounded value, got len %d", len(m.Got))
			}
			for _, other := range ms {
				if other.Field != tc.field {
					t.Errorf("unexpected extra mismatch on %s: %+v", other.Field, other)
				}
			}
		})
	}
}
