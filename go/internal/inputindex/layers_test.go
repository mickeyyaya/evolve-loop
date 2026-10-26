package inputindex

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func (f fixture) buildWith(cycle int, phase string, c ContextPolicy) Index {
	f.t.Helper()
	idx, err := Build(Request{ProjectRoot: f.root, Workspace: f.ws(cycle), Cycle: cycle, Phase: phase, ItemIDs: f.items[cycle], Context: c})
	if err != nil {
		f.t.Fatal(err)
	}
	return idx
}

func mustEntry(t *testing.T, idx Index, kind string, cycle int) Entry {
	t.Helper()
	e, ok := entryOf(idx, kind, cycle)
	if !ok {
		t.Fatalf("no %s entry of cycle %d in %+v", kind, cycle, idx.Entries)
	}
	return e
}

func requiredKindsOf(idx Index) []string {
	var kinds []string
	for _, e := range idx.Entries {
		if e.Required {
			kinds = append(kinds, e.Kind)
		}
	}
	return kinds
}

func retryFixture(t *testing.T) fixture {
	f := newFixture(t)
	f.cycle(5, "item-a", map[string]string{"retrospective-report.md": "# older retro\n", "failure-digest.json": `{"summary":"older digest"}`})
	f.cycle(6, "item-a", map[string]string{"retrospective-report.md": "# Audit FAIL: add a seam\n", "failure-digest.json": `{"summary":"the newest digest"}`, "audit-report.md": "# Audit: FAIL\n"})
	f.cycle(7, "item-a", map[string]string{
		"scout-report.md":           "# Scout: replay harness\n",
		"triage-decision.json":      `{"summary":"one item"}`,
		"test-report.md":            "# TDD: 3 RED tests\n",
		"handoff-tdd-build.json":    `{"summary":"asks for the builder"}`,
		"handoff-triage-tdd.json":   `{"summary":"asks for the tdd engineer"}`,
		"handoff-scout-triage.json": `{"summary":"asks for triage"}`,
	})
	f.write(".evolve/instincts/lessons/by-cycle.yaml", "summary: from cycle-6\n")
	return f
}

func TestBuild_EachEntryHasTheLayerOfItsClass(t *testing.T) {
	f := retryFixture(t)
	idx := f.build(7, "build")
	if idx.Schema != Schema || Schema != "evolve.inputs.index/2" || idx.Estimate != tokenEstimate || idx.Stage != stageShadow {
		t.Fatalf("header = schema %q estimate %q stage %q", idx.Schema, idx.Estimate, idx.Stage)
	}
	for _, e := range idx.Entries {
		want := map[string]string{ClassDeliverable: LayerUpstream, ClassLineage: LayerRecovery}[e.Class]
		if want == "" || e.Layer != want {
			t.Errorf("entry %s of class %s has layer %q, want %q", e.ID, e.Class, e.Layer, want)
		}
	}
	if mustEntry(t, idx, "retrospective", 6).Layer != LayerRecovery || mustEntry(t, idx, "scout-report", 7).Layer != LayerUpstream {
		t.Fatal("a lineage entry is L2 and a deliverable is L1")
	}
}

func TestBuild_EachEntryCarriesItsTokenEstimateAndItsReason(t *testing.T) {
	f := retryFixture(t)
	f.write(".evolve/runs/cycle-7/scout-report.md", "# "+strings.Repeat("x", 2418))
	idx := f.build(7, "build")
	scout := mustEntry(t, idx, "scout-report", 7)
	if scout.Bytes != 2420 || scout.Tokens != 807 {
		t.Fatalf("scout bytes %d tokens %d, want 2420 and ceil(2420/3) = 807", scout.Bytes, scout.Tokens)
	}
	for _, c := range []struct {
		kind  string
		cycle int
		why   string
	}{
		{"scout-report", 7, "upstream deliverable of scout; required by the build inputs"},
		{"test-report", 7, "upstream deliverable of tdd"},
		{"retrospective", 6, "lineage cycle 6 of item item-a, attempt 2; required by the build inputs"},
		{"retrospective", 5, "lineage cycle 5 of item item-a, attempt 1"},
		{"lesson", 0, "lesson that names cycle-6"},
	} {
		if got := mustEntry(t, idx, c.kind, c.cycle).Why; got != c.why {
			t.Errorf("%s of cycle %d: why = %q, want %q", c.kind, c.cycle, got, c.why)
		}
	}
	whys := map[string]string{}
	for _, e := range idx.Entries {
		if e.Kind == kindHandoffAsks {
			whys[e.Path[strings.LastIndex(e.Path, "/")+1:]] = e.Why
		}
	}
	want := map[string]string{
		"handoff-tdd-build.json":    "asks of tdd for build; required by the build inputs",
		"handoff-triage-tdd.json":   "asks of the pair triage-tdd",
		"handoff-scout-triage.json": "asks of the pair scout-triage",
	}
	for name, w := range want {
		if whys[name] != w {
			t.Errorf("%s: why = %q, want %q", name, whys[name], w)
		}
	}
	if tokensOf(0) != 0 || tokensOf(1) != 1 || tokensOf(3) != 1 || tokensOf(4) != 2 {
		t.Fatal("tokensOf must be ceil(bytes/3)")
	}
}

