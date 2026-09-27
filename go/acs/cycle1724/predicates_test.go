//go:build acs

// Package cycle1724 materialises the cycle-1724 acceptance criteria for this
// lane's single fleet-scoped task: remove-dormant-inboxbatch-dependency-grouping.
//
// Scope (per scout-report.md / triage-report.md, item
// inboxbatch-dormant-dependency-grouping): under ADR-0106 W3 dependency_blocked
// routing, a dependent is never offered in the same lane menu as its unlanded
// dependency, so `depRule` (go/internal/inboxbatch/rules.go) and the
// deps-ordering half of `topoOrder` (go/internal/inboxbatch/classify.go) are
// live-but-unreachable in production dispatch — dead code with passing tests.
// campaign grouping and the opt-in `ConnectsRule` are unaffected and must keep
// working exactly as today.
//
// Predicate strategy — every predicate below EXERCISES the system under test
// (calls inboxbatch.DefaultRules()/Classify, or renders the real triage prompt
// through triage.New(...).ComposePrompt); none is a source-grep of production
// text (the cycle-85 degenerate-predicate ban).
//
//   - 001 (AC1a) DefaultRules() must be exactly the two remaining structural
//     rules (campaign, file-area) — no depRule — AND items whose only shared
//     signal is a Deps chain must bind zero edges.
//   - 002 (AC1b) Classify's ordering must no longer respect Deps: a cluster
//     bound by campaign, with a heavier child declaring a dependency on a
//     lighter parent, must order weight-desc (child first), not topological
//     (parent first).
//   - 003 (AC3, regression guard) campaign grouping and the opt-in ConnectsRule
//     stay byte-for-byte unaffected — pre-existing GREEN today, must remain
//     GREEN after the deps removal.
//   - 004 (AC2) the real triage prompt's `inbox_batches` wording must name
//     exactly the grouping signals the triage path applies. It renders the
//     prompt over an inbox whose pairs share one signal each and checks the
//     wording against which pairs were really co-batched: campaign and
//     file-area must be named, connects_to (or "links") may be named only if
//     the connects_to pair is co-batched, and no dependency language is allowed
//     because a dependent is routed to dependency_blocked instead.
//   - 005 (AC5) the checked-in historical ACS predicates that pinned the
//     three-rule DefaultRules() (cycle1205, cycle1206, cycle1633) are
//     re-derived to the two-rule set and pass. The named tests must still run
//     and PASS, so deleting them does not count as a fix.
package cycle1724

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// wantRuleTypesAfterRemoval is the audited post-fix DefaultRules() composition:
// campaign and file-area only. depRule must no longer appear.
var wantRuleTypesAfterRemoval = []string{"inboxbatch.campaignRule", "inboxbatch.fileAreaRule"}

// TestC1724_001_DefaultRulesDropsDepRuleAndDepsAloneBindsNothing is AC1a.
func TestC1724_001_DefaultRulesDropsDepRuleAndDepsAloneBindsNothing(t *testing.T) {
	rules := inboxbatch.DefaultRules()
	if got := len(rules); got != len(wantRuleTypesAfterRemoval) {
		t.Fatalf("DefaultRules() returned %d rules, want %d (%v) — depRule must be deleted from rules.go:24 "+
			"once its deps-grouping is unreachable under ADR-0106 W3 dependency_blocked routing",
			got, len(wantRuleTypesAfterRemoval), wantRuleTypesAfterRemoval)
	}
	for i, r := range rules {
		if got := ruleTypeName(r); got != wantRuleTypesAfterRemoval[i] {
			t.Errorf("DefaultRules()[%d] is %s, want %s", i, got, wantRuleTypesAfterRemoval[i])
		}
	}

	// Behavioural half: items whose ONLY commonality is a Deps chain (no
	// Campaign, no shared Files) must bind zero edges once depRule is gone.
	items := []inboxbatch.Item{
		{ID: "parent", Weight: 0.2},
		{ID: "child", Weight: 0.95, Deps: []string{"parent"}},
	}
	if edges := allEdges(rules, items); len(edges) != 0 {
		t.Errorf("DefaultRules() bound %d edge(s) on items sharing only a Deps reference: %+v — "+
			"dep-based grouping is dead code under W3 routing and must be removed, not just unreachable",
			len(edges), edges)
	}
}

// TestC1724_002_ClassifyIgnoresDepsForOrdering is AC1b: once topoOrder's
// deps-ordering is removed, a campaign-bound cluster must order by
// weightDescThenID alone — a heavier child that declares a dependency on a
// lighter parent must come FIRST, not last.
func TestC1724_002_ClassifyIgnoresDepsForOrdering(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "parent", Weight: 0.2, Campaign: "camp-x"},
		{ID: "child", Weight: 0.95, Campaign: "camp-x", Deps: []string{"parent"}},
	}
	batches := inboxbatch.Classify(items, inboxbatch.Config{MaxItems: 4})
	if len(batches) != 1 {
		t.Fatalf("batches = %d, want 1 (campaign still unions them); got %+v", len(batches), batches)
	}
	if got := batchIDs(batches[0]); got != "child,parent" {
		t.Errorf("order = %s, want child,parent (weight-desc; topological dep ordering must be gone) — "+
			"topoOrder's deps-specific consumption (classify.go:142-233) is dead under W3 and must be removed",
			got)
	}
}

