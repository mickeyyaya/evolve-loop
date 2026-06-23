package specrunner

// RED-phase contract for cycle-249 task `phase-classify-declarative`:
// the declarative verdict evaluator must be EXPORTED as EvaluateClassify
// so built-in phases (triage, tdd, intent, build) can delegate their
// hand-coded classify logic to the one shared evaluator.
//
// The unexported evaluator's full matrix is already covered by
// TestEvaluateClassify in specrunner_test.go — these tests pin the
// EXPORTED surface only (signature + the contract rows built-in phases
// will rely on), so they complement rather than duplicate.
//
// Fails at baseline: EvaluateClassify is undefined (compile RED).

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
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
			// Intent AC: "config schema validated with explicit errors (no
			// silent fallback on malformed plugin config)" — a typo'd
			// verdict_on_pass must FAIL loudly, never pass silently.
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

// Pins heading-aware require_sections matching (inbox
// classify-heading-prefix-mismatch, 2026-06-07) — semantics documented on
// hasSection in specrunner.go. Includes strict-superset cases proving legacy
// matches are preserved.
func TestEvaluateClassifyExported_HeadingAwareSections(t *testing.T) {
	tests := []struct {
		name        string
		artifact    string
		rules       *phasespec.ClassifyRules
		wantVerdict string
	}{
		{
			// The cycle-249/250 regression: bare rule vs "## " heading.
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
			// CommonMark permits a tab as the marker/text separator.
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
			// Legacy prose-anchored behavior is preserved (strict superset).
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
			// "#"-run without trailing whitespace is not a markdown heading;
			// no stripping happens on that line.
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

// The missing-section diagnostic must NAME the missing section so a phase
// author can debug a FAIL from the message alone (easy-to-debug scaffold
// is an explicit cycle-249 goal).
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
