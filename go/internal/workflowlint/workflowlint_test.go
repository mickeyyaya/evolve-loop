package workflowlint_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/workflowlint"
)

const workflowPath = ".github/workflows/ci.yml"

const validWorkflow = "name: ci\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n  reuse:\n    uses: ./.github/workflows/x.yml\n"

func check(path, body string) []workflowlint.Finding {
	return workflowlint.Check([]workflowlint.File{{Path: path, Data: []byte(body)}})
}

func TestCheck_AcceptsAValidWorkflowAndAPlainYAMLFile(t *testing.T) {
	t.Parallel()
	cases := map[string][]workflowlint.File{
		"a workflow":        {{Path: workflowPath, Data: []byte(validWorkflow)}},
		"a plain yaml file": {{Path: "config/app.yaml", Data: []byte("a: 1\nb: [x]\n")}},
		"a plain file in a nested workflows directory": {{Path: "examples/.github/workflows/x.yml", Data: []byte("a: 1\n")}},
		"a plain empty file":                           {{Path: "config/empty.yml", Data: nil}},
		"a multi-doc file":                             {{Path: "k8s/all.yml", Data: []byte("a: 1\n---\nb: 2\n")}},
		"no files":                                     nil,
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := workflowlint.Check(files); len(got) != 0 {
				t.Errorf("Check = %v, want no finding", got)
			}
		})
	}
}

func TestCheck_ReportsTheRulesThatAWorkflowBreaks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want workflowlint.Finding
	}{
		{"a syntax error", "on: push\njobs: a: b\n", workflowlint.Finding{Path: workflowPath, Line: 2, Msg: "invalid YAML: "}},
		{"no on key", "jobs:\n  a:\n    runs-on: x\n", workflowlint.Finding{Path: workflowPath, Line: 1, Msg: "missing key on"}},
		{"no jobs key", "on: push\n", workflowlint.Finding{Path: workflowPath, Line: 1, Msg: "missing key jobs"}},
		{"empty jobs", "on: push\njobs: {}\n", workflowlint.Finding{Path: workflowPath, Line: 2, Msg: "jobs must be a non-empty mapping"}},
		{"jobs as a list", "on: push\njobs:\n  - a\n", workflowlint.Finding{Path: workflowPath, Line: 3, Msg: "jobs must be a non-empty mapping"}},
		{"a job with no runner", "on: push\njobs:\n  a:\n    name: x\n", workflowlint.Finding{Path: workflowPath, Line: 4, Msg: "job a needs runs-on or uses"}},
		{"a job that is not a mapping", "on: push\njobs:\n  a: nope\n", workflowlint.Finding{Path: workflowPath, Line: 3, Msg: "job a needs runs-on or uses"}},
		{"a list at the top", "- a\n", workflowlint.Finding{Path: workflowPath, Line: 1, Msg: "workflow must be a mapping"}},
		{"an empty file", "", workflowlint.Finding{Path: workflowPath, Line: 1, Msg: "workflow file has no document"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := check(workflowPath, tc.body)
			if len(got) != 1 || got[0].Path != tc.want.Path || got[0].Line != tc.want.Line || !startsWith(got[0].Msg, tc.want.Msg) {
				t.Errorf("Check = %+v, want one finding %+v", got, tc.want)
			}
		})
	}
}

func startsWith(s, prefix string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }

func TestCheck_ReportsEveryFindingOfEveryFileInOrder(t *testing.T) {
	t.Parallel()
	files := []workflowlint.File{
		{Path: "a.yml", Data: []byte("ok: 1\nb: c: d\n")},
		{Path: ".github/workflows/b.yml", Data: []byte("jobs:\n  x:\n    name: y\n  z:\n    name: w\n")},
		{Path: "c.yaml", Data: []byte("fine: true\n")},
	}

	got := workflowlint.Check(files)

	var paths []string
	for _, f := range got {
		paths = append(paths, f.Path)
	}
	want := []string{"a.yml", ".github/workflows/b.yml", ".github/workflows/b.yml", ".github/workflows/b.yml"}
	if !slices.Equal(paths, want) {
		t.Errorf("finding paths = %v, want %v (missing on, then one per runnerless job)", paths, want)
	}
}

func TestCheck_AcceptsJobsThatInheritTheirRunnerThroughAnAnchor(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"a merge key":  "on: push\ndefaults: &defaults\n  runs-on: ubuntu-latest\njobs:\n  a:\n    <<: *defaults\n    name: x\n",
		"an alias job": "on: push\nshared: &job\n  runs-on: ubuntu-latest\njobs:\n  a: *job\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := check(workflowPath, body); len(got) != 0 {
				t.Errorf("Check = %v, want no finding", got)
			}
		})
	}
}

func TestCheck_ChecksEveryDocumentOfAWorkflow(t *testing.T) {
	t.Parallel()

	got := check(workflowPath, validWorkflow+"---\non: push\n")

	if len(got) != 1 || got[0].Msg != "missing key jobs" || got[0].Line != 9 {
		t.Errorf("Check = %+v, want the second document to miss jobs on line 9", got)
	}
}

func TestFinding_StringIsPathLineMessage(t *testing.T) {
	t.Parallel()

	got := workflowlint.Finding{Path: "a/b.yml", Line: 7, Msg: "boom"}.String()

	if got != "a/b.yml:7: boom" {
		t.Errorf("String = %q, want a/b.yml:7: boom", got)
	}
}
