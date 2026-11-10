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
