package specrunner

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestEvaluateClassifyExported(t *testing.T) {
	tests := []struct {
		name        string
		artifact    string
		rules       *phasespec.ClassifyRules
		wantVerdict string
		wantDiag    bool
	}{
		{
			name:        "empty_artifact_nil_rules_fails",
			artifact:    "   \n",
			rules:       nil,
			wantVerdict: core.VerdictFAIL,
			wantDiag:    true,
		},
		{
			name:        "empty_artifact_fail_if_empty_fails",
			artifact:    "",
			rules:       &phasespec.ClassifyRules{FailIfEmpty: true},
			wantVerdict: core.VerdictFAIL,
			wantDiag:    true,
		},
		{
			name:        "empty_artifact_explicit_opt_out_passes",
			artifact:    "",
			rules:       &phasespec.ClassifyRules{FailIfEmpty: false, RequireSections: nil},
			wantVerdict: core.VerdictPASS,
		},
		{
			name:     "missing_required_section_fails",
			artifact: "## top_n\n- item one\n",
			rules: &phasespec.ClassifyRules{
				RequireSections: []string{"## top_n", "## RED Tests"},
				FailIfEmpty:     true,
			},
			wantVerdict: core.VerdictFAIL,
			wantDiag:    true,
		},
		{
			name:     "all_required_sections_present_passes",
			artifact: "## Acceptance\nstuff\n\n## RED Tests\nmore\n",
			rules: &phasespec.ClassifyRules{
				RequireSections: []string{"## Acceptance", "## RED Tests"},
				FailIfEmpty:     true,
			},
			wantVerdict: core.VerdictPASS,
		},
		{
			name:        "verdict_on_pass_projects_warn",
			artifact:    "## A\nbody\n",
			rules:       &phasespec.ClassifyRules{RequireSections: []string{"## A"}, VerdictOnPass: core.VerdictWARN},
			wantVerdict: core.VerdictWARN,
		},
		{
			name:        "invalid_verdict_on_pass_fails_loudly",
			artifact:    "## A\nbody\n",
			rules:       &phasespec.ClassifyRules{RequireSections: []string{"## A"}, VerdictOnPass: "PASSS"},
			wantVerdict: core.VerdictFAIL,
			wantDiag:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verdict, diags := EvaluateClassify(tc.artifact, tc.rules)
			if verdict != tc.wantVerdict {
				t.Errorf("verdict = %q, want %q (diags: %+v)", verdict, tc.wantVerdict, diags)
			}
			if tc.wantDiag && len(diags) == 0 {
				t.Errorf("expected an explicit error diagnostic, got none — silent failure is the cycle-249 anti-goal")
			}
			if tc.wantDiag {
				for _, d := range diags {
					if d.Severity != "error" {
						t.Errorf("diagnostic severity = %q, want \"error\"", d.Severity)
					}
				}
			}
		})
	}
}

func TestEvaluateClassifyExported_HeadingAwareSections(t *testing.T) {
	tests := []struct {
		name        string
		artifact    string
		rules       *phasespec.ClassifyRules
		wantVerdict string
	}{
		{
			name:        "bare rule matches h2 heading",
			artifact:    "## Baseline\n- ok\n",
			rules:       &phasespec.ClassifyRules{RequireSections: []string{"Baseline"}},
			wantVerdict: core.VerdictPASS,
		},
		{
			name:        "bare rule matches h3 heading",
			artifact:    "### Verdict\nPASS\n",
			rules:       &phasespec.ClassifyRules{RequireSections: []string{"Verdict"}},
			wantVerdict: core.VerdictPASS,
		},
		{
			name:        "tab-separated heading matches bare rule",
			artifact:    "##\tFindings\n- x\n",
			rules:       &phasespec.ClassifyRules{RequireSections: []string{"Findings"}},
			wantVerdict: core.VerdictPASS,
		},
		{
			name:        "prefixed rule matches bare line",
			artifact:    "Findings\n- one\n",
			rules:       &phasespec.ClassifyRules{RequireSections: []string{"## Findings"}},
			wantVerdict: core.VerdictPASS,
		},
		{
			name:        "bare rule matches line-anchored prose",
			artifact:    "Verdict: PASS\n",
			rules:       &phasespec.ClassifyRules{RequireSections: []string{"Verdict"}},
			wantVerdict: core.VerdictPASS,
		},
		{
			name:        "mid-line occurrence still does not match",
			artifact:    "see Findings below\n",
			rules:       &phasespec.ClassifyRules{RequireSections: []string{"Findings"}},
			wantVerdict: core.VerdictFAIL,
		},
		{
			name:        "hashtag-glued line is not a heading",
			artifact:    "##Findings\n",
			rules:       &phasespec.ClassifyRules{RequireSections: []string{"Findings"}},
			wantVerdict: core.VerdictFAIL,
		},
		{
			name:        "absent section still fails",
			artifact:    "## Other\n",
			rules:       &phasespec.ClassifyRules{RequireSections: []string{"Baseline"}},
			wantVerdict: core.VerdictFAIL,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verdict, diags := EvaluateClassify(tc.artifact, tc.rules)
			if verdict != tc.wantVerdict {
				t.Errorf("verdict = %q, want %q (diags: %+v)", verdict, tc.wantVerdict, diags)
			}
		})
	}
}

func TestEvaluateClassifyExported_DiagnosticNamesMissingSection(t *testing.T) {
	_, diags := EvaluateClassify("## present\nx\n", &phasespec.ClassifyRules{
		RequireSections: []string{"## present", "## absent-section"},
	})
	if len(diags) == 0 {
		t.Fatal("expected a diagnostic for the missing section")
	}
	if !strings.Contains(diags[0].Message, "## absent-section") {
		t.Errorf("diagnostic must name the missing section; got: %q", diags[0].Message)
	}
}
