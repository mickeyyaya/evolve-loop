package commitgate

import (
	"reflect"
	"strings"
	"testing"
)

const (
	codeLogHead = "[commit-gate] DENY: missing required review capability: "
	codeLogSyn1 = "[commit-gate]   simplify ← code-simplifier | code-review-simplify | refactor"
	codeLogSyn2 = "[commit-gate]   review   ← code-reviewer | code-review | a matching <lang>-reviewer (ECC variants OK)"
	codeLogTail = "[commit-gate] run them, then pass --reviewers (use the /commit skill)."
)

func codeDeny(missing string) []string {
	return []string{codeLogHead + missing, codeLogSyn1, codeLogSyn2, codeLogTail}
}

func TestRequiredReviewSatisfied_LogsExactlyWhatItLoggedBeforeTheExtraction(t *testing.T) {
	t.Parallel()
	docsDeny := []string{
		"[commit-gate] DENY: missing required review capability: docs review",
		"[commit-gate]   docs review ← docs-review (every changed path is Markdown)",
		"[commit-gate] run it, then pass --reviewers (use the /commit skill).",
	}
	cases := []struct {
		name      string
		reviewers string
		files     []string
		langs     []string
		ok        bool
		logs      []string
	}{
		{"nothing run", "", []string{"x.go"}, []string{"go"}, false, codeDeny("simplify review")},
		{"simplifier only", "code-simplifier", []string{"x.go"}, []string{"go"}, false, codeDeny("review")},
		{"reviewer only", "go-reviewer", []string{"x.go"}, []string{"go"}, false, codeDeny("simplify")},
		{"go reviewer does not serve python", "code-simplifier,go-reviewer", []string{"x.py"}, []string{"python"}, false, codeDeny("review")},
		{"both", "code-simplifier,go-reviewer", []string{"x.go"}, []string{"go"}, true, nil},
		{"one agent for both", "code-review-simplify", []string{"x.go"}, []string{"go"}, true, nil},
		{"namespaced and spaced", " evo:code-simplifier , ecc:code-reviewer", []string{"x.go"}, nil, true, nil},
		{"docs, no run", "code-simplifier,go-reviewer", []string{"a.md"}, nil, false, docsDeny},
		{"docs, docs-review", "docs-review", []string{"a.md", "b.md"}, nil, true, nil},
		{"no listed files take the code rule", "", nil, nil, false, codeDeny("simplify review")},
	}
	for _, c := range cases {
		res := &Result{}
		ok := Options{Reviewers: c.reviewers}.requiredReviewSatisfied(c.files, c.langs, res)
		if ok != c.ok || !reflect.DeepEqual(res.Logs, c.logs) {
			t.Errorf("%s: ok %v logs %q, want %v %q", c.name, ok, strings.Join(res.Logs, "|"), c.ok, strings.Join(c.logs, "|"))
		}
	}
}

func TestReviewersSatisfied_LogsExactlyWhatItLoggedBeforeTheExtraction(t *testing.T) {
	t.Parallel()
	res := &Result{}
	if (Options{Reviewers: "code-reviewer"}).reviewersSatisfied(nil, res) || !reflect.DeepEqual(res.Logs, codeDeny("simplify")) {
		t.Errorf("logs %q", res.Logs)
	}
	res = &Result{}
	if !(Options{Reviewers: "refactor,typescript-review"}).reviewersSatisfied([]string{"ts"}, res) || len(res.Logs) != 0 {
		t.Errorf("logs %q", res.Logs)
	}
}

func TestMissingReviewCapabilities_NamesWhatTheReviewersDoNotProve(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		files     []string
		langs     []string
		reviewers []string
		want      []string
	}{
		{"nothing", []string{"x.go"}, []string{"go"}, nil, []string{"simplify", "review"}},
		{"simplifier only", []string{"x.go"}, []string{"go"}, []string{"code-simplifier"}, []string{"review"}},
		{"reviewer only", []string{"x.go"}, []string{"go"}, []string{"go-reviewer"}, []string{"simplify"}},
		{"wrong language reviewer", []string{"x.py"}, []string{"python"}, []string{"code-simplifier", "go-reviewer"}, []string{"review"}},
		{"both", []string{"x.go"}, []string{"go"}, []string{"code-simplifier", "go-reviewer"}, nil},
		{"namespaced and spaced", []string{"x.go"}, nil, []string{" evo:code-simplifier ", "ecc:code-reviewer"}, nil},
		{"docs, wrong seats", []string{"a.md"}, nil, []string{"code-simplifier", "go-reviewer"}, []string{"docs review"}},
		{"docs, docs-review", []string{"a.md"}, nil, []string{"docs-review"}, nil},
		{"no files take the code rule", nil, nil, nil, []string{"simplify", "review"}},
	}
	for _, c := range cases {
		if got := MissingReviewCapabilities(c.files, c.langs, c.reviewers); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
