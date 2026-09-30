//go:build acs

package cycle1594

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

	evalSlug = "gitignore-staging-sweep"
)

func runNamedTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Errorf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			name, pkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("%s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s", name, stdout)
	}
}

func TestC1594_001_binding_tree_excludes_unrelated_untracked_residue(t *testing.T) {
	runNamedTest(t, corePkg, "TestWorktreeContentSHA_ExcludesUnrelatedUntrackedResidue")
}

func TestC1594_002_binding_tree_retains_declared_new_file(t *testing.T) {
	runNamedTest(t, corePkg, "TestWorktreeContentSHA_StagesDeclaredNewFile")
}

func TestC1594_003_binding_tree_captures_unstaged_tracked_modification(t *testing.T) {
	runNamedTest(t, corePkg, "TestWorktreeContentSHA_CapturesUnstagedTrackedModification")
}

func TestC1594_004_residue_only_worktree_keeps_base_identity(t *testing.T) {
	runNamedTest(t, corePkg, "TestWorktreeContentSHA_ResidueOnlyWorktreeKeepsBaseIdentity")
}

func TestC1594_005_production_audit_binding_path_emits_scoped_tree(t *testing.T) {
	runNamedTest(t, corePkg, "TestEmitPhaseBindings_AuditBindingTreeExcludesUnrelatedResidue")
}

type scoreCap struct {
	criterion string
	evidence  string
}

func parseScoreCaps(t *testing.T, path string) []scoreCap {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read eval %s: %v", path, err)
	}
	var caps []scoreCap
	var cur *scoreCap
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- criterion:"):
			caps = append(caps, scoreCap{criterion: unquote(strings.TrimPrefix(trimmed, "- criterion:"))})
			cur = &caps[len(caps)-1]
		case strings.HasPrefix(trimmed, "evidence:") && cur != nil:
			cur.evidence = unquote(strings.TrimPrefix(trimmed, "evidence:"))
		case trimmed == "---" && len(caps) > 0:
			return caps
		}
	}
	return caps
}

func unquote(s string) string {
	return strings.Trim(strings.TrimSpace(s), `"`)
}

func TestC1594_006_cycle_eval_is_rigorous(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, ".evolve", "evals", evalSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: path})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", path, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", path, res.Overall)
	}
	caps := parseScoreCaps(t, path)
	if len(caps) < 5 {
		t.Fatalf("eval %s declares %d score_cap row(s), want >= 5 — one per behavioral criterion", path, len(caps))
	}
	for i, c := range caps {
		if c.criterion == "" || c.evidence == "" {
			t.Errorf("eval score_cap[%d] incomplete: criterion=%q evidence=%q", i, c.criterion, c.evidence)
			continue
		}
		if !strings.Contains(c.evidence, "-run") || !strings.Contains(c.evidence, "./internal/") {
			t.Errorf("eval score_cap[%d] evidence %q is not a -run-narrowed single-package command", i, c.evidence)
		}
	}
	if t.Failed() {
		return
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("sh", "-c", "cd "+root+" && "+caps[0].evidence)
	if code != 0 || err != nil {
		t.Errorf("eval grader %q exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			caps[0].evidence, code, err, stdout, stderr)
	}
}