func TestRequired_PolicyThenSpecThenEdges(t *testing.T) {
	f := retryFixture(t)
	for name, c := range map[string]struct {
		policy ContextPolicy
		want   []string
	}{
		"the policy list wins over the spec": {ContextPolicy{Required: []string{"test-report"}, Inputs: []string{".evolve/runs/cycle-{cycle}/scout-report.md"}},
			[]string{"handoff-asks", "test-report", "failure-digest", "retrospective"}},
		"an empty policy list requires no deliverable": {ContextPolicy{Required: []string{}, Inputs: []string{".evolve/runs/cycle-{cycle}/scout-report.md"}},
			[]string{"handoff-asks", "failure-digest", "retrospective"}},
		"the spec inputs map a path template to its kind": {ContextPolicy{Inputs: []string{".evolve/runs/cycle-{cycle}/scout-report.md", ".evolve/runs/cycle-{cycle}/triage-report.md"}},
			[]string{"handoff-asks", "scout-report", "failure-digest", "retrospective"}},
		"with neither, the phaseio edges decide": {ContextPolicy{},
			[]string{"handoff-asks", "scout-report", "triage-decision", "failure-digest", "retrospective"}},
	} {
		got := requiredKindsOf(f.buildWith(7, "build", c.policy))
		slices.Sort(got)
		want := slices.Clone(c.want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: required = %v, want %v", name, got, want)
		}
	}
}

func TestRequired_OnlyTheNewestLineageCycleAndTheAsksForThisPhase(t *testing.T) {
	f := retryFixture(t)
	idx := f.build(7, "build")
	if !mustEntry(t, idx, "retrospective", 6).Required || !mustEntry(t, idx, "failure-digest", 6).Required {
		t.Fatal("the retrospective and the digest of the newest lineage cycle are required")
	}
	for _, e := range idx.Entries {
		if e.Required && (e.Cycle == 5 || e.Kind == "audit-report" || e.Kind == "lesson" || strings.HasSuffix(e.Path, "handoff-triage-tdd.json")) {
			t.Errorf("entry %s (%s of cycle %d) must not be required", e.ID, e.Kind, e.Cycle)
		}
	}
	var ids []string
	for _, e := range idx.Entries {
		if e.Required {
			ids = append(ids, e.ID)
		}
	}
	if !slices.Equal(idx.Required, ids) {
		t.Fatalf("header required = %v, want the ids in entry order %v", idx.Required, ids)
	}
	first := newFixture(t)
	first.cycle(7, "item-a", map[string]string{"scout-report.md": "# s\n"})
	if got := requiredKindsOf(first.build(7, "build")); !slices.Equal(got, []string{"scout-report"}) {
		t.Fatalf("a first attempt requires only the upstream deliverables, got %v", got)
	}
}

func TestBudget_AnOverflowClearsRequiredAndKeepsTheRow(t *testing.T) {
	f := retryFixture(t)
	f.write(".evolve/runs/cycle-7/scout-report.md", "# "+strings.Repeat("x", 298))
	limit := 100 + int(tokensOf(int64(len(`{"summary":"asks for the builder"}`))))
	idx := f.buildWith(7, "build", ContextPolicy{Required: []string{"scout-report", "triage-decision"}, Budget: Budget{L1Tokens: limit, L2Tokens: 1}})
	scout, triage := mustEntry(t, idx, "scout-report", 7), mustEntry(t, idx, "triage-decision", 7)
	if !scout.Required || scout.Tokens != 100 {
		t.Fatalf("the first required entry fits the budget exactly: %+v", scout)
	}
	if triage.Required || triage.Why != "upstream deliverable of triage; not required: over the L1 budget" {
		t.Fatalf("the entry past the budget keeps its row and loses the flag: %+v", triage)
	}
	if b := idx.Budgets[LayerUpstream]; b.LimitTokens != limit || b.RequiredTokens != int64(limit) || b.Over != 1 {
		t.Fatalf("L1 budget = %+v, want limit %d and %d used: the asks and the scout report fit exactly", b, limit, limit)
	}
	if b := idx.Budgets[LayerRecovery]; b != (LayerBudget{LimitTokens: 1, Over: 2}) {
		t.Fatalf("L2 budget = %+v, want limit 1 and nothing used", b)
	}
	overflow := 0
	for _, n := range idx.Notes {
		if strings.Contains(n, "over the L1 budget") || strings.Contains(n, "over the L2 budget") {
			overflow++
		}
	}
	if overflow < 3 {
		t.Fatalf("each entry past a budget gets a note, got %v", idx.Notes)
	}
	if got := f.build(7, "build").Budgets[LayerUpstream].LimitTokens; got != defaultL1Tokens {
		t.Fatalf("an unset budget uses the default %d, got %d", defaultL1Tokens, got)
	}
	if got := f.build(7, "build").Budgets[LayerRecovery].LimitTokens; got != defaultL2Tokens {
		t.Fatalf("an unset L2 budget uses the default %d, got %d", defaultL2Tokens, got)
	}
}

func TestBuild_TheStageOfThePolicyIsTheStageOfTheIndex(t *testing.T) {
	f := retryFixture(t)
	if got := f.buildWith(7, "build", ContextPolicy{Stage: "custom"}).Stage; got != "custom" {
		t.Fatalf("stage = %q, want the stage of the policy", got)
	}
}

func TestRead_AcceptsTheFirstSchemaForOneRelease(t *testing.T) {
	f := newFixture(t)
	ws := f.ws(7)
	f.write(".evolve/runs/cycle-7/inputs/index.json", `{"schema":"`+schemaV1+`","entries":[{"id":"d1","path":"a"}]}`)
	idx, err := Read(ws)
	if err != nil || len(idx.Entries) != 1 {
		t.Fatalf("a /1 index must stay readable: %+v %v", idx, err)
	}
	f.write(".evolve/runs/cycle-7/inputs/index.json", `{"schema":"evolve.inputs.index/3"}`)
	if _, err := Read(ws); err == nil || errors.Is(err, nil) {
		t.Fatal("a /3 index must be refused")
	}
}
