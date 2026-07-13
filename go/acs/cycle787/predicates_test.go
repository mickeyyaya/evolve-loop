//go:build acs

// Package cycle787 materializes the cycle-787 acceptance criteria for the
// fleet-scoped todo merge-rung0-trivial-rebase-fastpath, task
// composition-verdict-writer: ledger.WriteCompositionVerdict, the RUNG 0
// producer completing the read/verify/write triangle (cycle-786 landed the
// reader + kernel verifier; no producer existed anywhere in the tree).
//
// AC map (1:1 with .evolve/evals/composition-verdict-writer.md):
//
//	AC1 round-trip (write → read back → existing kernel verify accepts)
//	    → C787_001 runs TestWriteCompositionVerdict_RoundTrip. RED at
//	      authoring (WriteCompositionVerdict undefined: compile failure).
//	AC2 negative: forged patch_id / drifted diff / missing or failed
//	    required composed gate → error AND zero bytes appended
//	    → C787_002 runs TestWriteCompositionVerdict_RejectsPatchIDMismatch.
//	AC3 edge: empty AND whitespace-only diffs rejected, nothing persisted
//	    → C787_003 runs TestWriteCompositionVerdict_EmptyDiff.
//	AC4 `go build ./...` clean → C787_004 (whole-module build subprocess).
//	AC5 `go vet ./internal/adapters/ledger/...` clean → C787_005.
//	Step 6b: eval file passes the SSOT quality checker non-vacuously
//	    → C787_006.
//
// Adversarial axes: negative (C787_002 — four distinct rejection paths, each
// asserting NO partial write by byte-length), edge (C787_003 — nil, "", and
// two whitespace-only variants), semantic (round-trip acceptance, fail-closed
// rejection, and empty-input hygiene are three distinct behaviors). Every
// predicate executes the system under test via go test/build/vet subprocesses
// — no source-grep predicates (cycle-85 rule).
package cycle787

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	ledgerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	modulePkg = "github.com/mickeyyaya/evolve-loop/go/..."
	taskSlug  = "composition-verdict-writer"
)

// runGoTest runs the named tests of pkg (verbose, -race, fresh) and requires
// an explicit "--- PASS: <name>" for every wantPass — exit 0 alone never
// satisfies a predicate (rename/skip gaming).
func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-run", runExpr, "-v", pkg)
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

// AC1: an honestly-written composition-verdict line round-trips — read back
// with every field both consumers need, and the EXISTING kernel verifier
// (verifyCompositionLine via Verify) accepts it unchanged.
func TestC787_001_write_roundtrip_kernel_verifies(t *testing.T) {
	runGoTest(t, ledgerPkg, "^TestWriteCompositionVerdict_RoundTrip$",
		[]string{"TestWriteCompositionVerdict_RoundTrip"})
}

// AC2 (negatives): forged/mismatched patch_id and missing/failed required
// composed gates are rejected fail-closed — error returned AND the ledger's
// byte length unchanged (no partial write to roll back).
func TestC787_002_rejects_mismatch_and_bad_gates_without_partial_write(t *testing.T) {
	runGoTest(t, ledgerPkg, "^TestWriteCompositionVerdict_RejectsPatchIDMismatch$",
		[]string{"TestWriteCompositionVerdict_RejectsPatchIDMismatch"})
}

// AC3 (edge): empty and whitespace-only diffs (both artifacts) are rejected
// before anything is persisted.
func TestC787_003_empty_and_whitespace_diffs_rejected(t *testing.T) {
	runGoTest(t, ledgerPkg, "^TestWriteCompositionVerdict_EmptyDiff$",
		[]string{"TestWriteCompositionVerdict_EmptyDiff"})
}

// AC4: the whole module still builds (no unused imports / dead code).
func TestC787_004_module_builds(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			modulePkg, code, err, stdout, stderr)
	}
}

// AC5: go vet clean on the touched package.
func TestC787_005_vet_ledger_clean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", ledgerPkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			ledgerPkg, code, err, stdout, stderr)
	}
}

// Step 6b: the committed task's eval file exists and passes the SSOT quality
// checker with a NON-EMPTY command set (a missing/empty ```bash block would
// PASS vacuously — the existence-check gaming the eval gate forbids).
func TestC787_006_eval_file_passes_quality_check(t *testing.T) {
	root := acsassert.RepoRoot(t)
	evalPath := filepath.Join(root, ".evolve", "evals", taskSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", taskSlug, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Errorf("eval %s classified only %d command(s) — a vacuous eval is not a PASS", taskSlug, len(res.Commands))
	}
}
