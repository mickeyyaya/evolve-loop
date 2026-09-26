package phasespec

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// memoLikeOverlay mirrors the real .evolve/phases/memo/phase.json activation overlay.
func memoLikeOverlay() PhaseSpec {
	return PhaseSpec{
		Name:     "memo",
		Optional: true,
		After:    "ship",
		Routing: &config.RoutingBlock{
			InsertWhen: []config.Condition{{Field: "ship.class", Op: "==", Value: "cycle"}},
		},
	}
}

// builtinCatalogFixture mirrors the registry: "memo" is optional with no routing of its own, "audit" is mandatory.
func builtinCatalogFixture() Catalog {
	return Catalog{
		order: []string{"audit", "memo"},
		byName: map[string]PhaseSpec{
			"audit": {Name: "audit", Optional: false},
			"memo":  {Name: "memo", Optional: true},
		},
	}
}

func TestValidateUserSpecWithCatalog_ExemptsOptionalBuiltinName(t *testing.T) {
	v := ValidateUserSpecWithCatalog(memoLikeOverlay(), builtinCatalogFixture())
	if len(v) != 0 {
		t.Fatalf("memo overlay (single-word, matches an optional built-in) rejected: %v — want zero violations", v)
	}
}

func TestValidateUserSpecWithCatalog_RejectsGenuineNewSingleWordName(t *testing.T) {
	widget := PhaseSpec{Name: "widget", Optional: true}
	v := ValidateUserSpecWithCatalog(widget, builtinCatalogFixture())
	if len(v) == 0 {
		t.Fatalf("a genuinely new single-word name absent from the built-in catalog was accepted — the two-tier floor must still apply")
	}
}

func TestValidateUserSpecWithCatalog_RejectsNonOptionalBuiltinNameOverlay(t *testing.T) {
	hijack := PhaseSpec{Name: "audit", Optional: true}
	v := ValidateUserSpecWithCatalog(hijack, builtinCatalogFixture())
	if len(v) == 0 {
		t.Fatalf("an overlay named after a NON-optional built-in (audit) was accepted — must stay rejected regardless of the overlay's own optional:true")
	}
}

func TestApplyUserRouting_RoutesBuiltinNameOverlayWithoutWarning(t *testing.T) {
	cfg := config.RoutingConfig{
		Order:       []string{"scout", "build", "audit", "ship"},
		Triggers:    map[string]config.RoutingBlock{},
		PhaseEnable: map[string]config.Enable{},
	}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{memoLikeOverlay()}, builtinCatalogFixture())
	if len(warns) != 0 {
		t.Fatalf("memo overlay routing produced warnings: %v — want zero (the exact PASS-cycle-learning-capture blocker)", warns)
	}
	found := false
	for _, name := range cfg.Order {
		if name == "memo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cfg.Order = %v — memo was never spliced in", cfg.Order)
	}
	if _, ok := cfg.Triggers["memo"]; !ok {
		t.Error("memo's routing.insert_when trigger was never registered")
	}
}
