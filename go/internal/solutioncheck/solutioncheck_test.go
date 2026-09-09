package solutioncheck

// solutioncheck_test.go — ADR-0099 slice 2 RED contract. A `document` cycle's
// deliverable is solutions/<slug>/ with ≥N candidate options, a recommendation
// and an assumptions-and-evidence file; this package is the ONE deterministic,
// LLM-free engine that judges its shape (the build floor, the `evolve solution
// check` CLI and the audit gate line all project it). Judgment (rubric quality)
// stays in audit — never here.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func spec() Spec {
	return Spec{
		Root:          "solutions",
		MinOptions:    2,
		RequiredFiles: []string{"recommendation.md", "assumptions-and-evidence.md"},
		RequiredSections: map[string][]string{
			"recommendation.md":           {"Options Compared", "Recommendation"},
			"assumptions-and-evidence.md": {"Assumptions", "Evidence"},
		},
		ForbidPlaceholders: []string{"TBD", "TODO", "lorem ipsum"},
		EvidenceFile:       "assumptions-and-evidence.md",
	}
}

const evidenceRef = "see [A1](assumptions-and-evidence.md#a1)"

func writeSolution(t *testing.T, tree, slug string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(tree, "solutions", slug, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func goodSolution() map[string]string {
	return map[string]string{
		"options/1-device-bundling.md":  "# Option 1\n\nBundle device upgrades; margin +0.6pp " + evidenceRef + ".\n",
		"options/2-adaptive-bitrate.md": "# Option 2\n\nAdaptive bitrate on low-end devices; +0.5pp " + evidenceRef + ".\n",
		"recommendation.md":             "# Recommendation\n\n## Options Compared\n\n| option | margin | risk |\n|---|---|---|\n| 1 | +0.6 | low |\n| 2 | +0.5 | med |\n\n## Recommendation\n\nOption 1, " + evidenceRef + ".\n",
		"assumptions-and-evidence.md":   "# Assumptions and evidence\n\n## Assumptions\n\n- A1: device mix per region (source: 10-K).\n\n## Evidence\n\n- E1: cost-to-serve table.\n",
	}
}

func reasons(fs []Failure) string {
	var b strings.Builder
	for _, f := range fs {
		b.WriteString(f.Path + ": " + f.Reason + "\n")
	}
	return b.String()
}

func TestCheck_CompleteSolutionPasses(t *testing.T) {
	tree := t.TempDir()
	writeSolution(t, tree, "netflix-margin", goodSolution())
	if fs := Check(tree, "netflix-margin", spec()); len(fs) != 0 {
		t.Fatalf("complete solution must pass; got:\n%s", reasons(fs))
	}
}

func TestCheck_Failures(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(m map[string]string)
		want   string // substring of a failure reason
	}{
		{"one option only", func(m map[string]string) { delete(m, "options/2-adaptive-bitrate.md") }, "options"},
		{"recommendation missing", func(m map[string]string) { delete(m, "recommendation.md") }, "recommendation.md"},
		{"evidence file missing", func(m map[string]string) { delete(m, "assumptions-and-evidence.md") }, "assumptions-and-evidence.md"},
		{"required section missing", func(m map[string]string) {
			m["recommendation.md"] = "# Recommendation\n\n## Options Compared\n\nx " + evidenceRef + "\n"
		}, "Recommendation"},
		{"placeholder left behind", func(m map[string]string) {
			m["options/1-device-bundling.md"] = "# Option 1\n\nTBD " + evidenceRef + "\n"
		}, "placeholder"},
		{"option does not cite evidence", func(m map[string]string) {
			m["options/2-adaptive-bitrate.md"] = "# Option 2\n\nA number with no source: +0.5pp.\n"
		}, "assumptions-and-evidence.md"},
		{"empty option file", func(m map[string]string) { m["options/2-adaptive-bitrate.md"] = "" }, "empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := t.TempDir()
			files := goodSolution()
			tc.mutate(files)
			writeSolution(t, tree, "netflix-margin", files)
			fs := Check(tree, "netflix-margin", spec())
			if len(fs) == 0 {
				t.Fatalf("expected a failure mentioning %q", tc.want)
			}
			if !strings.Contains(reasons(fs), tc.want) {
				t.Errorf("failures do not mention %q:\n%s", tc.want, reasons(fs))
			}
		})
	}
}

