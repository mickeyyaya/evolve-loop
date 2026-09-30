//go:build acs

package cycle1206

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1206_001_DefaultRulesHasNoRootCauseSignal(t *testing.T) {
	probes := []struct {
		signal string
		items  []inboxbatch.Item
	}{
		{
			signal: "campaign",
			items: []inboxbatch.Item{
				{ID: "a", Campaign: "shared-campaign"},
				{ID: "b", Campaign: "shared-campaign"},
			},
		},
		{
			signal: "file-area",
			items: []inboxbatch.Item{
				{ID: "a", Files: []string{"go/internal/probearea/x.go"}},
				{ID: "b", Files: []string{"go/internal/probearea/y.go"}},
			},
		},
	}

	rules := inboxbatch.DefaultRules()
	if len(rules) != len(probes) {
		t.Errorf("C1206-001: DefaultRules() has %d rules, want exactly %d (campaign, file-area); an extra rule means a new binding signal was compiled in — root_cause binding is a rejected design (failedApproaches[54])", len(rules), len(probes))
	}

	for i, r := range rules {
		var responds []string
		for _, p := range probes {
			if len(r.Edges(p.items)) > 0 {
				responds = append(responds, p.signal)
			}
		}
		switch len(responds) {
		case 1:
		case 0:
			t.Errorf("C1206-001: DefaultRules()[%d] (%T) emits no edge for any of the documented structural signals (campaign, file-area) — an unaccounted binding signal is compiled into the default set; root_cause binding is a rejected design (failedApproaches[54], zero edges on 20/20 unique prose values)", i, r)
		default:
			t.Errorf("C1206-001: DefaultRules()[%d] (%T) responds to multiple signals %v — the default set must be single-signal rules", i, r, responds)
		}
	}
}

func TestC1206_002_RootCauseFieldBindsNothingEndToEnd(t *testing.T) {
	dir := t.TempDir()
	const sharedRootCause = "go/internal/phases/runner/runner.go: the file-authoritative verdict path drops the substantive error, so the tier reports a false RED under contention"

	write := func(name string, doc map[string]any) {
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("C1206-002: marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
			t.Fatalf("C1206-002: write %s: %v", name, err)
		}
	}

	write("alpha.json", map[string]any{
		"id":         "alpha-item",
		"title":      "alpha",
		"weight":     0.5,
		"files":      []string{"go/internal/alphaonly/a.go"},
		"root_cause": sharedRootCause,
	})
	write("beta.json", map[string]any{
		"id":         "beta-item",
		"title":      "beta",
		"weight":     0.4,
		"files":      []string{"docs/betaonly/b.md"},
		"root_cause": sharedRootCause,
	})

	items, warnings, err := inboxbatch.LoadDir(dir)
	if err != nil {
		t.Fatalf("C1206-002: LoadDir: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("C1206-002: LoadDir returned %d items, want 2 (warnings: %v)", len(items), warnings)
	}

	batches := inboxbatch.Classify(items, inboxbatch.Config{})
	if len(batches) != 2 {
		t.Errorf("C1206-002: an identical root_cause value grouped %d item(s) into %d batch(es), want 2 separate batches — root_cause must NOT be a binding signal (rejected design, failedApproaches[54]); batches=%+v", len(items), len(batches), batches)
	}
	for _, b := range batches {
		if len(b.Items) != 1 {
			t.Errorf("C1206-002: batch holds %d items (%+v) with reasons %v — root_cause bound them; want one item per batch", len(b.Items), b.Items, b.Reasons)
		}
	}
}

// acs-predicate: config-check — this criterion is inherently a
func TestC1206_003_RejectionRationaleRecordedAtDecisionSite(t *testing.T) {
	rules := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "inboxbatch", "rules.go")

	for _, needle := range []string{"root_cause", "failedApproaches[54]"} {
		if !acsassert.FileContains(t, rules, needle) {
			t.Errorf("C1206-003: %s does not mention %q — the root_cause rejection rationale is not recorded at the decision site (DefaultRules doc comment), so the task can resurface without its evidence", rules, needle)
		}
	}
	if !acsassert.FileContainsAny(rules, "zero edges", "no edges") {
		t.Errorf("C1206-003: %s records no measured outcome for root_cause binding — the rationale must state that exact-match binding emits zero edges on the real backlog (20/20 unique prose values, cycle-1206 measurement)", rules)
	}
}
