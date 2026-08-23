package phasespec

// catalog_membership_test.go — PhaseSpec.Catalog decides advisor-MENU membership,
// never whether the phase is installed.
//
// Measured 2026-08-23: 65 non-control phases were projected as advisor SELECT
// cards against 12 enriched slots, so 53 rendered degraded — and 47 of the 65
// had never been selected in 120 cycles. Declining a slot fixes the crowding
// without removing capability; the declined set is still indexed by name in the
// prompt.

import "testing"

func TestPhaseSpecIsOnDemand_OnlyTheExactWordDeclines(t *testing.T) {
	if (PhaseSpec{Name: "market-sizing", Catalog: CatalogOnDemand}).IsOnDemand() != true {
		t.Fatalf("%q must decline the SELECT slot", CatalogOnDemand)
	}
	// The absent key is the default and must keep the phase on the menu — this is
	// what makes the change byte-identical for every phase that says nothing.
	if (PhaseSpec{Name: "scout"}).IsOnDemand() {
		t.Fatalf("an absent catalog key must leave the phase on the menu")
	}
	if (PhaseSpec{Name: "scout", Catalog: CatalogSelect}).IsOnDemand() {
		t.Fatalf("%q (the explicit default) must leave the phase on the menu", CatalogSelect)
	}
	// A near-miss must NOT decline: silently dropping a phase off the advisor's
	// menu because of a typo is worse than the crowding being fixed.
	for _, near := range []string{"ondemand", "on demand", "On-Demand", "ON-DEMAND", " on-demand"} {
		if (PhaseSpec{Name: "x", Catalog: near}).IsOnDemand() {
			t.Fatalf("%q must not be read as a decline — only the exact word counts", near)
		}
	}
}

func TestKnownCatalogWord_AcceptsOnlyTheTwoDefinedWords(t *testing.T) {
	for _, ok := range []string{CatalogSelect, CatalogOnDemand} {
		if !KnownCatalogWord(ok) {
			t.Fatalf("%q is a defined membership word and must be accepted", ok)
		}
	}
	// Every rejected value here is one a human would plausibly write. The
	// repo-catalog guard turns a rejection into a loud authoring-time failure,
	// because an unknown word silently leaves the phase ON the menu — the
	// opposite of what its author intended.
	for _, bad := range []string{"ondemand", "on demand", "On-Demand", "off", "hidden", "none", "true", "menu"} {
		if KnownCatalogWord(bad) {
			t.Fatalf("%q must be rejected: an unrecognized word fails open onto the menu", bad)
		}
	}
}