func TestCheck_MissingSolutionDirIsOneLoudFailure(t *testing.T) {
	fs := Check(t.TempDir(), "nope", spec())
	if len(fs) != 1 || !strings.Contains(fs[0].Reason, "solutions/nope") {
		t.Fatalf("absent solution dir must be one loud failure naming the path; got:\n%s", reasons(fs))
	}
}

// TestFailure_String names the type's renderer (apicover) and pins the
// "path: reason" shape the build floor and the CLI print.
func TestFailure_String(t *testing.T) {
	f := Failure{Path: "solutions/x/recommendation.md", Reason: "missing"}
	if got := f.String(); got != "solutions/x/recommendation.md: missing" {
		t.Errorf("String() = %q", got)
	}
}

// TestDescribe: the contract prose the Task Contract renders is generated from
// the spec (one renderer, never hand-typed); a zero spec renders nothing.
func TestDescribe(t *testing.T) {
	got := Describe(spec())
	for _, want := range []string{"solutions/<slug>/", "at least 2 candidate", "recommendation.md (sections: Options Compared, Recommendation)", "assumptions-and-evidence.md", "placeholders forbidden: TBD, TODO, lorem ipsum", "evolve solution check solutions/<slug>"} {
		if !strings.Contains(got, want) {
			t.Errorf("Describe missing %q:\n%s", want, got)
		}
	}
	if Describe(Spec{}) != "" {
		t.Errorf("zero spec must render nothing")
	}
}

// TestCheck_HeadingInsideFenceDoesNotCount: the words of a required section
// pasted inside a code fence or an indented block are not structure.
func TestCheck_HeadingInsideFenceDoesNotCount(t *testing.T) {
	tree := t.TempDir()
	files := goodSolution()
	files["recommendation.md"] = "# R\n\n## Options Compared\n\nx " + evidenceRef + "\n\n```markdown\n## Recommendation\n```\n\n    ## Recommendation\n"
	writeSolution(t, tree, "netflix-margin", files)
	fs := Check(tree, "netflix-margin", spec())
	if !strings.Contains(reasons(fs), `"Recommendation" missing`) {
		t.Fatalf("a fenced/indented heading must not satisfy the section check; got:\n%s", reasons(fs))
	}
}

// TestCheck_SlugIsConfinedToTheIdVocabulary: a slug carrying a path segment,
// `..`, or characters outside the task-id vocabulary never reaches the
// filesystem — one loud failure at the engine, for every caller.
func TestCheck_SlugIsConfinedToTheIdVocabulary(t *testing.T) {
	tree := t.TempDir()
	writeSolution(t, tree, "netflix-margin", goodSolution())
	for _, bad := range []string{"../netflix-margin", "netflix-margin/../x", "Netflix_Margin", "", "a b"} {
		fs := Check(tree, bad, spec())
		if len(fs) != 1 || !strings.Contains(fs[0].Reason, "not a task id") {
			t.Errorf("slug %q must be one loud vocabulary failure; got:\n%s", bad, reasons(fs))
		}
	}
}

// TestCheck_MismatchedFenceMarkerStaysOpen: a `~~~` line or a shorter run does
// not close a backtick fence, so a heading after it is still fence content.
func TestCheck_MismatchedFenceMarkerStaysOpen(t *testing.T) {
	tree := t.TempDir()
	files := goodSolution()
	files["recommendation.md"] = "# R\n\n## Options Compared\n\nx " + evidenceRef + "\n\n````\ncode\n~~~\n```\n## Recommendation\n````\n"
	writeSolution(t, tree, "netflix-margin", files)
	if fs := Check(tree, "netflix-margin", spec()); !strings.Contains(reasons(fs), `"Recommendation" missing`) {
		t.Fatalf("a heading inside a fence closed by a mismatched marker must not count; got:\n%s", reasons(fs))
	}
}

// TestDescribe_PartialSections: a required file with no declared sections
// renders bare — no dangling "(sections: )".
func TestDescribe_PartialSections(t *testing.T) {
	s := spec()
	delete(s.RequiredSections, "assumptions-and-evidence.md")
	got := Describe(s)
	if strings.Contains(got, "assumptions-and-evidence.md (sections") || !strings.Contains(got, "recommendation.md (sections: Options Compared, Recommendation)") {
		t.Errorf("partial sections rendered wrong:\n%s", got)
	}
}
