package phasespec

// validate_builtin_exempt_test.go — RED contract for cycle-547's
// memo-phase-routing-repair task.
//
// PROBLEM (scout Key Finding 1): the built-in optional `memo` phase can never
// route. Its activation overlay (.evolve/phases/memo/phase.json) is a
// single-word name, and twoTierNameRE (validate.go) rejects every
// single-word name for a discovered overlay with no exemption for names
// that already exist in the built-in catalog — so ApplyUserRouting
// (routing.go) always skips it with a warning and cfg.Order never gets
// "memo" spliced in. ~150 PASS cycles have produced zero memo.md /
// carryover-todos.json artifacts as a result.
//
// FIX CONTRACT (this cycle's new surface — undefined until Builder adds it,
// so this whole package fails to compile today; that compile failure IS the
// RED evidence, mirroring the cycle-465/507 precedent):
//
//   - ValidateUserSpecWithCatalog(s PhaseSpec, builtin Catalog) []string
//     behaves exactly like ValidateUserSpec, EXCEPT: the twoTierNameRE
//     single-word floor is skipped when s.Name matches an existing builtin
//     catalog entry AND that entry's Optional field is true. A name that
//     matches a NON-optional built-in (audit, build, ship, ...) keeps the
//     floor — the exemption is scoped to already-optional built-ins only, so
//     an operator can never hijack a mandatory spine phase's name/slot.
//   - ApplyUserRouting gains a third parameter, `builtin Catalog`, and
//     consults ValidateUserSpecWithCatalog instead of the bare
//     ValidateUserSpec. Every existing call site (cmd_cycle.go's builtinCat,
//     routing_dispatch.go's o.catalog, and this package's own
//     routing_test.go) already has a Catalog in scope at the call site.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestValidateUserSpecWithCatalog_ExemptsOptionalBuiltinName
//     (the memo case itself).
//   - Negative : TestValidateUserSpecWithCatalog_RejectsGenuineNewSingleWordName
//     (a name genuinely absent from the built-in catalog must still fail —
//     the exemption must not degenerate into "any single-word name passes").
//   - Negative (the critical anti-gaming case):
//     TestValidateUserSpecWithCatalog_RejectsNonOptionalBuiltinNameOverlay —
//     an overlay literally named "audit" (a real, non-optional built-in) with
//     Optional:true set on the OVERLAY itself must still be rejected: the
//     exemption looks at the BUILT-IN's Optional flag, not the overlay's,
//     or an operator could hijack the mandatory audit phase's routing slot.
//   - E2E      : TestApplyUserRouting_RoutesBuiltinNameOverlayWithoutWarning
//     drives the real routing splice (cfg.Order/Triggers/PhaseEnable) end to
//     end and asserts zero warnings — the actual PASS-cycle-unblocking
//     behavior, not just the validator in isolation.
//
// ADR-0058 field-stripping (the 4th AC clause: "ADR-0058 field-stripping
// unaffected") is existing, untouched code (DiscoverUserSpecs strips
// on_pass/on_fail/branching_strategy at the real user-file ingestion point,
// unrelated to this fix's seam) — already covered by
// discover_test.go/activating_fields_test.go; no new predicate needed here
// (Step 5: regression coverage, not new work).
import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// memoLikeOverlay mirrors the real .evolve/phases/memo/phase.json overlay:
// single-word name, optional:true, a routing.insert_when trigger, and an
// After anchor.
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

// builtinCatalogFixture mirrors the relevant slice of
// docs/architecture/phase-registry.json: "memo" is optional:true (no routing
// of its own — that lives only in the overlay), "audit" is optional:false
// (a mandatory spine phase).
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
	// An operator names an overlay "audit" and marks the OVERLAY optional:true
	// — but the BUILT-IN "audit" entry is optional:false (a mandatory spine
	// phase). The exemption must consult the built-in's Optional flag, not
	// the overlay's, or this would let an operator hijack audit's name/slot.
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
