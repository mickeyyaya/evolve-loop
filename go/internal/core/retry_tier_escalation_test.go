package core

// retry_tier_escalation_test.go — ADR-0076 slice D pins (adversarial-review
// amended design): the escalation is a deterministic DISPATCH floor —
// mode-independent, raise-only, clamped through the real envelope guardrail
// (single-entry ClampPlanModelRouting — never a second clamp), driven by the
// max failure_count across the cycle's scoped items (lane scope ∪ this
// cycle's processing claims).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func escalationRun(t *testing.T, root, scopeCSV string, reader func(string) int, threshold int) *cycleRun {
	t.Helper()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.failureCountFor = reader
	o.failurePolicy = policy.DefaultSystemFailurePolicy()
	o.failurePolicy.Thresholds.BuildDeepEscalateAtFailures = threshold
	return &cycleRun{
		o:       o,
		cycle:   77,
		req:     CycleRequest{ProjectRoot: root},
		ctxSnap: map[string]string{"fleet_scope": scopeCSV},
	}
}

func TestEscalatedBuildTier_RaisesAtThresholdEnvelopeless(t *testing.T) {
	// No profiles on disk → universal envelope (Max=top) → deep passes.
	cr := escalationRun(t, t.TempDir(), "item-a,item-b", func(id string) int {
		return map[string]int{"item-a": 0, "item-b": 1}[id]
	}, 1)
	tier, raised := cr.escalatedBuildTier("")
	if !raised || tier != "deep" {
		t.Fatalf("max scoped failure_count at threshold must raise to deep, got (%q,%v)", tier, raised)
	}
}

func TestEscalatedBuildTier_BelowThresholdOrDisabled(t *testing.T) {
	cr := escalationRun(t, t.TempDir(), "item-a", func(string) int { return 0 }, 1)
	if _, raised := cr.escalatedBuildTier(""); raised {
		t.Fatal("count below threshold must not raise")
	}
	cr = escalationRun(t, t.TempDir(), "item-a", func(string) int { return 9 }, 0)
	if _, raised := cr.escalatedBuildTier(""); raised {
		t.Fatal("threshold 0 disables (policy escape hatch)")
	}
	cr = escalationRun(t, t.TempDir(), "item-a", nil, 1)
	if _, raised := cr.escalatedBuildTier(""); raised {
		t.Fatal("nil reader (root not wired) must be a no-op")
	}
}

func TestEscalatedBuildTier_RaiseOnlyNeverLowers(t *testing.T) {
	cr := escalationRun(t, t.TempDir(), "item-a", func(string) int { return 5 }, 1)
	if _, raised := cr.escalatedBuildTier("top"); raised {
		t.Fatal("a top proposal must never be lowered")
	}
	if _, raised := cr.escalatedBuildTier("deep"); raised {
		t.Fatal("already at the floor — no raise")
	}
}

func TestEscalatedBuildTier_EnvelopeMaxClampsThroughRealGuardrail(t *testing.T) {
	// A build profile with envelope Max=balanced must pull the raise back to
	// no-gain — via the REAL ClampPlanModelRouting, not a second clamp.
	root := t.TempDir()
	profDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prof := `{"model_tier_envelope":{"min":"fast","default":"balanced","max":"balanced"}}`
	if err := os.WriteFile(filepath.Join(profDir, "builder.json"), []byte(prof), 0o644); err != nil {
		t.Fatal(err)
	}
	cr := escalationRun(t, root, "item-a", func(string) int { return 3 }, 1)
	if tier, raised := cr.escalatedBuildTier("balanced"); raised {
		t.Fatalf("envelope Max=balanced must clamp the raise to no-gain, got %q", tier)
	}
}

func TestEscalationScopeIDs_UnionOfLaneScopeAndProcessingClaims(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-77")
	if err := os.MkdirAll(proc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proc, "x.json"), []byte(`{"id":"claimed-item"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proc, "bad.json"), []byte(`MALFORMED`), 0o644); err != nil {
		t.Fatal(err)
	}
	cr := escalationRun(t, root, "scope-item, scope-item ,", func(string) int { return 0 }, 1)
	ids := cr.escalationScopeIDs()
	want := map[string]bool{"scope-item": true, "claimed-item": true}
	if len(ids) != 2 || !want[ids[0]] || !want[ids[1]] {
		t.Fatalf("want deduped union {scope-item, claimed-item}, got %v", ids)
	}
}

func TestWithFailureCountReader_SetsAndIgnoresNil(t *testing.T) {
	o := &Orchestrator{}
	WithFailureCountReader(func(string) int { return 7 })(o)
	if o.failureCountFor == nil || o.failureCountFor("x") != 7 {
		t.Fatal("reader not injected")
	}
	WithFailureCountReader(nil)(o)
	if o.failureCountFor == nil {
		t.Fatal("nil must be ignored, keeping the prior reader")
	}
}
