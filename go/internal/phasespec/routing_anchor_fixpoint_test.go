package phasespec

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestApplyUserRouting_AnchorToLaterSpecInBatchIsHonored(t *testing.T) {
	cfg := config.RoutingConfig{Order: []string{"scout", "triage", "tdd", "build", "audit", "ship"}}
	specs := []PhaseSpec{ // alphabetical, exactly as DiscoverUserSpecs sorts them
		{Name: "bug-reproduction", Optional: true, After: "fault-localization"},
		{Name: "fault-localization", Optional: true, After: "triage"},
	}
	warns := ApplyUserRouting(&cfg, specs, Catalog{})
	if len(warns) != 0 {
		t.Fatalf("both anchors resolve within the batch — no warnings expected, got %v", warns)
	}
	want := []string{"scout", "triage", "fault-localization", "bug-reproduction", "tdd", "build", "audit", "ship"}
	if !reflect.DeepEqual(cfg.Order, want) {
		t.Errorf("Order = %v, want %v (bug-reproduction must follow its anchor, not the pre-audit fallback)", cfg.Order, want)
	}
}

func TestApplyUserRouting_AnchorChainResolvesRegardlessOfInputOrder(t *testing.T) {
	cfg := config.RoutingConfig{Order: []string{"scout", "audit", "ship"}}
	specs := []PhaseSpec{
		{Name: "a-third", Optional: true, After: "b-second"},
		{Name: "b-second", Optional: true, After: "c-first"},
		{Name: "c-first", Optional: true, After: "scout"},
	}
	if warns := ApplyUserRouting(&cfg, specs, Catalog{}); len(warns) != 0 {
		t.Fatalf("chain resolves — no warnings expected, got %v", warns)
	}
	want := []string{"scout", "c-first", "b-second", "a-third", "audit", "ship"}
	if !reflect.DeepEqual(cfg.Order, want) {
		t.Errorf("Order = %v, want %v", cfg.Order, want)
	}
}

func TestApplyUserRouting_UnresolvableAnchorFallsBackWithWarning(t *testing.T) {
	cfg := config.RoutingConfig{Order: []string{"scout", "build", "audit", "ship"}}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{{Name: "orphan-check", Optional: true, After: "no-such-phase"}}, Catalog{})
	want := []string{"scout", "build", "orphan-check", "audit", "ship"}
	if !reflect.DeepEqual(cfg.Order, want) {
		t.Errorf("Order = %v, want %v (fallback placement preserved)", cfg.Order, want)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "no-such-phase") {
		t.Errorf("warnings = %v, want exactly one naming the unresolved anchor", warns)
	}
}

func TestApplyUserRouting_AnchorCycleTerminatesWithOneWarning(t *testing.T) {
	cfg := config.RoutingConfig{Order: []string{"scout", "audit", "ship"}}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{
		{Name: "ouro-a", Optional: true, After: "ouro-b"},
		{Name: "ouro-b", Optional: true, After: "ouro-a"},
	}, Catalog{})
	if len(warns) != 1 || !strings.Contains(warns[0], "anchor cycle") {
		t.Fatalf("warnings = %v, want exactly one naming the broken cycle link", warns)
	}
	ai, bi := indexOfStr(cfg.Order, "ouro-a"), indexOfStr(cfg.Order, "ouro-b")
	if ai < 0 || bi < 0 {
		t.Fatalf("both cycle members must be placed; order=%v", cfg.Order)
	}
	if bi < ai {
		t.Errorf("ouro-b (after: ouro-a) at %d precedes its anchor at %d — the survivor must follow the force-placed member", bi, ai)
	}
}

func TestApplyUserRouting_CycleTailStillFollowsItsAnchor(t *testing.T) {
	cfg := config.RoutingConfig{Order: []string{"scout", "audit", "ship"}}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{
		{Name: "a-tail", Optional: true, After: "ouro-a"}, // tail FIRST in input: the batch-order hazard
		{Name: "ouro-a", Optional: true, After: "ouro-b"},
		{Name: "ouro-b", Optional: true, After: "ouro-a"},
	}, Catalog{})
	if len(warns) != 1 {
		t.Fatalf("warnings = %v, want exactly one (the broken cycle link only)", warns)
	}
	ti, ai := indexOfStr(cfg.Order, "a-tail"), indexOfStr(cfg.Order, "ouro-a")
	if ti < 0 || ai < 0 {
		t.Fatalf("all specs must be placed; order=%v", cfg.Order)
	}
	if ti < ai {
		t.Errorf("a-tail (after: ouro-a) at %d precedes its anchor at %d — batch-order burst regression", ti, ai)
	}
}

func TestApplyUserRouting_AlreadyPresentSpecWithAbsentAnchorIsSilent(t *testing.T) {
	cfg := config.RoutingConfig{Order: []string{"scout", "already-here", "audit", "ship"}}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{
		{Name: "already-here", Optional: true, After: "no-such-anchor"},
	}, Catalog{})
	if len(warns) != 0 {
		t.Errorf("warnings = %v, want none — the spec did not move", warns)
	}
	want := []string{"scout", "already-here", "audit", "ship"}
	if !reflect.DeepEqual(cfg.Order, want) {
		t.Errorf("Order = %v, want unchanged %v", cfg.Order, want)
	}
}

func TestApplyUserRouting_TransitivelyBlockedSpecStillFollowsItsAnchor(t *testing.T) {
	cfg := config.RoutingConfig{Order: []string{"scout", "audit", "ship"}}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{
		{Name: "a-dependent", Optional: true, After: "b-broken"},
		{Name: "b-broken", Optional: true, After: "typo-anchor-does-not-exist"},
	}, Catalog{})
	want := []string{"scout", "b-broken", "a-dependent", "audit", "ship"}
	if !reflect.DeepEqual(cfg.Order, want) {
		t.Errorf("Order = %v, want %v — the dependent must follow its force-placed anchor", cfg.Order, want)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "typo-anchor-does-not-exist") {
		t.Errorf("warns = %v, want exactly one, for the truly-broken anchor only", warns)
	}
}
