package evalgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestMaterializationGate_RequiresACodeGraderPerEval pins the rule this
// gate's own remediation has always promised ("Each must contain at least
// one `[code]` grader") but never checked: an eval that exists with no
// [code] grader passed here, reached the build, and was refused by the
// lane's own durability test at the SHIP gate (cycle 1679, 2026-09-14) —
// after which the builder could not fix it (its sandbox denies .evolve/evals)
// and two repair rounds burned. The scout owns the eval; the gate must fail
// the scout while the scout still holds the pen.
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
	if rem := g.remediation(in); !strings.Contains(rem, evalPath) || !strings.Contains(rem, "[code]") {
		t.Fatalf("remediation must name the ungraded eval's path and the [code] rule; got %q", rem)
	}
	graded := ungraded + "\n### AC2: absent artifacts are INDETERMINATE [code]\n- run: cd go && go test -run TestIndeterminate ./internal/coherence/\n"
	if err := os.WriteFile(evalPath, []byte(graded), 0o644); err != nil {
		t.Fatal(err)
	}
	if msg, blocked := g.check(in); blocked {
		t.Fatalf("a graded eval must pass: %q", msg)
	}
}
