package evalgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// writeScoutReport writes a scout-report.md selecting the given slugs (prose form).
func writeScoutReport(t *testing.T, workspace string, slugs ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("## Selected Tasks\n\n")
	for i, s := range slugs {
		b.WriteString("### Task ")
		b.WriteByte(byte('1' + i))
		b.WriteString("\n- **Slug:** " + s + "\n\n")
	}
	if err := os.WriteFile(filepath.Join(workspace, scoutReportName), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write scout-report: %v", err)
	}
}

// writeEval writes <projectRoot>/.evolve/evals/<slug>.md with a bash grader body.
func writeEval(t *testing.T, projectRoot, slug, bashBody string) {
	t.Helper()
	dir := filepath.Join(projectRoot, ".evolve", "evals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir evals: %v", err)
	}
	body := "# Eval " + slug + "\n\n```bash\n" + bashBody + "\n```\n"
	if err := os.WriteFile(filepath.Join(dir, slug+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write eval: %v", err)
	}
}

func TestMaterializationGate(t *testing.T) {
	t.Run("all evals present → pass", func(t *testing.T) {
		ws, root := t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, "a", "b")
		writeEval(t, root, "a", "go test ./internal/a/...")
		writeEval(t, root, "b", "go test ./internal/b/...")
		reason, block := materializationGate{}.check(core.ReviewInput{Phase: "scout", Workspace: ws, ProjectRoot: root})
		if reason != "" || block {
			t.Errorf("want pass; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("missing eval → certain block, names the slug", func(t *testing.T) {
		ws, root := t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, "a", "b")
		writeEval(t, root, "a", "go test ./internal/a/...") // b missing
		reason, block := materializationGate{}.check(core.ReviewInput{Phase: "scout", Workspace: ws, ProjectRoot: root})
		if !block {
			t.Fatalf("want block; got reason=%q block=%v", reason, block)
		}
		if !strings.Contains(reason, "b") || strings.Contains(reason, " a,") {
			t.Errorf("reason should name only the missing slug b; got %q", reason)
		}
	})

	t.Run("eval found in workspace fallback → pass", func(t *testing.T) {
		ws, root := t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, "a")
		writeEval(t, ws, "a", "go test ./...") // eval lives under workspace, not project root
		reason, block := materializationGate{}.check(core.ReviewInput{Phase: "scout", Workspace: ws, ProjectRoot: root})
		if reason != "" || block {
			t.Errorf("workspace-fallback eval should pass; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("eval in cycle worktree is invisible → block", func(t *testing.T) {
		ws, root, wt := t.TempDir(), t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, "a")
		writeEval(t, wt, "a", "go test ./...") // mirrors an agent cwd inside .evolve/worktrees/cycle-*
		reason, block := materializationGate{}.check(core.ReviewInput{Phase: "scout", Workspace: ws, ProjectRoot: root})
		if !block || !strings.Contains(reason, "a") {
			t.Errorf("worktree-local eval must not satisfy Gate A; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("no report → fail-open", func(t *testing.T) {
		reason, block := materializationGate{}.check(core.ReviewInput{Phase: "scout", Workspace: t.TempDir(), ProjectRoot: t.TempDir()})
		if reason != "" || block {
			t.Errorf("absent report must fail open; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("zero selected slugs → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, scoutReportName), []byte("## Gap Analysis\nconvergence\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		reason, block := materializationGate{}.check(core.ReviewInput{Phase: "scout", Workspace: ws, ProjectRoot: t.TempDir()})
		if reason != "" || block {
			t.Errorf("no slugs must fail open; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("appliesTo scout only", func(t *testing.T) {
		g := materializationGate{}
		if !g.appliesTo("scout") || g.appliesTo("tdd") || g.appliesTo("build") {
			t.Error("Gate A must apply to scout only")
		}
	})
}

func TestQualityGate(t *testing.T) {
	t.Run("tautology eval → certain block", func(t *testing.T) {
		ws, root := t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, "taut")
		writeEval(t, root, "taut", ":") // no-op tautology → LevelHalt
		reason, block := qualityGate{}.check(core.ReviewInput{Phase: "tdd", Workspace: ws, ProjectRoot: root})
		if !block || !strings.Contains(reason, "taut") {
			t.Errorf("tautology must block + name slug; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("weak (echo) eval → advisory, never blocks", func(t *testing.T) {
		ws, root := t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, "weak")
		writeEval(t, root, "weak", "echo checking")
		reason, block := qualityGate{}.check(core.ReviewInput{Phase: "tdd", Workspace: ws, ProjectRoot: root})
		if block {
			t.Errorf("weak eval must be advisory (block=false); got reason=%q block=%v", reason, block)
		}
		if reason == "" {
			t.Error("weak eval should still surface an advisory reason")
		}
	})

	t.Run("behavioral eval → pass", func(t *testing.T) {
		ws, root := t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, "real")
		writeEval(t, root, "real", "go test -race ./internal/real/...")
		reason, block := qualityGate{}.check(core.ReviewInput{Phase: "tdd", Workspace: ws, ProjectRoot: root})
		if reason != "" || block {
			t.Errorf("behavioral eval must pass; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("missing eval is Gate A's job → fail-open here", func(t *testing.T) {
		ws, root := t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, "gone") // no eval written
		reason, block := qualityGate{}.check(core.ReviewInput{Phase: "tdd", Workspace: ws, ProjectRoot: root})
		if reason != "" || block {
			t.Errorf("missing eval must fail open in Gate B; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("appliesTo tdd only", func(t *testing.T) {
		g := qualityGate{}
		if !g.appliesTo("tdd") || g.appliesTo("scout") || g.appliesTo("build") {
			t.Error("Gate B must apply to tdd only")
		}
	})
}
