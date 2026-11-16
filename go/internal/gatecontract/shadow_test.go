package gatecontract

import (
	"context"
	"testing"
)

func TestEffectiveStage_ReadsThePolicyKeyOfAShadowEntryAndTheSeedOtherwise(t *testing.T) {
	t.Parallel()
	policyStage := func(key string) string { return map[string]string{"a.stage": StageEnforce}[key] }
	withShadow := Entry{Name: "a", Shadow: &Shadow{PolicyKey: "a.stage"}}
	if got := withShadow.EffectiveStage(policyStage); got != StageEnforce {
		t.Errorf("shadow entry stage = %q, want %q", got, StageEnforce)
	}
	seeded := Entry{Name: "b", Stage: StageShadow}
	if got := seeded.EffectiveStage(policyStage); got != StageShadow {
		t.Errorf("seed entry stage = %q, want %q", got, StageShadow)
	}
	keyless := Entry{Name: "c", Stage: StageEnforce, Shadow: &Shadow{}}
	if got := keyless.EffectiveStage(policyStage); got != StageEnforce {
		t.Errorf("keyless shadow stage = %q, want %q", got, StageEnforce)
	}
}

func TestShadowTypes_CarryTheDescriptorAsTypedData(t *testing.T) {
	t.Parallel()
	shadow := Shadow{
		PolicyKey:     "x.stage",
		Signal:        "X_SHADOW",
		Fields:        []string{"would_stop"},
		EmitSites:     []EmitSite{{Path: "p.go", Func: "emit", Ref: "CodeX"}},
		Promotion:     Promotion{MaxEscapes: 1, MaxFalseStops: 2, MinDecisions: 3, WindowCycles: 4, MaxFalseStopPct: 5},
		Closure:       []string{"internal/x"},
		PolicySubtree: "x",
	}
	shadow.Decide = func(_ context.Context, in StageInput) (Decision, error) {
		return Decision{Fields: in.Fields}, nil
	}
	shadow.Enforce = func(context.Context, StageInput, Decision) error { return nil }
	shadow.Score = func(_ context.Context, recs []EvidenceRecord) (StageVerdict, error) {
		return StageVerdict{Ready: len(recs) > 0, Decisions: len(recs)}, nil
	}
	got, err := shadow.Decide(context.Background(), StageInput{Fields: map[string]string{"a": "b"}})
	if err != nil || got.Fields["a"] != "b" {
		t.Fatalf("Decide = %+v, %v", got, err)
	}
	if err := shadow.Enforce(context.Background(), StageInput{}, got); err != nil {
		t.Fatalf("Enforce = %v", err)
	}
	verdict, err := shadow.Score(context.Background(), []EvidenceRecord{{Key: "x.stage", Cycle: 1, Seq: 1}})
	if err != nil || !verdict.Ready || verdict.Decisions != 1 {
		t.Fatalf("Score = %+v, %v", verdict, err)
	}
}
