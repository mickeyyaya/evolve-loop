package ship

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

type ranCmd struct {
	name string
	dir  string
	args []string
}

func scriptedRun(ran *[]ranCmd, code int, err error) CmdRunner {
	return func(_ context.Context, name, dir string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		*ran = append(*ran, ranCmd{name: name, dir: dir, args: args})
		return code, err
	}
}

func TestGateStepRunner_RunsTheGateCommandInTheGoDirOfTheTree(t *testing.T) {
	t.Parallel()
	var ran []ranCmd
	red, err := GateStepRunner(scriptedRun(&ran, 0, nil))(context.Background(), "/lane", []baseattr.Key{baseattr.StepKey("golangci-lint: Lint"), baseattr.StepKey("make cover-changed")})
	if err != nil || len(red) != 0 {
		t.Fatalf("red = %v, err = %v, want none", red, err)
	}
	want := []ranCmd{
		{name: "golangci-lint", dir: filepath.Join("/lane", "go"), args: []string{"run"}},
		{name: "make", dir: filepath.Join("/lane", "go"), args: []string{"cover-changed"}},
	}
	if len(ran) != 2 || ran[0].name != want[0].name || ran[0].dir != want[0].dir || !slices.Equal(ran[0].args, want[0].args) ||
		ran[1].name != want[1].name || ran[1].dir != want[1].dir || !slices.Equal(ran[1].args, want[1].args) {
		t.Errorf("ran = %+v, want %+v", ran, want)
	}
}

func TestGateStepRunner_AFailingCommandIsRedAndAnUnknownStepIsRedWithoutARun(t *testing.T) {
	t.Parallel()
	var ran []ranCmd
	lint := baseattr.StepKey("golangci-lint run")
	unknown := baseattr.StepKey("Lint")
	red, err := GateStepRunner(scriptedRun(&ran, 1, nil))(context.Background(), "/lane", []baseattr.Key{lint, unknown})
	if err != nil || !slices.Equal(red, []baseattr.Key{lint, unknown}) {
		t.Errorf("red = %v, err = %v, want both red", red, err)
	}
	if len(ran) != 1 {
		t.Errorf("ran = %+v, want one run (the unknown step runs nothing)", ran)
	}
}

func TestGateStepRunner_ACommandThatCannotStartIsAnError(t *testing.T) {
	t.Parallel()
	var ran []ranCmd
	red, err := GateStepRunner(scriptedRun(&ran, -1, errors.New("no golangci-lint")))(context.Background(), "/lane", []baseattr.Key{baseattr.StepKey("golangci-lint run")})
	if err == nil || red != nil {
		t.Errorf("red = %v, err = %v, want an error and no red", red, err)
	}
}
