package phaseregistrar

import "testing"

// TestResult_NamedType names the phaseregistrar.Result struct (Register returns
// it but the bare type is never named in a test) and pins the two-field
// contract: Spec is the normalized (forced-Optional) spec the caller splices
// into the catalog, and Runner is a live core.PhaseRunner reporting the spec name.
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
