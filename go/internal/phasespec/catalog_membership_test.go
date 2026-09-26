package phasespec

import "testing"

func TestPhaseSpecIsOnDemand_OnlyTheExactWordDeclines(t *testing.T) {
	if (PhaseSpec{Name: "market-sizing", Catalog: CatalogOnDemand}).IsOnDemand() != true {
		t.Fatalf("%q must decline the SELECT slot", CatalogOnDemand)
	}
	if (PhaseSpec{Name: "scout"}).IsOnDemand() {
		t.Fatalf("an absent catalog key must leave the phase on the menu")
	}
	if (PhaseSpec{Name: "scout", Catalog: CatalogSelect}).IsOnDemand() {
		t.Fatalf("%q (the explicit default) must leave the phase on the menu", CatalogSelect)
	}
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
	for _, bad := range []string{"ondemand", "on demand", "On-Demand", "off", "hidden", "none", "true", "menu"} {
		if KnownCatalogWord(bad) {
			t.Fatalf("%q must be rejected: an unrecognized word fails open onto the menu", bad)
		}
	}
}
