//go:build acs

package cycle1313

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evalSlug  = "triage-commit-time-protected-surface-admission"
	triagePkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
)

var rejectTestNames = []string{
	"TestTriageClassify_RejectsProtectedSurfaceTopNCard_BraceSyntax",
	"TestTriageClassify_RejectsProtectedSurfaceTopNCard_BareSyntax",
	"TestTriageClassify_RejectsAmongMultipleCards_NamesOffendingIdOnly",
}

var allowTestNames = []string{
	"TestTriageClassify_AllowsNonProtectedTopNCard",
	"TestTriageClassify_NoFilesSegmentIsUnaffected",
}

func TestC1313_001_reject_protected_surface_contract_green(t *testing.T) {
	pattern := "TestTriageClassify_(RejectsProtectedSurfaceTopNCard_BraceSyntax|RejectsProtectedSurfaceTopNCard_BareSyntax|RejectsAmongMultipleCards_NamesOffendingIdOnly)"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, triagePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, triagePkg, code, err, stdout, stderr)
	}
	for _, name := range rejectTestNames {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("admission-check test %s did not report PASS (renamed, skipped, or not run)", name)
		}
	}
}

func TestC1313_002_non_protected_cards_unaffected_contract_green(t *testing.T) {
	pattern := "TestTriageClassify_(AllowsNonProtectedTopNCard|NoFilesSegmentIsUnaffected)"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, triagePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, triagePkg, code, err, stdout, stderr)
	}
	for _, name := range allowTestNames {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("regression test %s did not report PASS (renamed, skipped, or not run)", name)
		}
	}
}

func TestC1313_003_triage_package_suite_green(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", triagePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			triagePkg, code, err, stdout, stderr)
	}
}

func TestC1313_004_eval_file_passes_quality_check(t *testing.T) {
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
		t.Fatalf("eval %s has %d classifiable command(s), want >=2 (vacuous-empty-eval guard)", evalPath, len(res.Commands))
	}
}
