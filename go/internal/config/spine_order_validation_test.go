package config

import "testing"

// TestValidateSpine_ShipBeforeAuditWarns covers the ADR-0058 S6 spine-order guard:
// the config-driven floor positions anchors by their configured order, so a
// scrambled order placing ship before audit must surface a loud spine-order
// warning (the legality graph + audit verdict branch still block it, but the
// misordering must never go unnoticed). A sane order raises no such warning.
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
