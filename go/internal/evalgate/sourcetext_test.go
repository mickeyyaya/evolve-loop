package evalgate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestSourceTextGate_WiredIntoReviewer(t *testing.T) {
	worktree := t.TempDir()
	dir := filepath.Join(worktree, "go", "acs", "cycle4243")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package cycle4243\n\nimport (\n\t\"testing\"\n\n\t\"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert\"\n)\n\nfunc TestC4243_Grep(t *testing.T) {\n\tacsassert.FileContains(t, \"a\", \"b\")\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(t.TempDir(), "cycle-4243")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	in := core.ReviewInput{Phase: "tdd", Workspace: workspace, Worktree: worktree, ProjectRoot: t.TempDir()}
	res := NewReviewer(config.StageEnforce).Review(context.Background(), in)
	if res.Approve || !strings.Contains(res.Reason, "TestC4243_Grep") {
		t.Errorf("result = %+v, want refusal naming TestC4243_Grep", res)
	}
}
