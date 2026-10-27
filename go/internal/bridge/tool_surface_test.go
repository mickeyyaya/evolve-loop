package bridge

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
)

func TestPreparePrompt_GivesTheCellItsToolSurfaceFromTheClassTable(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "the task")
	out, err := preparePrompt(&Config{PromptFile: pf, Workspace: ws, Agent: "build"}, Deps{})
	if err != nil {
		t.Fatal(err)
	}
	if out != "the task\n\n"+agentclass.Brief(ws) {
		t.Fatalf("the prompt must keep the task and end with the tool surface of the cell, got %q", out)
	}
	if !strings.Contains(out, filepath.Join(ws, "requests")) {
		t.Fatalf("the tool surface must name the request directory of this cell, got %q", out)
	}
}

func TestPreparePrompt_APaneWithNoAgentGetsNoToolSurface(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "probe body")
	out, err := preparePrompt(&Config{PromptFile: pf, Workspace: ws}, Deps{})
	if err != nil || out != "probe body" {
		t.Fatalf("a probe pane has no phase cell; the prompt must be the body alone, got %q %v", out, err)
	}
}
