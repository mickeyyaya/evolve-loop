//go:build acs

// Package cycle1594 materializes the cycle-1594 acceptance criteria for the one
// task this fleet lane committed (scout-report.md ## Selected Tasks;
// triage-report.md ## top_n): `gitignore-staging-sweep`. Per R9.3 nothing here
// binds to the deferred `ship-addall-staging-surface` item — the broader ship
// staging manifest gets ZERO predicates this cycle.
//
// The defect. go/internal/core/phase_bindings.go:226 computes the cycle's
// content identity with `git add -A`, so any untracked file sitting in the lane
// — a bug-reproduction reproducer, a regenerated go/coverage.*.txt artifact, a
// minted phase stub, another agent's scratch — is adopted into BOTH the audit
// binding's WorktreeTreeSHA (phase_bindings.go:129) and the ADR-0048 Slice B
// verdict-cache key. The binding then attests a tree the auditor never
// reviewed. Reproduced in .evolve/runs/cycle-1594/bug-reproduction-report.md
// (foreign-residue.txt present in the emitted tree).
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 unrelated untracked residue absent from the binding tree   → C1594_001
//	AC2 declared (staged) new builder output retained              → C1594_002
//	AC3 NEGATIVE: unstaged TRACKED edits still captured            → C1594_003
//	AC4 EDGE: residue-only lane keeps its base identity            → C1594_004
//	AC5 WIRING: the production binding path emits the scoped tree   → C1594_005
//	AC6 the cycle eval is rigorous, not vacuous                    → C1594_006
//	AC7 existing core suite (incl. -tags integration) stays green  → manual+checklist
//	    (a package sweep is a banned flaky-predicate shape; the build floor,
//	     the ship gate and CI own it)
//
// Adversarial axes: negative (C1594_003 — the cheapest way to pass the two
// exclusion pins is to bind HEAD^{tree} or stage nothing, which silently drops
// real work and manufactures base-identical identities: the INTEGRITY_TREE_DRIFT
// class of cycle-152 and the fresh-base verdict-cache collision), edge (C1594_004
// — the EMPTY declared selection, where there is no legitimate change to shelter
// residue behind), semantic (exclusion, retention, tracked-capture, production
// reachability and eval rigor are five distinct behaviors).
//
// No source-grep predicates (the cycle-85 ban): C1594_001..005 each execute the
// real production code as a subprocess `go test` run against a real Git
// repository and require a NAMED `--- PASS:` marker (a bare exit 0 would hide a
// renamed, skipped, or deleted test); C1594_006 runs the SSOT eval-quality
// checker. Every invocation names ONE package and is -run-narrowed — never a
// `/...` sweep (the flaky-predicate-shape rule).
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

// runNamedTest runs exactly ONE named test in ONE package and requires its
// `--- PASS:` marker. Never a /... sweep, always -run-narrowed.
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

// AC1: an untracked file nobody declared must be ABSENT from the tree the audit
// binding and the verdict cache are keyed on. This is the reproduced defect.
func TestC1594_001_binding_tree_excludes_unrelated_untracked_residue(t *testing.T) {
	runNamedTest(t, corePkg, "TestWorktreeContentSHA_ExcludesUnrelatedUntrackedResidue")
}

// AC2: a NEW file the builder declared by staging it must be RETAINED — scoping
// the selection may not drop the builder's own output from the tree ship commits.
func TestC1594_002_binding_tree_retains_declared_new_file(t *testing.T) {
	runNamedTest(t, corePkg, "TestWorktreeContentSHA_StagesDeclaredNewFile")
}

// AC3 (NEGATIVE): an unstaged modification to a TRACKED file must still be
// captured. A fix that returns the base tree, or stages nothing, passes AC1 and
// AC4 and dies here — a base-identical binding is the INTEGRITY_TREE_DRIFT /
// fresh-base cache-collision failure the identity exists to prevent.
func TestC1594_003_binding_tree_captures_unstaged_tracked_modification(t *testing.T) {
	runNamedTest(t, corePkg, "TestWorktreeContentSHA_CapturesUnstagedTrackedModification")
}

// AC4 (EDGE): a lane whose only difference from its base is residue must keep
// its base identity exactly — the empty declared selection must not adopt.
func TestC1594_004_residue_only_worktree_keeps_base_identity(t *testing.T) {
	runNamedTest(t, corePkg, "TestWorktreeContentSHA_ResidueOnlyWorktreeKeepsBaseIdentity")
}

// AC5 (WIRING PROOF): the scoped selection must be reached from the PRODUCTION
// caller — emitPhaseBindings → recordAuditBinding → worktreeContentSHA, the path
// that writes WorktreeTreeSHA into the auditor ledger entry ship verifies. A
// correct helper with no production caller is dead code; the named test drives
// the real orchestrator method and reads the identity off the real ledger entry.
func TestC1594_005_production_audit_binding_path_emits_scoped_tree(t *testing.T) {
	runNamedTest(t, corePkg, "TestEmitPhaseBindings_AuditBindingTreeExcludesUnrelatedResidue")
}

// scoreCap is one row of the eval's YAML frontmatter score_cap block.
type scoreCap struct {
	criterion string
	evidence  string
}

// parseScoreCaps reads the emitted eval artifact's score_cap rows. The eval is a
// deliverable of THIS cycle, so its absence or malformation is a failure.
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

// AC6: the permanent regression entry for this task must be rigorous — an eval
// whose graders are vacuous or unrunnable PASSes trivially and caps nothing in
// later cycles. The SSOT checker rules on shape; this predicate additionally
// requires one row per behavioral criterion, each -run-narrowed at a single
// package, and then EXECUTES the eval's own first grader so a decorative
// evidence string cannot stand in for a runnable one.
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
	// Behavioral half: the eval's own grader must actually run green.
	stdout, stderr, code, err := acsassert.SubprocessOutput("sh", "-c", "cd "+root+" && "+caps[0].evidence)
	if code != 0 || err != nil {
		t.Errorf("eval grader %q exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			caps[0].evidence, code, err, stdout, stderr)
	}
}
