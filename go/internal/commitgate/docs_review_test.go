package commitgate

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestRequiredReview_ADocsOnlyChangeNeedsTheDocsReviewCapability(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		files     []string
		reviewers string
		want      bool
	}{
		{"docs-only with docs-review", []string{"docs/plans/a.md", "skills/x/SKILL.md"}, "docs-review", true},
		{"docs-only with a namespaced docs-review", []string{"README.md"}, "evo:docs-review", true},
		{"docs-only with only code reviewers", []string{"docs/plans/a.md"}, "code-simplifier,code-reviewer", false},
		{"docs-only with nothing", []string{"docs/plans/a.md"}, "", false},
		{"a Go file with only docs-review", []string{"docs/a.md", "go/x.go"}, "docs-review", false},
		{"a Go file with the code chain", []string{"docs/a.md", "go/x.go"}, "code-simplifier,go-reviewer", true},
		{"a JSON file is not docs-only", []string{"docs/a.md", ".evolve/policy.json"}, "docs-review", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := Options{Reviewers: tc.reviewers}
			res := &Result{}
			if got := o.requiredReviewSatisfied(tc.files, detectLangs(tc.files), res); got != tc.want {
				t.Fatalf("requiredReviewSatisfied(%v, %q) = %v, want %v; log %v", tc.files, tc.reviewers, got, tc.want, res.Logs)
			}
		})
	}
}

func TestRequiredReview_ADeniedDocsOnlyChangeNamesTheDocsReviewCapability(t *testing.T) {
	t.Parallel()
	o := Options{Reviewers: "code-simplifier,code-reviewer"}
	res := &Result{}
	if o.requiredReviewSatisfied([]string{"docs/a.md"}, nil, res) {
		t.Fatal("a docs-only change passed with only code reviewers")
	}
	log := strings.Join(res.Logs, "\n")
	for _, want := range []string{"missing required review capability: docs review", "docs review ← docs-review"} {
		if !strings.Contains(log, want) {
			t.Fatalf("log lacks %q:\n%s", want, log)
		}
	}
}

func TestIsDocsOnly_EveryPathMustBeMarkdown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		files []string
		want  bool
	}{
		{[]string{"docs/a.md", "CLAUDE.md"}, true},
		{[]string{"docs/a.md", "docs/b.png"}, false},
		{[]string{"docs/a.mdx"}, false},
		{[]string{"README.MD"}, false},
		{nil, false},
	}
	for _, tc := range tests {
		if got := isDocsOnly(tc.files); got != tc.want {
			t.Fatalf("isDocsOnly(%v) = %v, want %v", tc.files, got, tc.want)
		}
	}
}

func docsGate(t *testing.T, reviewers string, rules ...scriptRule) Options {
	t.Helper()
	o, _, _ := gateF3(t, "README.md", []string{"shasum"}, rules...)
	o.Reviewers = reviewers
	return o
}

func TestRun_ADocsOnlyChangeCommitsWithTheDocsReviewAloneAndRecordsIt(t *testing.T) {
	t.Parallel()
	o := docsGate(t, "docs-review")

	res := o.Run(context.Background())

	if res.ExitCode != ExitPass || res.Attestation == nil || !reflect.DeepEqual(res.Attestation.ReviewersRun, []string{"docs-review"}) {
		t.Fatalf("ExitCode=%d attestation=%+v logs=%v", res.ExitCode, res.Attestation, res.Logs)
	}
}

func TestRun_ADocsOnlyChangeWithOnlyCodeReviewersIsDenied(t *testing.T) {
	t.Parallel()
	o := docsGate(t, "code-simplifier,code-reviewer")

	res := o.Run(context.Background())

	if res.ExitCode != ExitFail || res.Attestation != nil {
		t.Fatalf("ExitCode=%d attestation=%+v logs=%v", res.ExitCode, res.Attestation, res.Logs)
	}
}

func TestRun_ACodeFileRenamedToMarkdownStillNeedsTheCodeReview(t *testing.T) {
	t.Parallel()
	o := docsGate(t, "docs-review", scriptRule{matchPrefix: "git diff --name-only --no-renames -z HEAD", stdout: "foo.go\x00README.md\x00"})

	res := o.Run(context.Background())

	if res.ExitCode != ExitFail || !strings.Contains(strings.Join(res.Logs, "\n"), "missing required review capability: simplify review") {
		t.Fatalf("ExitCode=%d logs=%v", res.ExitCode, res.Logs)
	}
}

func TestRun_AnUnlistableChangeIsRefusedBeforeTheReviewStep(t *testing.T) {
	t.Parallel()
	o := docsGate(t, "docs-review", scriptRule{matchPrefix: "git diff --name-only --no-renames -z HEAD", exit: 2})

	res := o.Run(context.Background())

	if res.ExitCode == ExitPass || res.Attestation != nil {
		t.Fatalf("ExitCode=%d attestation=%+v logs=%v", res.ExitCode, res.Attestation, res.Logs)
	}
}

func TestListedChange_AListingErrorGivesNoPathSoTheCodeReviewApplies(t *testing.T) {
	t.Parallel()
	o := docsGate(t, "docs-review", scriptRule{matchPrefix: "git diff --name-only --no-renames -z HEAD", exit: 2})

	paths := o.listedChange(context.Background())

	if paths != nil || isDocsOnly(paths) {
		t.Fatalf("listedChange = %v, want nil", paths)
	}
}
