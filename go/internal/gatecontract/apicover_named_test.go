package gatecontract

import (
	"context"
	"testing"
)

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	t.Parallel()
	entry := Entry{
		Name:   "x",
		Kind:   KindScanner,
		Files:  []string{"f"},
		Covers: []string{"c"},
		Owners: []Owner{{Pattern: "p", Phase: "build"}},
		Inputs: []Declared{{Path: "a", Class: "subject"}},
		Stage:  StageEnforce,
		Run: func(context.Context, Input) (Verdict, error) {
			return Verdict{Reds: []string{"r"}, Notes: []string{"n"}}, nil
		},
	}
	verdict, err := entry.Run(context.Background(), Input{Root: "r", Base: "b"})
	if err != nil || len(verdict.Reds) != 1 || len(verdict.Notes) != 1 {
		t.Fatalf("Run = %+v, %v", verdict, err)
	}
	if entry.Stage != StageEnforce || StageShadow == StageEnforce || KindPhaseFloor == KindShipCheck {
		t.Error("the stage and kind words are not distinct")
	}
}

func TestAPICover_NamesTheShadowDescriptor(t *testing.T) {
	t.Parallel()
	entry := Entry{Name: "s", Kind: KindStageFeature, Shadow: &Shadow{PolicyKey: "s.stage", Promotion: Promotion{MinDecisions: 1}, EmitSites: []EmitSite{{Path: "p", Func: "f", Ref: "r"}}}}
	input, decision := StageInput{Root: "r", Cycle: 1}, Decision{Fields: map[string]string{}}
	record, verdict := EvidenceRecord{Key: "k", Cycle: 1, Seq: 1}, StageVerdict{Ready: true, FalseStops: 1, Escapes: 1, Conflicts: 1}
	if entry.EffectiveStage(func(string) string { return StageShadow }) != StageShadow || input.Cycle+record.Seq != 2 || decision.Fields == nil || !verdict.Ready {
		t.Error("the shadow descriptor did not round-trip")
	}
}
