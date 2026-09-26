package config

import "testing"

func TestValidateSpine_ShipBeforeAuditWarns(t *testing.T) {
	t.Parallel()

	var scrambled []Warning
	validateSpine(RoutingConfig{
		Mandatory: []string{"scout", "build", "audit", "ship"},
		Order:     []string{"scout", "build", "ship", "audit"}, // ship before audit
	}, &scrambled)
	if !hasWarning(scrambled, "spine-order") {
		t.Errorf("expected spine-order warning when ship precedes audit; got %v", scrambled)
	}

	var sane []Warning
	validateSpine(RoutingConfig{
		Mandatory: []string{"scout", "build", "audit", "ship"},
		Order:     []string{"scout", "build", "audit", "ship"},
	}, &sane)
	if hasWarning(sane, "spine-order") {
		t.Errorf("a sane audit→ship order must not warn; got %v", sane)
	}
}
