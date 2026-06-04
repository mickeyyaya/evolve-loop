package phasecontract

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestFromSpec(t *testing.T) {
	tests := []struct {
		name string
		spec phasespec.PhaseSpec
		want Contract
	}{
		{
			name: "evaluate phase with require_sections and templated outputs path",
			spec: phasespec.PhaseSpec{
				Name: "foo",
				Role: "evaluate",
				Classify: &phasespec.ClassifyRules{
					RequireSections: []string{"Findings"},
				},
				Outputs: phasespec.IO{
					Files: []string{".evolve/runs/cycle-{cycle}/foo-report.md"},
				},
			},
			want: Contract{
				Phase:        "foo",
				AgentName:    "evolve-foo",
				ArtifactName: "foo-report.md",
				Kind:         KindMarkdown,
				Sections:     []Section{{Canonical: "## Findings", Accepted: []string{"## Findings", "Findings"}}},
				Verdicts:     nil,
				RequiredKeys: nil,
				WriteTarget:  TargetWorkspace,
			},
		},
		{
			name: "no outputs.files falls back to <name>-report.md",
			spec: phasespec.PhaseSpec{
				Name: "security-scan",
				Role: "evaluate",
				Classify: &phasespec.ClassifyRules{
					RequireSections: []string{"Findings"},
				},
				Outputs: phasespec.IO{Signals: []string{"security.severity_max"}},
			},
			want: Contract{
				Phase:        "security-scan",
				AgentName:    "evolve-security-scan",
				ArtifactName: "security-scan-report.md",
				Kind:         KindMarkdown,
				Sections:     []Section{{Canonical: "## Findings", Accepted: []string{"## Findings", "Findings"}}},
				WriteTarget:  TargetWorkspace,
			},
		},
		{
			name: "json output extension yields KindJSON",
			spec: phasespec.PhaseSpec{
				Name:    "emit-plan",
				Outputs: phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/emit-plan.json"}},
			},
			want: Contract{
				Phase:        "emit-plan",
				AgentName:    "evolve-emit-plan",
				ArtifactName: "emit-plan.json",
				Kind:         KindJSON,
				WriteTarget:  TargetWorkspace,
			},
		},
		{
			name: "verdict opt-in: evaluate + verdict_on_pass yields standard vocab",
			spec: phasespec.PhaseSpec{
				Name: "adversarial-review",
				Role: "evaluate",
				Classify: &phasespec.ClassifyRules{
					RequireSections: []string{"Threat Model", "Findings", "Verdict"},
					VerdictOnPass:   "PASS",
				},
				Outputs: phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/adversarial-review-report.md"}},
			},
			want: Contract{
				Phase:        "adversarial-review",
				AgentName:    "evolve-adversarial-review",
				ArtifactName: "adversarial-review-report.md",
				Kind:         KindMarkdown,
				Sections: []Section{
					{Canonical: "## Threat Model", Accepted: []string{"## Threat Model", "Threat Model"}},
					{Canonical: "## Findings", Accepted: []string{"## Findings", "Findings"}},
					{Canonical: "## Verdict", Accepted: []string{"## Verdict", "Verdict"}},
				},
				Verdicts:    []string{"PASS", "FAIL", "WARN", "SKIPPED"},
				WriteTarget: TargetWorkspace,
			},
		},
		{
			name: "verdict NOT inferred without opt-in even for evaluate archetype",
			spec: phasespec.PhaseSpec{
				Name:     "perf-profile",
				Role:     "evaluate",
				Classify: &phasespec.ClassifyRules{RequireSections: []string{"Benchmarks"}},
				Outputs:  phasespec.IO{Files: []string{"perf-profile-report.md"}},
			},
			want: Contract{
				Phase:        "perf-profile",
				AgentName:    "evolve-perf-profile",
				ArtifactName: "perf-profile-report.md",
				Kind:         KindMarkdown,
				Sections:     []Section{{Canonical: "## Benchmarks", Accepted: []string{"## Benchmarks", "Benchmarks"}}},
				Verdicts:     nil,
				WriteTarget:  TargetWorkspace,
			},
		},
		{
			name: "explicit agent overrides evolve-<name> default",
			spec: phasespec.PhaseSpec{
				Name:    "bar",
				Agent:   "custom-agent",
				Outputs: phasespec.IO{Files: []string{"bar-report.md"}},
			},
			want: Contract{
				Phase:        "bar",
				AgentName:    "custom-agent",
				ArtifactName: "bar-report.md",
				Kind:         KindMarkdown,
				WriteTarget:  TargetWorkspace,
			},
		},
		{
			name: "zero-value spec yields safe fallbacks (no panic)",
			spec: phasespec.PhaseSpec{},
			want: Contract{
				Phase:        "",
				AgentName:    "evolve-",
				ArtifactName: "-report.md",
				Kind:         KindMarkdown,
				WriteTarget:  TargetWorkspace,
			},
		},
		{
			name: "author-supplied heading prefix is not double-prefixed",
			spec: phasespec.PhaseSpec{
				Name:     "baz",
				Classify: &phasespec.ClassifyRules{RequireSections: []string{"## Findings"}},
				Outputs:  phasespec.IO{Files: []string{"baz-report.md"}},
			},
			want: Contract{
				Phase:        "baz",
				AgentName:    "evolve-baz",
				ArtifactName: "baz-report.md",
				Kind:         KindMarkdown,
				Sections:     []Section{{Canonical: "## Findings", Accepted: []string{"## Findings"}}},
				WriteTarget:  TargetWorkspace,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromSpec(tt.spec)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromSpec(%s) mismatch\n got: %+v\nwant: %+v", tt.spec.Name, got, tt.want)
			}
		})
	}
}
