package phasespec

import (
	"strings"
	"testing"
)

// TestValidateOutputsPartition pins the ADR-0100 declaration rule at the seam
// both spec sources pass through, so an unclassified secondary is a load
// failure — never a phase that is silently ungated.
func TestValidateOutputsPartition(t *testing.T) {
	spec := func(files, owed, harness []string) PhaseSpec {
		return PhaseSpec{Name: "x", Outputs: IO{Files: files, AgentOwed: owed, HarnessProduced: harness}}
	}
	for _, tc := range []struct {
		name string
		s    PhaseSpec
		want string // "" = valid; else a substring of one violation
	}{
		{"primary only", spec([]string{"a/r.md"}, nil, nil), ""},
		{"no outputs", spec(nil, nil, nil), ""},
		{"owed secondary", spec([]string{"a/r.md", "a/h.json"}, []string{"h.json"}, nil), ""},
		{"harness secondary", spec([]string{"a/r.md", "a/v.json"}, nil, []string{"v.json"}), ""},
		{"unclassified secondary", spec([]string{"a/r.md", "a/h.json"}, nil, nil), "not classified"},
		{"classified twice", spec([]string{"a/r.md", "a/h.json"}, []string{"h.json"}, []string{"h.json"}), "classified twice"},
		{"undeclared classification", spec([]string{"a/r.md"}, []string{"h.json"}, nil), "does not declare"},
		{"path instead of basename", spec([]string{"a/r.md", "a/h.json"}, []string{"a/h.json"}, nil), "must be the basename"},
		{"primary is never a secondary", spec([]string{"a/r.md", "a/h.json"}, []string{"r.md", "h.json"}, nil), "does not declare"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := ValidateOutputsPartition(tc.s)
			if tc.want == "" {
				if len(v) != 0 {
					t.Fatalf("want valid, got %v", v)
				}
				return
			}
			if len(v) == 0 || !strings.Contains(strings.Join(v, "\n"), tc.want) {
				t.Fatalf("want a violation containing %q, got %v", tc.want, v)
			}
		})
	}
}

// TestValidateUserSpec_EnforcesOutputsPartition: overlays and user phases pass
// through the same rule — an overlay replaces a built-in's spec wholesale, so
// a rule that ran only on the registry would let the overlay ungate the phase.
func TestValidateUserSpec_EnforcesOutputsPartition(t *testing.T) {
	s := PhaseSpec{Name: "widget-scan", Optional: true, Kind: "llm",
		Outputs: IO{Files: []string{".evolve/runs/cycle-{cycle}/widget-scan-report.md", ".evolve/runs/cycle-{cycle}/widget-findings.json"}}}
	if v := ValidateUserSpec(s); !strings.Contains(strings.Join(v, "\n"), "not classified") {
		t.Fatalf("a user phase with an unclassified secondary must be rejected, got %v", v)
	}
	s.Outputs.AgentOwed = []string{"widget-findings.json"}
	for _, viol := range ValidateUserSpec(s) {
		if strings.Contains(viol, "classified") || strings.Contains(viol, "secondary") {
			t.Fatalf("a classified secondary must not be a violation: %q", viol)
		}
	}
}
