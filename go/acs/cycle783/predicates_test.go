//go:build acs

package cycle783

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	taskID      = "token-telemetry-input-cache-fidelity"
	evalSlug    = "verify-and-close-token-cache-fidelity"
	cycle779Pkg = "github.com/mickeyyaya/evolve-loop/go/acs/cycle779"
)

var cycle779PredicateNames = []string{
	"TestC779_001_scanner_extracts_input_and_cache",
	"TestC779_002_scanner_absent_cache_fields_not_fabricated",
	"TestC779_003_per_driver_coverage_warns_not_zeros",
	"TestC779_004_unknown_driver_fails_open_no_error",
	"TestC779_005_engine_passes_driver_to_resolver",
	"TestC779_006_tokens_report_coverage_line_present",
	"TestC779_007_tokens_report_zero_window_not_covered",
	"TestC779_008_tokenusage_package_race_clean",
}

func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

func TestC783_001_cycle779_suite_reverified_8of8(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-tags", "acs", "-v", cycle779Pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race -tags acs %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			cycle779Pkg, code, err, stdout, stderr)
	}
	for _, name := range cycle779PredicateNames {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("cycle-779 predicate %s did not report PASS (renamed, skipped, or not run)", name)
		}
	}
}

func TestC783_002_closure_recorded_in_build_report(t *testing.T) {
	report := filepath.Join(stateRoot(t), ".evolve", "runs", "cycle-783", "build-report.md")
	if !acsassert.FileExists(t, report) {
		return
	}
	if !acsassert.LineContainsAll(report, taskID, "completed") {
		t.Errorf("build-report has no line marking %s as completed", taskID)
	}
	if !acsassert.LineContainsAll(report, taskID, "8/8") {
		t.Errorf("build-report closure for %s does not cite the 8/8 predicate evidence", taskID)
	}
}

func TestC783_003_no_live_inbox_reproposal(t *testing.T) {
	live, err := filepath.Glob(filepath.Join(stateRoot(t), ".evolve", "inbox", "*.json"))
	if err != nil {
		t.Fatalf("glob inbox: %v", err)
	}
	for _, f := range live {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("read live inbox item %s: %v", f, err)
			continue
		}
		if strings.Contains(string(data), `"id": "`+taskID+`"`) {
			t.Errorf("live inbox item %s still proposes %s — closure did not stick", f, taskID)
		}
	}
}

func TestC783_004_eval_file_passes_quality_check(t *testing.T) {
	evalPath := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", evalSlug+".md")
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
		t.Fatalf("eval %s overall level %d, want PASS(0)", evalPath, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Fatalf("eval %s classified only %d command(s) — a vacuous/empty eval is not a PASS", evalPath, len(res.Commands))
	}
}
