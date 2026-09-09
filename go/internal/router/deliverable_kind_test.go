package router

// deliverable_kind_test.go — ADR-0099 (deliverable kinds: solution cycles on
// the same spine). RED contract:
//
//   - The kernel derives TWO new objective signals from the report headers the
//     phases already write (handoff JSON has been extinct since ~cycle 215, so
//     the header line is the trusted path, exactly as triage's
//     `cycle_size_estimate:` reaches RoutingSignals today):
//       scout-report.md  → `goal_type: <goal_recipes key>` + `deliverable_kind: code|document`
//       triage-report.md → `deliverable_kind: code|document` (authoritative, like cycle_size)
//   - RoutingSignals.DeliverableKind() projects triage > scout > "code". The
//     absent-default is "code" — the CONSERVATIVE side: a conditional rule
//     `deliverable_kind != document` evaluates TRUE pre-handoff, so the tdd pin
//     holds at plan time and is released only by a digested document signal
//     (post-scout RePlan). An unrecognised kind word is fail-safe: it never
//     releases anything.
//   - `scout.goal_type` finally has a producer, so the 15 shipped domain
//     phases' `insert_when` triggers (dormant since 2026-06-06) can fire.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

const scoutReportDocument = "# Cycle 999 Scout Report\n<!-- challenge-token: abc -->\n" +
	"goal_type: business-strategy\ndeliverable_kind: document\n\n" +
	"## Handoff Summary\n- **Decisions:** x\n\n## Selected Tasks\n### Task 1: margin strategy\n- **Slug:** netflix-margin\n"

func triageReport(kind string) string {
	return "<!-- challenge-token: abc -->\n<!-- ANCHOR:triage_decision -->\n# Triage Decision — Cycle 999\n\n" +
		"cycle_size_estimate: medium\ndeliverable_kind: " + kind + "\nphase_skip: []\n\n## top_n\n- netflix-margin: x\n"
}

func TestDigest_ScoutFromReportFallback_ExtractsGoalTypeAndDeliverableKind(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "scout-report.md", scoutReportDocument)
	sig, err := Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !sig.Scout.Present {
		t.Fatalf("scout report present → Present must be true")
	}
	if sig.Scout.GoalType != "business-strategy" {
		t.Errorf("Scout.GoalType = %q, want business-strategy", sig.Scout.GoalType)
	}
	if sig.Scout.DeliverableKind != "document" {
		t.Errorf("Scout.DeliverableKind = %q, want document", sig.Scout.DeliverableKind)
	}
	if got := sig.DeliverableKind(); got != "document" {
		t.Errorf("DeliverableKind() = %q, want document (scout projection when triage absent)", got)
	}
	// The typed fields are also routable by name through resolveField.
	if _, _, str, ok := resolveField(sig, "scout.goal_type"); !ok || str != "business-strategy" {
		t.Errorf("resolveField(scout.goal_type) = %q/%v, want business-strategy/true", str, ok)
	}
	if _, _, str, ok := resolveField(sig, "scout.deliverable_kind"); !ok || str != "document" {
		t.Errorf("resolveField(scout.deliverable_kind) = %q/%v, want document/true", str, ok)
	}
}

func TestDigest_TriageFromReportFallback_DeliverableKindIsAuthoritative(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "scout-report.md", scoutReportDocument)   // scout says document …
	writeFile(t, ws, "triage-report.md", triageReport("code")) // … triage refines to code (mixed top_n)
	sig, err := Digest(ws, []string{"scout", "triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if sig.Triage.DeliverableKind != "code" {
		t.Errorf("Triage.DeliverableKind = %q, want code", sig.Triage.DeliverableKind)
	}
	if got := sig.DeliverableKind(); got != "code" {
		t.Errorf("DeliverableKind() = %q, want code (triage is authoritative over scout)", got)
	}
	if _, _, str, ok := resolveField(sig, "triage.deliverable_kind"); !ok || str != "code" {
		t.Errorf("resolveField(triage.deliverable_kind) = %q/%v, want code/true", str, ok)
	}
	if _, _, str, ok := resolveField(sig, "deliverable_kind"); !ok || str != "code" {
		t.Errorf("resolveField(deliverable_kind) = %q/%v, want the projected code/true", str, ok)
	}
}

