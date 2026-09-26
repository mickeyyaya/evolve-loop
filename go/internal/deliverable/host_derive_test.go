package deliverable

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

const triageReport = "# Triage Decision — Cycle 7\n\ncycle_size_estimate: small\ndeliverable_kind: code\nphase_skip: []\n\n" +
	"## top_n (this cycle)\n- lineage-datestamp-normalization: extend LineageKey\n\n## deferred\n(none)\n\n## dropped\n(none)\n"

// derivingCatalog declares triage's decision as derived from its report, as the registry does.
func derivingCatalog(t *testing.T) phasespec.Catalog {
	t.Helper()
	cat, warnings := (phasespec.Catalog{}).Merge([]phasespec.PhaseSpec{{
		Name: "triage", Role: "triage",
		Outputs: phasespec.IO{
			Files:       []string{".evolve/runs/cycle-{cycle}/triage-report.md", ".evolve/runs/cycle-{cycle}/triage-decision.json"},
			AgentOwed:   []string{"triage-decision.json"},
			DerivedFrom: map[string]string{"triage-decision.json": "triage-report.md"},
		},
	}})
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	return cat
}

func performTriage(t *testing.T, report, pin, decision string) (string, error) {
	t.Helper()
	in := triageInput(t, "/project", pin, decision)
	if report != "" {
		writeFile(t, in.Workspace, "triage-report.md", report)
	}
	var calls []claimCall
	err := NewHostEffects(derivingCatalog(t), recordingClaimer(&calls)).Perform(context.Background(), in)
	return filepath.Join(in.Workspace, "triage-decision.json"), err
}

func TestHostEffects_DerivesTheDecisionTheAgentLeftAbsent(t *testing.T) {
	path, err := performTriage(t, triageReport, "", "")

	if err != nil {
		t.Fatalf("Perform: %v", err)
	}
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("the decision was not derived: %v", rerr)
	}
	var doc struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
		Projected bool `json:"projected_by_orchestrator"`
	}
	if json.Unmarshal(raw, &doc) != nil || len(doc.TopN) != 1 || doc.TopN[0].ID != "lineage-datestamp-normalization" || !doc.Projected {
		t.Fatalf("derived decision = %s", raw)
	}
}

func TestHostEffects_NeverTouchesADecisionTheAgentWrote(t *testing.T) {
	const written = `{"top_n":[{"id":"chosen-by-the-agent"}]}`

	path, err := performTriage(t, triageReport, "", written)

	if err != nil {
		t.Fatalf("Perform: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != written {
		t.Fatalf("the agent's decision was rewritten: %s", got)
	}
}

func TestHostEffects_ADeclinedDerivationIsLoudAndLeavesTheFileAbsent(t *testing.T) {
	for name, tc := range map[string]struct{ report, pin string }{
		"a report without its buckets":              {report: "# Triage\n\nnothing here\n"},
		"a pinned item the report does not include": {report: triageReport, pin: `{"todo_ids":["other-item"],"goal_hash":"g"}`},
		"no report at all":                          {},
	} {
		t.Run(name, func(t *testing.T) {
			path, err := performTriage(t, tc.report, tc.pin, "")

			if err == nil || !strings.Contains(err.Error(), "derive triage-decision.json") {
				t.Fatalf("want a loud decline, got %v", err)
			}
			if _, serr := os.Stat(path); !os.IsNotExist(serr) {
				t.Fatalf("a declined derivation must leave the gate its absence, stat err = %v", serr)
			}
		})
	}
}

// The registry, not Go, declares what is derivable, and every declaration has a deriver behind it.
func TestHostEffects_TheRegistryDeclaresTriagesDerivation(t *testing.T) {
	cat, err := phasespec.Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Skipf("real registry not reachable: %v", err)
	}
	spec, ok := cat.Get("triage")
	if !ok || spec.Outputs.DerivedFrom["triage-decision.json"] != "triage-report.md" {
		t.Fatalf("triage outputs.derived_from = %v", spec.Outputs.DerivedFrom)
	}
	for owed := range spec.Outputs.DerivedFrom {
		if derivers[owed] == nil {
			t.Errorf("%s is declared derivable but no deriver is registered", owed)
		}
	}
}

// A built-in contract's primary can differ from the registry's; the host derives only from the primary
// the registry named, never from whatever file happens to be first.
func TestHostEffects_DerivesOnlyFromTheDeclaredPrimary(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "other-report.md", triageReport)
	c := phasecontract.Contract{Phase: "triage", ArtifactName: "other-report.md", DerivedFrom: map[string]string{"triage-decision.json": "triage-report.md"}}

	errs := (&HostEffects{}).derive(c, phasecontract.Roots{Workspace: ws, Cycle: 7})

	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "primary") {
		t.Fatalf("errs = %v", errs)
	}
	if _, err := os.Stat(filepath.Join(ws, "triage-decision.json")); !os.IsNotExist(err) {
		t.Fatal("nothing may be derived from an undeclared primary")
	}
}
