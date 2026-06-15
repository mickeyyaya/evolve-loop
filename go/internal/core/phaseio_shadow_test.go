package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
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
