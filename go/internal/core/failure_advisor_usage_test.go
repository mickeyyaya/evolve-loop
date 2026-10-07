package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFailureAdvisor_PromptCarriesTheUsageEvidenceRecordedForTheFailure(t *testing.T) {
	ws := t.TempDir()
	record := `{"origin":"Bridge.Launch","phase":"build","driver":"agy-claude-tmux","trigger":"exit 81","exit_code":81,"summary":"quota exhausted, verified by a usage query: CLAUDE AND GPT MODELS 5h window 100% used, resets 2h 15m"}` + "\n" +
		`{"origin":"Bridge.Launch","phase":"build","driver":"claude-tmux","trigger":"exit 80","exit_code":80,"summary":"quota ruled out by a usage query: session session window 7% used"}` + "\n"
	if err := os.WriteFile(filepath.Join(ws, UsageEvidenceFile), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}

	prompt := NewFailureAdvisor(nil).composePrompt(FailureAdviseInput{Phase: "build", CLI: "agy-claude-tmux", ExitCode: 81, Workspace: ws}, filepath.Join(ws, "failure-advice.json"))

	for _, want := range []string{"# Usage evidence", "agy-claude-tmux (exit 81): quota exhausted", "claude-tmux (exit 80): quota ruled out"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the advisor prompt lacks %q:\n%s", want, prompt)
		}
	}
	if bare := NewFailureAdvisor(nil).composePrompt(FailureAdviseInput{Workspace: t.TempDir()}, "x"); strings.Contains(bare, "# Usage evidence") {
		t.Errorf("a workspace with no usage record must not grow an empty section")
	}
}
