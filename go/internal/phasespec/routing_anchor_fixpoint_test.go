package phasespec

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// Cycle-1550 (soak-20260824a): DiscoverUserSpecs hands ApplyUserRouting an
// alphabetically-sorted batch, and a spec anchored to an alphabetically-LATER
// spec found its anchor absent from cfg.Order at splice time — spliceAfter
// silently took the before-audit fallback, so bug-reproduction (a red-first
// Evaluate phase planned at index 3, pre-build) executed EIGHTH, post-build,
// where its deliberately-failing deliverable can only red the lane. Placement
// must therefore be a fixpoint over the batch: a spec waits for its anchor as
// long as any pass still makes progress.
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

// A chain of anchors in fully-reversed input order needs one pass per link —
// the fixpoint must keep iterating while progress is made, not stop after two.
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

// An anchor that never resolves must still place the phase (before audit, the
// long-standing fallback) but LOUDLY: silent fallback is what hid cycle-1550's
// mis-slotting for the life of the catalog.
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

// Mutually-anchored specs can never all resolve by anchor. The deadlock break
// force-places exactly ONE member (warned); the rest then resolve after it on
// later passes — so only the broken link is loud and every survivor still
// follows its declared anchor.
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

// Design-review note: a spec anchored to a cycle MEMBER without being on the
// cycle (a tail) must not be batch-order force-placed ahead of its anchor —
// the deadlock break picks a dependency TARGET, so tails resolve honorably.
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

// An activation overlay whose name is ALREADY in the order must stay silent
// even with an absent anchor: nothing moves, so no warning may claim otherwise.
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

// go-reviewer MEDIUM (this PR's review): a spec whose anchor is itself an
// unresolvable batch-mate is only TRANSITIVELY blocked — after the broken
// anchor force-places, the dependent must still land AFTER it (its declared
// order), never before it in batch order, and without its own warning.
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
