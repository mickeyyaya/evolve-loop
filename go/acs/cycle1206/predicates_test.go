//go:build acs

// Package cycle1206 encodes the cycle-1206 ACS predicates for task
// `reject-inboxbatch-rootcause-rule`: the formal closure of
// `add-inboxbatch-rootcause-rule` as a DESIGN REJECTION rather than an
// implementation.
//
// The predicates are deliberately shaped as anti-regression assertions. The
// cycle-1204 attempt (state.json failedApproaches[54], audit FAIL, D1-D4) added
// a `rootCauseRule` binding inbox items whose `root_cause` field matched by
// exact string equality. It was fully reverted. Live measurement of the real
// backlog (20/20 non-empty root_cause values unique free-form prose, 122-1564
// bytes, zero duplicates) proves exact-match binding on that field emits zero
// edges — so the correct deliverable is that the rule STAYS ABSENT and the
// reason is recorded where the next author will look.
package cycle1206

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC1206_001_DefaultRulesHasNoRootCauseSignal asserts the compiled default
// rule set is EXACTLY the three documented structural signals — campaign,
// file-area, dep — by probing each rule with a pair of items bound by one
// signal only and requiring every rule to be accounted for by one probe.
//
// This is the load-bearing anti-regression assertion: a re-added rootCauseRule
// responds to none of the three structural probes (the Go Item type carries no
// root_cause field at all), so it lands as UNACCOUNTED and this predicate goes
// RED. A source grep for "RootCause" would be gameable by renaming; this is not.
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
		{
			signal: "dep",
			items: []inboxbatch.Item{
				{ID: "a"},
				{ID: "b", Deps: []string{"a"}},
			},
		},
	}

	rules := inboxbatch.DefaultRules()
	if len(rules) != 3 {
		t.Errorf("C1206-001: DefaultRules() has %d rules, want exactly 3 (campaign, file-area, dep); an extra rule means a new binding signal was compiled in — root_cause binding is a rejected design (failedApproaches[54])", len(rules))
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
			// Accounted for by exactly one known structural signal.
		case 0:
			t.Errorf("C1206-001: DefaultRules()[%d] (%T) emits no edge for any of the three documented structural signals (campaign, file-area, dep) — an unaccounted binding signal is compiled into the default set; root_cause binding is a rejected design (failedApproaches[54], zero edges on 20/20 unique prose values)", i, r)
		default:
			t.Errorf("C1206-001: DefaultRules()[%d] (%T) responds to multiple signals %v — the default set must be three single-signal rules", i, r, responds)
		}
	}
}

// TestC1206_002_RootCauseFieldBindsNothingEndToEnd is the NEGATIVE / OOD
// predicate: two real inbox JSON files whose ONLY shared field is `root_cause`
// (identical verbatim — the strongest possible input for an exact-match rule,
// and a case the real backlog never even produces) must still classify into
// SEPARATE batches. Exercised through the production path LoadDir -> Classify
// with the compiled default rules, so it covers decode, sanitize, rule union,
// union-find and batching together.
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

	// No shared campaign, no shared file area, no dep/connects edge. Only
	// root_cause is identical.
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

// TestC1206_003_RejectionRationaleRecordedAtDecisionSite asserts the design
// rejection is DURABLY recorded at the site a future author will read before
// adding a binding signal: the DefaultRules doc comment in
// go/internal/inboxbatch/rules.go, which already carries exactly this kind of
// exclusion rationale for connects_to ("real-backlog validation showed...").
// Without this, `add-inboxbatch-rootcause-rule` resurfaces as a bare todo-id
// stripped of its "why" — the lesson_to_action_gap failure mode.
//
// acs-predicate: config-check — this criterion is inherently a
// documentation-presence assertion (the deliverable IS the recorded rationale);
// there is no runtime behavior to invoke. Behavioral coverage of the rejection
// itself lives in C1206-001 and C1206-002.
func TestC1206_003_RejectionRationaleRecordedAtDecisionSite(t *testing.T) {
	rules := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "inboxbatch", "rules.go")

	// The rationale must name the field, the measured zero-edge result, and the
	// failedApproaches pointer, so the record survives without this report.
	for _, needle := range []string{"root_cause", "failedApproaches[54]"} {
		if !acsassert.FileContains(t, rules, needle) {
			t.Errorf("C1206-003: %s does not mention %q — the root_cause rejection rationale is not recorded at the decision site (DefaultRules doc comment), so the task can resurface without its evidence", rules, needle)
		}
	}
	if !acsassert.FileContainsAny(rules, "zero edges", "no edges") {
		t.Errorf("C1206-003: %s records no measured outcome for root_cause binding — the rationale must state that exact-match binding emits zero edges on the real backlog (20/20 unique prose values, cycle-1206 measurement)", rules)
	}
}
