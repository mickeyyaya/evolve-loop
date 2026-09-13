package campaign

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

// TestLoadFile_NamesCampaignPublicTypesAndFileLoader graduates the campaign
// package into the public-API gate while pinning the file-backed entrypoint.
func TestLoadFile_NamesCampaignPublicTypesAndFileLoader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign-plan.json")
	data := []byte(`{"version":1,"goal":"g","research":{"summary":"s","citations":[{"title":"source"}]},"cycles":[{"id":"c1"}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	plan, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	var research Research = plan.Research
	var citation Citation = research.Citations[0]
	if plan.Goal != "g" || citation.Title != "source" {
		t.Fatalf("loaded Plan = %+v, citation = %+v", plan, citation)
	}
}

func TestPlanFromUnifiedCommitment_PreservesMemberContract(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "a", Files: []string{"a.go"}, Acceptance: []string{"a passes"}},
		{ID: "b", Files: []string{"b.go"}, Deps: []string{"a"}, Acceptance: []string{"b passes"}},
	}
	c := inboxbatch.UnifiedCommitment{
		RootCauseHypothesis: "shared defect",
		SharedSeam:          "internal/shared",
		DesignRequirements:  []string{"one implementation"},
		Members: []inboxbatch.UnifiedMember{
			{ID: "a", Evidence: "a shows the defect"},
			{ID: "b", Evidence: "b shows the defect"},
		},
	}
	plan, err := PlanFromUnifiedCommitment(c, items)
	if err != nil {
		t.Fatalf("PlanFromUnifiedCommitment: %v", err)
	}
	if got := plan.Cycles[1].OutputContract; got != "b passes" {
		t.Fatalf("member output contract = %q, want %q", got, "b passes")
	}
	if got := plan.Cycles[1].DependsOn; len(got) != 1 || got[0] != "a" {
		t.Fatalf("member dependencies = %v, want [a]", got)
	}
}
