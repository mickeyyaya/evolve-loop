package inboxbatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validCommitmentF4() UnifiedCommitment {
	return UnifiedCommitment{
		RootCauseHypothesis: "one retry budget is read in two places",
		SharedSeam:          "internal/retrybudget",
		DesignRequirements:  []string{"single source of the budget"},
		Members: []UnifiedMember{
			{ID: "a", Evidence: "reads budget at dispatch"},
			{ID: "b", Evidence: "reads budget at resume"},
		},
	}
}

func itemsF4() []Item {
	return []Item{{ID: "a", Campaign: "c1"}, {ID: "b", Campaign: "c1"}, {ID: "c", Campaign: "c2"}, {ID: "d", Campaign: "c1", DeliverableKind: "document"}}
}

func TestUnifiedCommitmentValidate_RejectsEachDefectWithItsOwnMessage(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*UnifiedCommitment)
		want   string
	}{
		{"blank root cause", func(c *UnifiedCommitment) { c.RootCauseHypothesis = " \t" }, "root cause hypothesis is empty"},
		{"blank shared seam", func(c *UnifiedCommitment) { c.SharedSeam = "" }, "shared seam is empty"},
		{"no design requirements", func(c *UnifiedCommitment) { c.DesignRequirements = nil }, "design requirements are empty"},
		{"blank second requirement", func(c *UnifiedCommitment) { c.DesignRequirements = []string{"ok", "  "} }, "design requirement 2 is empty"},
		{"a single member", func(c *UnifiedCommitment) { c.Members = c.Members[:1] }, "at least two members"},
		{"blank member id", func(c *UnifiedCommitment) { c.Members[1].ID = "  " }, "member id is empty"},
		{"member without evidence", func(c *UnifiedCommitment) { c.Members[1].Evidence = "" }, `member "b" has no evidence`},
		{"duplicated member", func(c *UnifiedCommitment) { c.Members[1].ID = " a " }, `member "a" is duplicated`},
		{"unknown member", func(c *UnifiedCommitment) { c.Members[1].ID = "zzz" }, `member "zzz" is not a known inbox item`},
		{"members span campaigns", func(c *UnifiedCommitment) { c.Members[1].ID = "c" }, "span distinct campaigns"},
		{"members mix deliverable kinds", func(c *UnifiedCommitment) { c.Members[1].ID = "d" }, "mix deliverable kinds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validCommitmentF4()
			c.Members = append([]UnifiedMember(nil), c.Members...)
			tc.mutate(&c)
			err := c.Validate(itemsF4())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestUnifiedCommitmentValidate_AcceptsACompleteSameCampaignSameKindCommitment(t *testing.T) {
	if err := validCommitmentF4().Validate(itemsF4()); err != nil {
		t.Fatalf("err = %v", err)
	}
	two := itemsF4()
	two[0].DeliverableKind = "code"
	if err := validCommitmentF4().Validate(two); err != nil {
		t.Fatalf("an explicit code kind equals the default kind: %v", err)
	}
}

func TestUnifiedCommitmentSize_SplitsAtTheDefaultBatchCap(t *testing.T) {
	for _, tc := range []struct {
		members int
		want    string
	}{{0, "small"}, {1, "small"}, {DefaultMaxItems, "small"}, {DefaultMaxItems + 1, "large"}} {
		c := UnifiedCommitment{Members: make([]UnifiedMember, tc.members)}
		if got := c.Size(); got != tc.want {
			t.Errorf("Size() with %d members = %q, want %q", tc.members, got, tc.want)
		}
	}
}

func TestIsOperatorState_EntriesWithNoPathTokensAreNotOperatorState(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  bool
	}{
		{"every file under .evolve", []string{".evolve/state.json", ".evolve/inbox/x.json"}, true},
		{"a whitespace-only entry carries no token", []string{".evolve/state.json", "   "}, false},
		{"a prose-only entry carries no path token", []string{"see the notes"}, false},
		{"a source file among state files", []string{".evolve/state.json", "go/internal/a.go"}, false},
		{"a lookalike prefix is not state", []string{".evolvex/state.json"}, false},
		{"no files at all", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			it := Item{Class: "pipeline-architecture", Files: tc.files}
			if got := IsOperatorState(it); got != tc.want {
				t.Fatalf("IsOperatorState = %v, want %v", got, tc.want)
			}
		})
	}
	if IsOperatorState(Item{Class: "other", Files: []string{".evolve/state.json"}}) {
		t.Error("only the pipeline-architecture class can be operator state")
	}
}

func TestClassify_NoItemsYieldsNoBatches(t *testing.T) {
	if got := Classify(nil, Config{}); got != nil {
		t.Fatalf("nil items: %v", got)
	}
	if got := Classify([]Item{}, Config{MaxItems: 2}); got != nil {
		t.Fatalf("empty items: %v", got)
	}
}

func TestRenderMarkdown_MarksContinuationBatchesAndOnlyThem(t *testing.T) {
	batches := []Batch{
		{Items: []Item{{ID: "a"}, {ID: "b"}}, Reasons: []string{"campaign:x"}},
		{Items: []Item{{ID: "c"}}, Reasons: []string{"campaign:x"}, DependsOnPrev: true},
	}

	got := RenderMarkdown(batches, func(it Item) string { return "label-" + it.ID })

	want := "- batch 1 (campaign:x): a, b\n  - a: label-a\n  - b: label-b\n" +
		"- batch 2 (campaign:x) (continuation — run the previous batch first): c\n  - c: label-c\n"
	if got != want {
		t.Fatalf("RenderMarkdown =\n%s\nwant\n%s", got, want)
	}
}

func TestLoadDir_PathThatIsAFileIsAReadError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "inbox")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	items, warnings, err := LoadDir(file)
	if err == nil || items != nil || warnings != nil {
		t.Fatalf("items=%v warnings=%v err=%v", items, warnings, err)
	}
}

func TestRoutedResolver_AppendsOnlyTheLoadWarningsOfTheRoutedItemsOwnFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.json", `{"id":"routed","route":"console-manual","title":"bad\u0007title"}`)
	write("b.json", `{"id":"other","title":"also\u0007bad"}`)

	resolve := RoutedResolver(dir, nil)

	routed, reason := resolve("routed")
	if !routed || !strings.Contains(reason, "route:console-manual") || !strings.Contains(reason, "; INBOX_LOAD_WARNING: a.json: sanitized control characters in title") {
		t.Fatalf("routed=%v reason=%q", routed, reason)
	}
	if strings.Contains(reason, "b.json") {
		t.Errorf("another file's warning leaked into the reason: %q", reason)
	}
	if other, why := resolve("other"); other || why != "" {
		t.Errorf("a dispatchable item resolves with no reason: %v %q", other, why)
	}
}