// TestC1724_003_ConnectsToAndCampaignGroupingUnaffected is AC3, a regression
// guard: campaign grouping and the opt-in ConnectsRule must keep working
// exactly as today. Expected pre-existing GREEN — neither codepath is touched
// by this cycle's fix.
func TestC1724_003_ConnectsToAndCampaignGroupingUnaffected(t *testing.T) {
	campaignItems := []inboxbatch.Item{
		{ID: "a", Weight: 0.9, Campaign: "camp-x"},
		{ID: "b", Weight: 0.5, Campaign: "camp-x"},
		{ID: "c", Weight: 0.7},
	}
	batches := inboxbatch.Classify(campaignItems, inboxbatch.Config{MaxItems: 4})
	if len(batches) != 2 || batchIDs(batches[0]) != "a,b" || batchIDs(batches[1]) != "c" {
		t.Fatalf("campaign grouping regressed: batches = %+v, want [a,b] [c]", batches)
	}

	connectItems := []inboxbatch.Item{
		{ID: "alpha", Weight: 0.8, ConnectsTo: []string{"beta (shares the digest surface)"}},
		{ID: "beta", Weight: 0.3},
	}
	if got := len(inboxbatch.Classify(connectItems, inboxbatch.Config{MaxItems: 4})); got != 2 {
		t.Fatalf("connects_to must stay opt-in: batches = %d, want 2 (no binding without ConnectsRule)", got)
	}
	withLink := inboxbatch.Classify(connectItems, inboxbatch.Config{
		MaxItems: 4,
		Rules:    append(inboxbatch.DefaultRules(), inboxbatch.ConnectsRule{}),
	})
	if len(withLink) != 1 || batchIDs(withLink[0]) != "alpha,beta" {
		t.Fatalf("opt-in ConnectsRule regressed: batches = %+v, want one alpha,beta batch", withLink)
	}
}

// TestC1724_004_InboxBatchesWordingNamesOnlySignalsTriageApplies is AC2: the
// `inbox_batches` line is an instruction to the triage LLM, so every grouping
// signal it names must be one the triage path really applied. The fixture
// holds one pair per signal and the wording is checked against the pairs the
// rendered prompt actually co-batched.
func TestC1724_004_InboxBatchesWordingNamesOnlySignalsTriageApplies(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	writeInboxItem(t, inboxDir, "camp-a", `{"id":"camp-a","weight":0.9,"campaign":"wiring"}`)
	writeInboxItem(t, inboxDir, "camp-b", `{"id":"camp-b","weight":0.8,"campaign":"wiring"}`)
	writeInboxItem(t, inboxDir, "area-a", `{"id":"area-a","weight":0.7,"files":["go/internal/zetaarea/a.go"]}`)
	writeInboxItem(t, inboxDir, "area-b", `{"id":"area-b","weight":0.6,"files":["go/internal/zetaarea/b.go"]}`)
	writeInboxItem(t, inboxDir, "link-a", `{"id":"link-a","weight":0.5,"connects_to":["link-b (shares the digest surface)"]}`)
	writeInboxItem(t, inboxDir, "link-b", `{"id":"link-b","weight":0.4}`)
	writeInboxItem(t, inboxDir, "dep-parent", `{"id":"dep-parent","weight":0.3}`)
	writeInboxItem(t, inboxDir, "dep-child", `{"id":"dep-child","weight":0.95,"deps":["dep-parent"]}`)

	prompt := triage.New(triage.Config{}).ComposePrompt("", core.PhaseRequest{ProjectRoot: root})

	wording := findLine(prompt, "- inbox_batches:")
	if wording == "" {
		t.Fatalf("no `- inbox_batches:` line rendered in the triage prompt:\n%s", prompt)
	}
	batches := renderedBatches(prompt)
	if !coBatched(batches, "camp-a", "camp-b") || !coBatched(batches, "area-a", "area-b") {
		t.Fatalf("fixture invalid: the campaign pair and the file-area pair must each share a rendered batch; "+
			"batches = %v", batches)
	}
	if inAnyBatch(batches, "dep-child") {
		t.Fatalf("fixture invalid: dep-child depends on the pending dep-parent, so ADR-0106 W3 must route it to "+
			"dependency_blocked, not into a batch; batches = %v", batches)
	}
	if !strings.Contains(findLine(prompt, "- dependency_blocked:"), "dep-child") {
		t.Fatalf("fixture invalid: dep-child is missing from the dependency_blocked line:\n%s", prompt)
	}

	lower := strings.ToLower(wording)
	for _, signal := range []string{"campaign", "file-area"} {
		if !strings.Contains(lower, signal) {
			t.Errorf("inbox_batches wording omits %q, a signal the triage path applies (DefaultRules); got %q",
				signal, wording)
		}
	}
	namesConnects := strings.Contains(lower, "connects") || strings.Contains(lower, "link")
	appliesConnects := coBatched(batches, "link-a", "link-b")
	if namesConnects && !appliesConnects {
		t.Errorf("inbox_batches wording claims connects_to/link grouping, but the triage path left the "+
			"connects_to-linked pair in separate batches (Classify(ready, Config{}) binds only DefaultRules; "+
			"ConnectsRule is opt-in). Name only campaign/file-area, or opt ConnectsRule in at the triage call "+
			"site. wording = %q, batches = %v", wording, batches)
	}
	if appliesConnects && !namesConnects {
		t.Errorf("the triage path co-batches connects_to-linked items but the wording does not say so: %q", wording)
	}
	if strings.Contains(lower, "depend") || strings.Contains(lower, "dep ") || strings.Contains(lower, "deps") {
		t.Errorf("inbox_batches wording still implies dependency-based grouping; dependents are routed to "+
			"dependency_blocked, never co-batched: %q", wording)
	}
}

