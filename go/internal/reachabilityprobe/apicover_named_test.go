package reachabilityprobe

// apicover_named_test.go — repo-wide apicover public-API coverage (House
// Rule 1 / ADR-0069's second gate): names and exercises every exported symbol
// of this brand-new package (ImportGraph, CallSite, Violation, CheckCallSite)
// by identifier, in the same diff that graduates it into go/.apicover-enforce.

import "testing"

// TestExportedSymbols_Named names every exported identifier of this package
// and pins the two load-bearing contracts: CheckCallSite detects the
// cycle-644 shape (non-nil Violation with a populated Cycle and Error()) and
// leaves an acyclic pin unchanged (nil).
func TestExportedSymbols_Named(t *testing.T) {
	var graph ImportGraph = ImportGraph{
		"storage": {"core"},
		"core":    {},
	}
	cyclic := CallSite{PinningPackage: "core", ReferencedPackage: "storage", Symbol: "UpdateStateMap"}

	var v *Violation = CheckCallSite(graph, cyclic)
	if v == nil {
		t.Fatalf("CheckCallSite(%+v, %+v) = nil, want a *Violation (cycle-644 shape)", graph, cyclic)
	}
	if v.Site != cyclic {
		t.Errorf("Violation.Site = %+v, want %+v", v.Site, cyclic)
	}
	if len(v.Cycle) == 0 {
		t.Error("Violation.Cycle is empty, want a non-empty import chain")
	}
	if v.Error() == "" {
		t.Error("Violation.Error() returned empty string, want a diagnostic message")
	}

	acyclic := CallSite{PinningPackage: "leaf", ReferencedPackage: "storage", Symbol: "UpdateStateMap"}
	if got := CheckCallSite(graph, acyclic); got != nil {
		t.Errorf("CheckCallSite(%+v, %+v) = %+v, want nil (leaf is absent from graph)", graph, acyclic, got)
	}
}
