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
		Run: func(context.Context, Input, Ports) (Verdict, error) {
			return Verdict{Reds: []string{"r"}, Notes: []string{"n"}}, nil
		},
	}
	verdict, err := entry.Run(context.Background(), Input{Root: "r", Base: "b"}, Ports{})
	if err != nil || len(verdict.Reds) != 1 || len(verdict.Notes) != 1 {
		t.Fatalf("Run = %+v, %v", verdict, err)
	}
	if entry.Stage != StageEnforce || StageShadow == StageEnforce || KindPhaseFloor == KindShipCheck {
		t.Error("the stage and kind words are not distinct")
	}
}

type fakeLane struct{}

func (fakeLane) TrackedFiles(string, string) ([]string, error) { return []string{"f"}, nil }
func (fakeLane) TrackedTree(string, string) ([]string, error)  { return []string{"t"}, nil }
func (fakeLane) TrackedSet(string, string, string) (map[string]bool, error) {
	return map[string]bool{"s": true}, nil
}

type fakeToolchain struct{}

func (fakeToolchain) List(context.Context, Call) (Output, error) {
	return Output{Stdout: []byte("l")}, nil
}
func (fakeToolchain) Test(context.Context, Call) (Output, error) { return Output{ExitCode: 1}, nil }

func TestPorts_HoldALaneAndAToolchainThatAnEntryRunCalls(t *testing.T) {
	t.Parallel()
	ports := Ports{Lane: fakeLane{}, Toolchain: fakeToolchain{}}
	files, err := ports.Lane.TrackedFiles("r", "d")
	tree, _ := ports.Lane.TrackedTree("r", "d")
	set, _ := ports.Lane.TrackedSet("r", "d", ".go")
	if err != nil || files[0] != "f" || tree[0] != "t" || !set["s"] {
		t.Fatalf("Lane = %v %v %v %v", files, tree, set, err)
	}
	listed, _ := ports.Toolchain.List(context.Background(), Call{Dir: "d", Env: []string{"e"}, Args: []string{"a"}})
	tested, _ := ports.Toolchain.Test(context.Background(), Call{})
	if string(listed.Stdout) != "l" || tested.ExitCode != 1 || len(listed.Stderr) != 0 {
		t.Errorf("Toolchain = %+v %+v", listed, tested)
	}
}