// historicalRuleSetPins are the checked-in ACS predicates that pinned the
// three-rule DefaultRules() composition, keyed by package.
var historicalRuleSetPins = []struct {
	pkg   string
	tests []string
}{
	{"./acs/cycle1205/", []string{
		"TestC1205_001_DefaultRulesStaysBoundedStructuralRules",
		"TestC1205_005_RegressionTestFailsOnTheRejectedDesign",
	}},
	{"./acs/cycle1206/", []string{"TestC1206_001_DefaultRulesHasNoRootCauseSignal"}},
	{"./acs/cycle1633/", []string{"TestC1633_008_DefaultBatchRulesUnchangedBySynthesis"}},
}

// TestC1724_005_HistoricalRuleSetPredicatesRederived is AC5: removing depRule
// must not leave any checked-in predicate that pinned the old rule set RED.
// Each package runs on its own (one named package per `go test`, the shape the
// flaky-predicate lint allows). The named tests must report PASS, so deleting
// them does not satisfy this predicate.
func TestC1724_005_HistoricalRuleSetPredicatesRederived(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	for _, pin := range historicalRuleSetPins {
		t.Run(filepath.Base(pin.pkg), func(t *testing.T) {
			cmd := exec.Command("go", "test", "-tags", "acs", "-count=1", "-v", pin.pkg)
			cmd.Dir = goDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("%s is RED after the depRule removal — re-derive it to the two-rule DefaultRules() "+
					"(campaign, file-area): %v\n%s", pin.pkg, err, failLines(string(out)))
			}
			for _, name := range pin.tests {
				if !strings.Contains(string(out), "--- PASS: "+name+" ") {
					t.Errorf("%s: %q did not run and PASS — the rule-set pin must be re-derived, not deleted "+
						"or renamed away", pin.pkg, name)
				}
			}
		})
	}
}

// --- helpers -------------------------------------------------------------

func allEdges(rules []inboxbatch.Rule, items []inboxbatch.Item) []inboxbatch.Edge {
	var edges []inboxbatch.Edge
	for _, r := range rules {
		edges = append(edges, r.Edges(items)...)
	}
	return edges
}

// ruleTypeName renders a Rule's concrete type as package.Type. The rule types
// are unexported, so %T is the only handle a predicate outside the package has.
func ruleTypeName(r inboxbatch.Rule) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", r), "*")
}

func batchIDs(b inboxbatch.Batch) string {
	ids := make([]string, len(b.Items))
	for i, it := range b.Items {
		ids[i] = it.ID
	}
	return strings.Join(ids, ",")
}

func writeInboxItem(t *testing.T, dir, id, jsonBody string) {
	t.Helper()
	path := filepath.Join(dir, id+".json")
	if err := os.WriteFile(path, []byte(jsonBody), 0o644); err != nil {
		t.Fatalf("write inbox item %s: %v", path, err)
	}
}

// renderedBatches returns the item ids of each `- batch N (...): a, b` line.
func renderedBatches(prompt string) [][]string {
	var out [][]string
	for _, l := range strings.Split(prompt, "\n") {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "- batch ") {
			continue
		}
		cut := strings.LastIndex(l, ": ")
		if cut < 0 {
			continue
		}
		out = append(out, strings.Split(l[cut+2:], ", "))
	}
	return out
}

func coBatched(batches [][]string, a, b string) bool {
	for _, ids := range batches {
		if containsID(ids, a) && containsID(ids, b) {
			return true
		}
	}
	return false
}

func inAnyBatch(batches [][]string, id string) bool {
	for _, ids := range batches {
		if containsID(ids, id) {
			return true
		}
	}
	return false
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if strings.TrimSpace(x) == id {
			return true
		}
	}
	return false
}

// failLines keeps the FAIL and assertion lines of verbose go test output so a
// failure message names the broken predicate without the whole transcript.
func failLines(out string) string {
	var keep []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "FAIL") || strings.Contains(l, "predicates_test.go:") {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

func findLine(text, prefix string) string {
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			return l
		}
	}
	return ""
}
