//go:build acs

package cycle941

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	ledgerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	modulePkg = "github.com/mickeyyaya/evolve-loop/go/..."

	coreTaskSlug = "merge-rung2-scoped-review-core"
	wireTaskSlug = "merge-rung2-wire-ship-recovery"
)

func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runExpr, "-v", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS (renamed, skipped, or not run)\nstdout:\n%s", name, stdout)
		}
	}
}

func TestC941_001_only_intersecting_hunks_dispatched(t *testing.T) {
	runGoTest(t, corePkg, "^TestScopedReview_SeesOnlyIntersectingHunks$",
		[]string{"TestScopedReview_SeesOnlyIntersectingHunks"})
}

func TestC941_002_compatible_composes_entangled_escalates(t *testing.T) {
	runGoTest(t, corePkg, "^TestScopedReview_CompatibleComposesEntangledEscalates$",
		[]string{"TestScopedReview_CompatibleComposesEntangledEscalates"})
}

func TestC941_003_empty_intersection_no_dispatch(t *testing.T) {
	runGoTest(t, corePkg, "^TestScopedReview_EmptyIntersectionNoDispatch$",
		[]string{"TestScopedReview_EmptyIntersectionNoDispatch"})
}

func TestC941_004_malformed_diff_fails_closed(t *testing.T) {
	runGoTest(t, corePkg, "^TestScopedReview_MalformedDiffFailsClosed$",
		[]string{"TestScopedReview_MalformedDiffFailsClosed"})
}

func TestC941_005_ledger_method_field(t *testing.T) {
	runGoTest(t, ledgerPkg,
		"^TestWriteCompositionVerdict_Method(ScopedReview|DefaultsTrivialRebase)$",
		[]string{
			"TestWriteCompositionVerdict_MethodScopedReview",
			"TestWriteCompositionVerdict_MethodDefaultsTrivialRebase",
		})
}

func TestC941_006_orchestrator_scoped_review_wired(t *testing.T) {
	runGoTest(t, corePkg, "^TestOrchestrator_ScopedMergeReviewWired$",
		[]string{"TestOrchestrator_ScopedMergeReviewWired"})
}

func TestC941_007_llm_resolution_reenters_rung0(t *testing.T) {
	runGoTest(t, corePkg, "^TestLLMResolution_ReentersRung0Verification$",
		[]string{"TestLLMResolution_ReentersRung0Verification"})
}

func TestC941_008_module_builds(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			modulePkg, code, err, stdout, stderr)
	}
}

func TestC941_009_vet_touched_packages_clean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", corePkg, ledgerPkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			corePkg, ledgerPkg, code, err, stdout, stderr)
	}
}

func TestC941_010_eval_files_pass_quality_check(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, slug := range []string{coreTaskSlug, wireTaskSlug} {
		evalPath := filepath.Join(root, ".evolve", "evals", slug+".md")
		res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
		if err != nil {
			t.Fatalf("eval quality-check %s: %v", evalPath, err)
		}
		if res.Overall != evalqualitycheck.LevelPass {
			for _, c := range res.Commands {
				if c.Level != evalqualitycheck.LevelPass {
					t.Errorf("eval %s command %q classified level %d: %s", slug, c.Line, c.Level, c.Reason)
				}
			}
			t.Fatalf("eval %s overall level %d, want PASS(0)", slug, res.Overall)
		}
		if len(res.Commands) < 2 {
			t.Errorf("eval %s classified only %d command(s) — a vacuous eval is not a PASS", slug, len(res.Commands))
		}
	}
}
