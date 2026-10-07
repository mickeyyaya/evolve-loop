package phasecontract

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestFromSpec_ProjectsTheDeclaredReportGrammars(t *testing.T) {
	declared := phasespec.PhaseSpec{Name: "code-review", Classify: &phasespec.ClassifyRules{RequireSections: []string{"Scores"}, Grammars: []string{phasespec.GrammarCodeReviewReport}}}
	if got := FromSpec(declared).Grammars; !reflect.DeepEqual(got, []string{phasespec.GrammarCodeReviewReport}) {
		t.Errorf("Grammars = %v, want [%s]: the deliverable gate checks only what the contract carries", got, phasespec.GrammarCodeReviewReport)
	}
	for name, spec := range map[string]phasespec.PhaseSpec{
		"no classify block":    {Name: "smell-scan"},
		"classify, no grammar": {Name: "smell-scan", Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings"}}},
	} {
		if got := FromSpec(spec).Grammars; got != nil {
			t.Errorf("%s: Grammars = %v, want none", name, got)
		}
	}
}
