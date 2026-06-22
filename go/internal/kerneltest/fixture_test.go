package kerneltest

import "testing"

// TestFixture_LoadsAndResolvesStructurally exercises the fixture against the real
// registry and asserts the structural accessors resolve sensibly — without
// hardcoding any phase name (the whole point of the fixture). It also covers the
// public API for the apicover gate.
func TestFixture_LoadsAndResolvesStructurally(t *testing.T) {
	t.Parallel()
	var f *Fixture = Load(t)

	if len(f.Catalog.Names()) == 0 {
		t.Fatal("fixture catalog must be non-empty")
	}
	if len(f.Mandatory()) == 0 {
		t.Fatal("reference flow must declare mandatory anchors")
	}
	if len(f.Spine()) < 2 {
		t.Fatal("reference flow must declare a multi-phase spine")
	}

	// First anchor and ship terminal are the ends of the mandatory chain.
	m := f.Mandatory()
	if f.FirstAnchor() != m[0] {
		t.Errorf("FirstAnchor = %q, want %q", f.FirstAnchor(), m[0])
	}
	if f.ShipTerminal() != m[len(m)-1] {
		t.Errorf("ShipTerminal = %q, want %q", f.ShipTerminal(), m[len(m)-1])
	}
	if f.SpineEntry() != f.Spine()[0] {
		t.Errorf("SpineEntry = %q, want %q", f.SpineEntry(), f.Spine()[0])
	}

	// The evaluator is a mandatory phase that declares a verdict branch.
	eval := f.Evaluator()
	if eval == "" {
		t.Fatal("reference flow must declare a verdict-branching evaluator (on_pass/on_fail)")
	}
	spec, ok := f.Catalog.Get(eval)
	if !ok || spec.OnPass == "" || spec.OnFail == "" {
		t.Errorf("Evaluator %q must declare on_pass+on_fail; got %+v", eval, spec)
	}
	if f.Config.Stage == 0 && len(f.Config.Order) == 0 {
		t.Error("fixture Config must be populated from the registry")
	}
}
