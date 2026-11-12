package signalcenter

import (
	"strings"
	"testing"
)

var convergenceCodesBesideRung = []Code{
	"CONVERGENCE_NO_PROGRESS",
	"CONVERGENCE_REPAIR_DAMAGE",
	"CONVERGENCE_CONCENTRATION",
	"CONVERGENCE_DEFERRED",
	"CONVERGENCE_FILED",
	"CONVERGENCE_SPLIT",
	"CONVERGENCE_ACCEPTED_LIMITS",
	"CONVERGENCE_STOP",
	"CONVERGENCE_DISPUTED",
	"CONVERGENCE_NO_HEADROOM",
}

func TestRegistry_ConvergenceCodesAreOwnedByTheConvergenceModuleWithADoc(t *testing.T) {
	docs := map[Code]string{}
	for _, cd := range RegisteredCodes()[ModuleConvergence] {
		docs[cd.Code] = cd.Doc
	}
	for _, c := range convergenceCodesBesideRung {
		owner, ok := IsRegistered(c)
		if !ok || owner != ModuleConvergence {
			t.Errorf("%s: owner = %q, registered = %v, want %q", c, owner, ok, ModuleConvergence)
		}
		if strings.TrimSpace(docs[c]) == "" {
			t.Errorf("%s has no doc in the convergence module", c)
		}
	}
}

func TestRegistry_ConvergenceDocsNameTheFieldsOfTheSpecTable(t *testing.T) {
	docs := map[Code]string{}
	for _, cd := range RegisteredCodes()[ModuleConvergence] {
		docs[cd.Code] = cd.Doc
	}
	wantFields := map[Code][]string{
		"CONVERGENCE_NO_PROGRESS":     {"mass_prev", "mass", "bar"},
		"CONVERGENCE_REPAIR_DAMAGE":   {"damage", "repairs"},
		"CONVERGENCE_CONCENTRATION":   {"component", "share", "window"},
		"CONVERGENCE_DEFERRED":        {"count", "followups"},
		"CONVERGENCE_FILED":           {"late", "capability"},
		"CONVERGENCE_SPLIT":           {"component", "followup"},
		"CONVERGENCE_ACCEPTED_LIMITS": {"component", "certificate"},
		"CONVERGENCE_STOP":            {"round", "open", "land_round"},
		"CONVERGENCE_DISPUTED":        {"count", "critical", "adjudicator"},
		"CONVERGENCE_NO_HEADROOM":     {"family", "from", "to"},
	}
	for code, fields := range wantFields {
		for _, f := range fields {
			if !strings.Contains(docs[code], f) {
				t.Errorf("%s doc %q does not name field %q", code, docs[code], f)
			}
		}
	}
}

func TestRegistry_ConvergenceCodesAddNoConflict(t *testing.T) {
	if got := RegistryConflicts(); len(got) != 0 {
		t.Fatalf("registry conflicts: %+v", got)
	}
}
