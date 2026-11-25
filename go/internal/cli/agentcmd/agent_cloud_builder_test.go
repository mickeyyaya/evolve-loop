package agentcmd

import (
	"strings"
	"testing"
)

func TestAgentRun_TheCloudBuildSeatIsAdmittedAsAWorktreeWriterAndGetsAPullRequestPrompt(t *testing.T) {
	root := project(t)
	wt := linkedWorktree(t)
	fb := &fakeBridge{artifact: builderReport}
	trees := &treeStates{values: []string{"before", "after"}}
	args := []string{"run", "cloud-builder", "--project-root", root, "--json", "--worktree", wt, "--writable", "go/x.go"}

	code, _, stderr := runJSON(t, seatDeps(fb, trees), args, "build item X")

	if code != 0 || len(fb.prompts) != 1 {
		t.Fatalf("exit %d launches %d stderr %q, want 0 and one launch", code, len(fb.prompts), stderr)
	}
	prompt := fb.prompts[0]
	for _, want := range []string{"Open one pull request", "In a cloud session", "In a local seat", "Do not write the report to a file of the repository"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("rendered prompt lacks %q", want)
		}
	}
	for _, banned := range []string{"Never run git commit, push", "Never run git commit"} {
		if strings.Contains(prompt, banned) {
			t.Errorf("rendered prompt holds the ban %q that contradicts the pull request", banned)
		}
	}
}
