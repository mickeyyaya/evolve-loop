package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
)

type trailerCommit struct {
	MessageHeadline string `json:"messageHeadline"`
	MessageBody     string `json:"messageBody"`
}

func trailerPRView(t *testing.T, body string, commits ...trailerCommit) string {
	t.Helper()
	view := map[string]any{
		"state": "OPEN", "isDraft": false, "mergeable": "MERGEABLE",
		"headRefOid": "h12", "baseRefName": "main", "body": body, "commits": commits,
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func trailerStageMerger(t *testing.T, stage, view string) (prMerger, *fakePRGitHub, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	seedCurationItems(t, root, map[string]string{
		"open-item.json":     `{"id":"open-item"}`,
		"consumed/done.json": `{"id":"done-item"}`,
	})
	gh := &fakePRGitHub{
		views:  map[string]string{"12": view},
		behind: map[string]string{"h12": `{"behind_by":0}`},
		merged: map[string]string{"12": `{"state":"MERGED","mergeCommit":{"oid":"` + mergeSHA + `"}}`},
	}
	green := func(context.Context, string) (ciwatch.RunStatus, error) {
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}, nil
	}
	m, stdout, stderr := greenMerger(gh, green, nil)
	m.root = root
	m.trailerStage = stage
	return m, gh, stdout, stderr
}

func TestPRMerge_RefusesAPRWithoutATrailerInEnforce(t *testing.T) {
	t.Parallel()
	view := trailerPRView(t, "A change that names no inbox item.", trailerCommit{"feat: a change", "It names no inbox item either."})
	m, gh, stdout, stderr := trailerStageMerger(t, "enforce", view)

	code := m.mergeAll(context.Background(), []string{"12", "13"})

	if code != cliExitRefused || stdout.Len() != 0 {
		t.Fatalf("exit %d stdout %q stderr %q, want exit %d and no merged line", code, stdout.String(), stderr.String(), cliExitRefused)
	}
	if !strings.Contains(stderr.String(), "no Closes-Inbox or Refs-Inbox trailer") || !strings.Contains(stderr.String(), "not attempted: #13") {
		t.Errorf("stderr %q, want the missing trailer named and #13 not attempted", stderr.String())
	}
	for _, call := range gh.calls {
		if strings.HasPrefix(call, "pr merge") || strings.HasPrefix(call, "pr view 13") {
			t.Errorf("an unlabelled PR must merge nothing and touch no later PR: %s", call)
		}
	}
}

func TestPRMerge_RefusesAnIdThatIsNotOpenInEnforce(t *testing.T) {
	t.Parallel()
	view := trailerPRView(t, "Closes-Inbox: done-item")
	m, gh, stdout, stderr := trailerStageMerger(t, "enforce", view)

	code := m.mergeAll(context.Background(), []string{"12"})

	if code != cliExitRefused || stdout.Len() != 0 || !strings.Contains(stderr.String(), "done-item is not an open inbox item") {
		t.Fatalf("exit %d stdout %q stderr %q, want a refusal naming done-item", code, stdout.String(), stderr.String())
	}
	if len(callsWith(gh.calls, "pr merge")) != 0 {
		t.Errorf("a PR naming a closed item must not merge: %v", gh.calls)
	}
}

func TestPRMerge_AcceptsADeclaredTrailerInEnforce(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body string
		commits    []trailerCommit
	}{
		{"none in the PR body", "chore: tidy\n\nCloses-Inbox: none", nil},
		{"an open id in the PR body", "feat: a change\n\nCloses-Inbox: open-item", nil},
		{"a Refs-Inbox id in the PR body", "feat: a change\n\nRefs-Inbox: open-item", nil},
		{"an open id in one commit message", "A change without a trailer.", []trailerCommit{
			{"feat: part one", "Part one."},
			{"feat: part two", "Closes-Inbox: open-item"},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m, gh, stdout, stderr := trailerStageMerger(t, "enforce", trailerPRView(t, c.body, c.commits...))

			code := m.mergeAll(context.Background(), []string{"12"})

			if code != 0 || stdout.String() != "merged #12 "+mergeSHA+"\n" {
				t.Fatalf("exit %d stdout %q stderr %q, want a clean merge", code, stdout.String(), stderr.String())
			}
			if len(callsWith(gh.calls, "pr merge 12 --squash --match-head-commit h12")) != 1 {
				t.Errorf("want one head-bound squash merge, calls:\n%s", strings.Join(gh.calls, "\n"))
			}
		})
	}
}

func TestPRMerge_ShadowStageWarnsAndStillMerges(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"shadow", ""} {
		t.Run("stage "+stage, func(t *testing.T) {
			t.Parallel()
			m, gh, stdout, stderr := trailerStageMerger(t, stage, trailerPRView(t, "A change with no trailer."))

			code := m.mergeAll(context.Background(), []string{"12"})

			if code != 0 || stdout.String() != "merged #12 "+mergeSHA+"\n" {
				t.Fatalf("exit %d stdout %q stderr %q, want the merge to go on under shadow", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), "shadow: would refuse: no Closes-Inbox or Refs-Inbox trailer") {
				t.Errorf("stderr %q, want the shadow warning", stderr.String())
			}
			if len(callsWith(gh.calls, "pr merge")) != 1 {
				t.Errorf("shadow must still merge once: %v", gh.calls)
			}
		})
	}
}

func TestPRMerge_ReadsTheBodyAndCommitsOfThePR(t *testing.T) {
	t.Parallel()
	m, gh, _, _ := trailerStageMerger(t, "shadow", trailerPRView(t, "body"))

	m.mergeAll(context.Background(), []string{"12"})

	views := callsWith(gh.calls, "pr view 12 --json")
	if len(views) == 0 || !strings.Contains(views[0], "body") || !strings.Contains(views[0], "commits") {
		t.Errorf("the first PR view must ask for body and commits, got %v", views)
	}
}

func TestNewPRMerger_CarriesTheTrailerStageOfPolicy(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, policy, want string }{
		{"enforce", `{"inbox":{"curator":{"trailer":{"stage":"enforce"}}}}`, "enforce"},
		{"default", `{}`, "shadow"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(c.policy), 0o644); err != nil {
				t.Fatal(err)
			}

			m, err := newPRMerger(root, prMergeArgs{}, &bytes.Buffer{}, &bytes.Buffer{})

			if err != nil || m.trailerStage != c.want {
				t.Errorf("newPRMerger trailerStage = %q, err %v, want %q", m.trailerStage, err, c.want)
			}
		})
	}
}
