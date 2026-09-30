//go:build acs

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

func TestC787_001_write_roundtrip_kernel_verifies(t *testing.T) {
	runGoTest(t, ledgerPkg, "^TestWriteCompositionVerdict_RoundTrip$",
		[]string{"TestWriteCompositionVerdict_RoundTrip"})
}

func TestC787_002_rejects_mismatch_and_bad_gates_without_partial_write(t *testing.T) {
	runGoTest(t, ledgerPkg, "^TestWriteCompositionVerdict_RejectsPatchIDMismatch$",
		[]string{"TestWriteCompositionVerdict_RejectsPatchIDMismatch"})
}

func TestC787_003_empty_and_whitespace_diffs_rejected(t *testing.T) {
	runGoTest(t, ledgerPkg, "^TestWriteCompositionVerdict_EmptyDiff$",
		[]string{"TestWriteCompositionVerdict_EmptyDiff"})
}

func TestC787_004_module_builds(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			modulePkg, code, err, stdout, stderr)
	}
}

func TestC787_005_vet_ledger_clean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", ledgerPkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			ledgerPkg, code, err, stdout, stderr)
	}
}

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
