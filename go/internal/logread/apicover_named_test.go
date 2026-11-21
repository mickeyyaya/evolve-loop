package logread

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestAPI_BudgetsLayersAndErrorAreNamed(t *testing.T) {
	t.Parallel()
	if BudgetD0 != 40 || BudgetD1 != 200 || BudgetD2 != 200 || BudgetD3 != 150 || MaxLineBytes != 1024 {
		t.Error("the budgets are the plan values of section 6.2")
	}
	for l, want := range map[Layer]string{D0: "D0", D1: "D1", D2: "D2", D3: "D3"} {
		if string(l) != want {
			t.Errorf("layer %q is spelled %q", l, want)
		}
	}
	if !errors.Is(fmt.Errorf("wrapped: %w", ErrNoLog), ErrNoLog) {
		t.Error("ErrNoLog is a sentinel for errors.Is")
	}
}

func TestAPI_ReaderQueryAndVerbsAreNamed(t *testing.T) {
	t.Parallel()
	r := Reader{RunDir: t.TempDir(), Cycle: 3, Signals: signalcenter.New()}
	q := Query{Phase: "build", Kind: "phase.outcome", Code: "X_Y", Level: "WARN", From: 1}
	if _, err := r.Events(q); err == nil {
		t.Error("Events of a cycle without a stream must fail, not print nothing")
	}
	if _, err := r.Failures(q); err == nil {
		t.Error("Failures of a cycle without a stream must fail")
	}
	if got, err := r.Digest(); err != nil || len(got) != 1 {
		t.Errorf("Digest = %v, %v", got, err)
	}
	if got, err := r.Index(1); err != nil || len(got) != 0 {
		t.Errorf("Index of an empty dir = %v, %v", got, err)
	}
	if _, err := r.Show("build", D2, 1); err == nil {
		t.Error("Show of a missing file must fail")
	}
	if got := r.DispatchBlock(); len(got) != 2 {
		t.Errorf("DispatchBlock = %v", got)
	}
	if r.RecordShadow("build", nil, D3, "show") {
		t.Error("RecordShadow of a phase without log_access must do nothing")
	}
	if l, ok := ParseLayer("D1"); !ok || l != D1 {
		t.Errorf("ParseLayer = %q, %v", l, ok)
	}
}
