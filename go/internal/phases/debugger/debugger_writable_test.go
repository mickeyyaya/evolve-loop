package debugger

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestHooksComposePrompt_NamesTheConflictedPathsTheDebuggerMayWrite(t *testing.T) {
	for want, paths := range map[string][]string{
		"- conflicted_paths: go/.apicover-enforce\n":            {"go/.apicover-enforce"},
		"- conflicted_paths: go/.apicover-enforce, docs/a.md\n": {"go/.apicover-enforce", "docs/a.md"},
	} {
		req := core.PhaseRequest{Worktree: "/worktree", WorktreeReadOnly: true, WorktreeWritablePaths: paths}
		if prompt := (hooks{}).ComposePrompt("body", req); !strings.Contains(prompt, want) {
			t.Errorf("prompt does not name %v as %q:\n%s", paths, want, prompt)
		}
	}
	if other := (hooks{}).ComposePrompt("body", core.PhaseRequest{Worktree: "/worktree"}); strings.Contains(other, "conflicted_paths") {
		t.Errorf("a dispatch with no writable path names none:\n%s", other)
	}
}
