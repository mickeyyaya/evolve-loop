package evalgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestMaterializationGate_RequiresACodeGraderPerEval(t *testing.T) {
	root, ws := scoutWorkspaceSelecting(t, "crossartifact-invariant-stack")
	evalPath := filepath.Join(root, ".evolve", "evals", "crossartifact-invariant-stack.md")
	if err := os.MkdirAll(filepath.Dir(evalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	ungraded := "---\nscore_cap:\n  - criterion: \"the aggregate stays advisory\"\n    max_if_missing: 8\n    evidence: \"cd go && go test ./internal/coherence/...\"\n---\n\n## Acceptance Criteria\n\n### AC1: the aggregate evaluates all four classes\n"
	if err := os.WriteFile(evalPath, []byte(ungraded), 0o644); err != nil {
		t.Fatal(err)
	}
	in := core.ReviewInput{Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws}
	g := materializationGate{}
	msg, blocked := g.check(in)
	if !blocked || !strings.Contains(msg, "crossartifact-invariant-stack") || !strings.Contains(msg, "[code]") {
		t.Fatalf("an eval without a [code] grader must block the scout and name the slug and the rule; got blocked=%v msg=%q", blocked, msg)
	}
	wsEvalPath := filepath.Join(ws, ".evolve", "evals", "crossartifact-invariant-stack.md")
	if rem := g.remediation(in); !strings.Contains(rem, wsEvalPath) || strings.Contains(rem, evalPath) || !strings.Contains(rem, "[code]") {
		t.Fatalf("remediation must name the writable workspace path and the [code] rule, never the deny-write root path; got %q", rem)
	}
	graded := ungraded + "\n### AC2: absent artifacts are INDETERMINATE [code]\n- run: cd go && go test -run TestIndeterminate ./internal/coherence/\n"
	if err := os.WriteFile(evalPath, []byte(graded), 0o644); err != nil {
		t.Fatal(err)
	}
	if msg, blocked := g.check(in); blocked {
		t.Fatalf("a graded eval must pass: %q", msg)
	}
}

func TestMaterializationGate_WorkspaceGradedEvalSatisfiesAnUngradedRootEval(t *testing.T) {
	root, ws := scoutWorkspaceSelecting(t, "ws-overrides-root")
	for dir, body := range map[string]string{
		root: "# Eval\n\nexistence only\n",
		ws:   "# Eval\n\n### AC1: behaves [code]\n- run: cd go && go test -run TestX ./internal/x/\n",
	} {
		p := filepath.Join(dir, ".evolve", "evals", "ws-overrides-root.md")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	in := core.ReviewInput{Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws}
	if msg, blocked := (materializationGate{}).check(in); blocked {
		t.Fatalf("following the remediation (a graded eval at the workspace path) must satisfy the gate; got %q", msg)
	}
}
