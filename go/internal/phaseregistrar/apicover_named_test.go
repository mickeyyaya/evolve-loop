package phaseregistrar

import "testing"

func TestResult_NamedType(t *testing.T) {
	r := newRegistrar(t)
	var got Result
	got, err := r.Register(validCfg())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got.Runner == nil {
		t.Fatal("Result.Runner must be non-nil")
	}
	if got.Runner.Name() != "minted-reviewer" {
		t.Errorf("Result.Runner.Name() = %q, want minted-reviewer", got.Runner.Name())
	}
	if got.Spec.Name != "minted-reviewer" {
		t.Errorf("Result.Spec.Name = %q, want minted-reviewer", got.Spec.Name)
	}
	if !got.Spec.Optional {
		t.Error("Result.Spec.Optional must be forced true for a minted phase")
	}
}