func TestRoutingSignals_DeliverableKind_DefaultsToCodeConservatively(t *testing.T) {
	var sig RoutingSignals // nothing digested yet (plan time)
	if got := sig.DeliverableKind(); got != "code" {
		t.Fatalf("DeliverableKind() pre-handoff = %q, want code", got)
	}
	// The tdd release clause must NOT fire pre-handoff: typed field, present, default code.
	if !evalCondition(sig, config.Condition{Field: "deliverable_kind", Op: "ne", Value: "document"}) {
		t.Errorf("deliverable_kind ne document with zero signals = false; the release must stay false (tdd pinned) pre-handoff")
	}
	// An unrecognised kind word is not a kind: fail-safe to code, never a release.
	ws := t.TempDir()
	writeFile(t, ws, "triage-report.md", triageReport("strategy"))
	sig2, err := Digest(ws, []string{"triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if got := sig2.DeliverableKind(); got != "code" {
		t.Errorf("DeliverableKind() with unknown word = %q, want code (fail-safe)", got)
	}
	if evalCondition(sig2, config.Condition{Field: "deliverable_kind", Op: "eq", Value: "document"}) {
		t.Errorf("an unknown kind word must never satisfy == document")
	}
}

func TestTddPinned_ReleasedForDocumentCycles(t *testing.T) {
	rule := config.CondRule{Field: "cycle_size", Op: "!=", Value: "trivial",
		And: []config.CondRule{{Field: "deliverable_kind", Op: "!=", Value: "document"}}}
	in := RouteInput{Cfg: config.RoutingConfig{Conditional: map[string]config.CondRule{"tdd": rule}}}
	if !tddPinned(in) {
		t.Errorf("plan time (no signals): tdd must stay pinned (conservative side)")
	}
	in.Signals = RoutingSignals{Triage: TriageSignals{CycleSize: "medium", DeliverableKind: "document", Present: true}}
	if tddPinned(in) {
		t.Errorf("non-trivial DOCUMENT cycle: the config rule must release tdd")
	}
	in.Signals = RoutingSignals{Triage: TriageSignals{CycleSize: "medium", DeliverableKind: "code", Present: true}}
	if !tddPinned(in) {
		t.Errorf("non-trivial CODE cycle: tdd stays pinned")
	}
	in.Signals = RoutingSignals{Triage: TriageSignals{CycleSize: "trivial", Present: true}}
	if tddPinned(in) {
		t.Errorf("trivial cycle: the legacy exemption is preserved by the AND rule")
	}
}

// TestDomainPhaseTrigger_FiresOnScoutGoalType is the first live firing of the
// domain catalog: the SHIPPED forces-analysis overlay's insert_when (read from
// the repo, never re-typed here) fires on a scout report that declares the goal
// type, and stays quiet on any other goal.
func TestDomainPhaseTrigger_FiresOnScoutGoalType(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", ".evolve", "phases", "forces-analysis", "phase.json"))
	if err != nil {
		t.Fatalf("read shipped overlay: %v", err)
	}
	var spec struct {
		Routing config.RoutingBlock `json:"routing"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil || len(spec.Routing.InsertWhen) == 0 {
		t.Fatalf("overlay routing block: %v (%+v)", err, spec.Routing)
	}
	ws := t.TempDir()
	writeFile(t, ws, "scout-report.md", scoutReportDocument)
	sig, err := Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !triggerFires(sig, spec.Routing) {
		t.Errorf("forces-analysis insert_when %+v did not fire on goal_type: business-strategy", spec.Routing.InsertWhen)
	}
	ws2 := t.TempDir()
	writeFile(t, ws2, "scout-report.md", "# Cycle 1 Scout Report\n<!-- challenge-token: abc -->\ngoal_type: feature\ndeliverable_kind: code\n\n## Selected Tasks\n### Task 1: x\n")
	sig2, _ := Digest(ws2, []string{"scout"})
	if triggerFires(sig2, spec.Routing) {
		t.Errorf("forces-analysis fired on goal_type: feature")
	}
}

func TestHandoffsFromSignals_CarriesGoalTypeAndDeliverableKind(t *testing.T) {
	sig := RoutingSignals{
		Scout:  ScoutSignals{Present: true, GoalType: "business-strategy", DeliverableKind: "document"},
		Triage: TriageSignals{Present: true, CycleSize: "medium", DeliverableKind: "document"},
	}
	h := HandoffsFromSignals(sig)
	sc, ok := h.Scout()
	if !ok || sc.GoalType != "business-strategy" || sc.DeliverableKind != "document" {
		t.Errorf("ScoutView = %+v (ok=%v), want goal_type/deliverable_kind carried", sc, ok)
	}
	tr, ok := h.Triage()
	if !ok || tr.DeliverableKind != "document" {
		t.Errorf("TriageView = %+v (ok=%v), want deliverable_kind carried", tr, ok)
	}
}

// TestNormalizeDeliverableKind names the kind vocabulary and its normaliser
// (apicover: every exported symbol is named by a test): case/whitespace
// tolerant for the two kinds, fail-safe "" for anything else.
func TestNormalizeDeliverableKind(t *testing.T) {
	if got := NormalizeDeliverableKind("CODE"); got != DeliverableKindCode {
		t.Errorf("NormalizeDeliverableKind(CODE) = %q, want %q", got, DeliverableKindCode)
	}
	if got := NormalizeDeliverableKind(" Document \t"); got != DeliverableKindDocument {
		t.Errorf("NormalizeDeliverableKind( Document ) = %q, want %q", got, DeliverableKindDocument)
	}
	for _, bad := range []string{"strategy", "", "documents"} {
		if got := NormalizeDeliverableKind(bad); got != "" {
			t.Errorf("NormalizeDeliverableKind(%q) = %q, want \"\" (an unrecognised word is not a kind)", bad, got)
		}
	}
}

// TestRoutingSignals_DeclaredDeliverableKind — ADR-0099 slice 3: the kernel
// exposes whether a kind was DECLARED (triage > scout) separately from its
// conservative "code" default, so the dispatch-time projection can substitute
// the project default only when no report spoke, while the integrity floor
// keeps reading declarations alone.
func TestRoutingSignals_DeclaredDeliverableKind(t *testing.T) {
	var none RoutingSignals
	if k, ok := none.DeclaredDeliverableKind(); ok || k != "" {
		t.Errorf("undeclared = (%q,%v), want (\"\",false)", k, ok)
	}
	if got := none.DeliverableKind(); got != config.DeliverableKindCode {
		t.Errorf("default = %q, want code", got)
	}
	scoutDoc := RoutingSignals{Scout: ScoutSignals{Present: true, DeliverableKind: config.DeliverableKindDocument}}
	if k, ok := scoutDoc.DeclaredDeliverableKind(); !ok || k != config.DeliverableKindDocument {
		t.Errorf("scout-declared = (%q,%v), want (document,true)", k, ok)
	}
	triageCode := scoutDoc
	triageCode.Triage = TriageSignals{Present: true, DeliverableKind: config.DeliverableKindCode}
	if k, ok := triageCode.DeclaredDeliverableKind(); !ok || k != config.DeliverableKindCode {
		t.Errorf("triage overrides scout = (%q,%v), want (code,true)", k, ok)
	}
	if got := triageCode.DeliverableKind(); got != config.DeliverableKindCode {
		t.Errorf("DeliverableKind must project the same declaration; got %q", got)
	}
}

// TestResolveField_OverlaySignalVocabulary: the overlay `when` selector and the
// kernel's conditional rules name a signal with ONE word — the policy constants
// are the routable field names, so an operator writes `scout.goal_type` in
// both places and it resolves to the same value.
func TestResolveField_OverlaySignalVocabulary(t *testing.T) {
	sig := RoutingSignals{Scout: ScoutSignals{Present: true, GoalType: "partnership-deal", DeliverableKind: config.DeliverableKindDocument}}
	if !evalCondition(sig, config.Condition{Field: config.SignalGoalType, Op: "eq", Value: "partnership-deal"}) {
		t.Errorf("%q must resolve to the scout's declared goal type", config.SignalGoalType)
	}
	if !evalCondition(sig, config.Condition{Field: config.SignalDeliverableKind, Op: "eq", Value: config.DeliverableKindDocument}) {
		t.Errorf("%q must resolve to the projected deliverable kind", config.SignalDeliverableKind)
	}
}
