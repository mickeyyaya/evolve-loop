package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
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
	t.Parallel()

	cr := escalationRun(t, t.TempDir(), "item-a,item-b", func(id string) int {
		return map[string]int{"item-a": 0, "item-b": 1}[id]
	}, 1)
	tier, raised := cr.escalatedBuildTier("")
	if !raised || tier != "deep" {
		t.Fatalf("max scoped failure_count at threshold must raise to deep, got (%q,%v)", tier, raised)
	}
}

func TestEscalatedBuildTier_BelowThresholdOrDisabled(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	cr := escalationRun(t, t.TempDir(), "item-a", func(string) int { return 5 }, 1)
	if _, raised := cr.escalatedBuildTier("top"); raised {
		t.Fatal("a top proposal must never be lowered")
	}
	if _, raised := cr.escalatedBuildTier("deep"); raised {
		t.Fatal("already at the floor — no raise")
	}
}

func TestEscalatedBuildTier_EnvelopeMaxClampsThroughRealGuardrail(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

func TestComposeBuildTier_TheHigherOfTheLadderTargetAndTheFloor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, target, floor, want string }{
		{"target below the floor", "balanced", "deep", "deep"},
		{"target above the floor", "top", "deep", "top"},
		{"floor absent", "balanced", "", "balanced"},
		{"both absent", "", "", ""},
	} {
		if got := composeBuildTier(tc.target, tc.floor); got != tc.want {
			t.Errorf("%s: composeBuildTier(%q,%q) = %q, want %q", tc.name, tc.target, tc.floor, got, tc.want)
		}
	}
}

func TestWorstScopedClass_CapabilityWinsThenTheFirstClass(t *testing.T) {
	t.Parallel()
	read := func(id string) string {
		return map[string]string{"a": "spec", "b": "capability", "c": "", "d": "unknown"}[id]
	}
	if got := worstScopedClass(read, []string{"a", "d"}); got != retryladder.Spec {
		t.Fatalf("got %q, want the first class spec", got)
	}
	if got := worstScopedClass(read, []string{"c", "a", "b"}); got != retryladder.Capability {
		t.Fatalf("got %q, want capability", got)
	}
	if got := worstScopedClass(read, []string{"c", "a"}); got != retryladder.Spec {
		t.Fatalf("got %q, want spec", got)
	}
	if got := worstScopedClass(nil, []string{"a"}); got != "" {
		t.Fatalf("a nil reader must give no class, got %q", got)
	}
}

func TestEscalatedBuildTier_TheLadderTargetFeedsTheDispatchedTierOnlyAtEnforce(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, stage string
		wantRaised  bool
	}{
		{"enforce lets the ladder target raise the tier", policy.RetryLadderEnforce, true},
		{"shadow records the ladder and never moves the tier", policy.RetryLadderShadow, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeCLIProfile(t, root, "builder", "agy-tmux", nil)
			cr := escalationRun(t, root, "item-a", func(string) int { return 1 }, 2)
			cr.o.workflowConfig = ladderWorkflow(tc.stage)
			cr.o.failureClassFor = func(string) string { return string(retryladder.Unknown) }
			base := cr.ladderPlan(PhaseBuild).Model
			if base == "" || base == "deep" {
				t.Fatalf("the routed base tier %q cannot show a ladder-only raise", base)
			}
			tier, raised := cr.escalatedBuildTier("")
			if raised != tc.wantRaised || (raised && tier != base) {
				t.Fatalf("tier = (%q,%v), want raised=%v at the ladder target %q", tier, raised, tc.wantRaised, base)
			}
		})
	}
}

func TestEscalatedBuildTier_TheFloorSignalNamesTheFloorOnly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		threshold int
		wantLine  bool
	}{
		{"the floor applies and prints", 1, true},
		{"a ladder-only raise prints no floor provenance", 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeCLIProfile(t, root, "builder", "agy-tmux", nil)
			var out strings.Builder
			cr := escalationRun(t, root, "item-a", func(string) int { return 1 }, tc.threshold)
			cr.o.stderr = &out
			cr.o.workflowConfig = ladderWorkflow(policy.RetryLadderEnforce)
			cr.o.failureClassFor = func(string) string { return string(retryladder.Unknown) }
			if _, raised := cr.escalatedBuildTier(""); !raised {
				t.Fatal("the ladder or the floor must raise")
			}
			if got := strings.Contains(out.String(), "deterministic floor"); got != tc.wantLine {
				t.Fatalf("floor line printed = %v, want %v: %q", got, tc.wantLine, out.String())
			}
		})
	}
}

func TestNextItemDecision_IsAQueryThatLeavesTheCycleStateAlone(t *testing.T) {
	t.Parallel()
	center, events := captureSignals()
	cr := escalationRun(t, t.TempDir(), "item-a", func(string) int { return 1 }, 1)
	cr.o.workflowConfig = ladderWorkflow(policy.RetryLadderEnforce)
	cr.o.signals = center
	cr.o.failureClassFor = func(string) string { return string(retryladder.Capability) }
	d, verdict := cr.nextItemDecision(retryladder.Capability)
	if verdict.Class != retryladder.Capability || d.Action != retryladder.ActionRaise {
		t.Fatalf("query = %+v %+v", d, verdict)
	}
	if cr.cs.Ladder != nil || len(eventsWithCode(*events, CodeLadderDecision)) != 0 {
		t.Fatalf("the query wrote state or emitted: %+v", cr.cs.Ladder)
	}
	cr.recordStartRung(d, verdict)
	stored := cr.cs.Ladder
	cr.recordStartRung(d, verdict)
	if stored == nil || cr.cs.Ladder != stored || len(eventsWithCode(*events, CodeLadderDecision)) != 1 {
		t.Fatalf("the start rung must be recorded exactly once")
	}
}
