//go:build acs

package cycle784

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evalSlug  = "chronicle-s3-digest-wiring"
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	scoutPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/phases/scout"
	triagePkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
)

var coreTestNames = []string{
	"TestNewCycleRun_SeedsRecentOutcomesDigestAtShadow",
	"TestNewCycleRun_OffStageWritesNoDigest",
	"TestNewCycleRun_EnforceInjectsRecentOutcomesContext",
	"TestNewCycleRun_DigestFailureWarnsNotAborts",
}

func TestC784_001_digest_seeding_contract_green(t *testing.T) {
	pattern := "TestNewCycleRun_(SeedsRecentOutcomesDigestAtShadow|OffStageWritesNoDigest|EnforceInjectsRecentOutcomesContext|DigestFailureWarnsNotAborts)"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", pattern, corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, corePkg, code, err, stdout, stderr)
	}
	for _, name := range coreTestNames {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("core chronicle test %s did not report PASS (renamed, skipped, or not run)", name)
		}
	}
}

func TestC784_002_prompt_injection_contract_green(t *testing.T) {
	pattern := "ComposePrompt_(InjectsRecentOutcomes|NoDigestContextKeyIsByteIdentical)"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", pattern, scoutPkg, triagePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, code, err, stdout, stderr)
	}
	for _, name := range []string{
		"TestScoutComposePrompt_InjectsRecentOutcomes",
		"TestTriageComposePrompt_InjectsRecentOutcomes",
	} {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("prompt test %s did not report PASS (renamed, skipped, or not run)", name)
		}
	}
	if n := strings.Count(stdout, "--- PASS: TestComposePrompt_NoDigestContextKeyIsByteIdentical"); n < 2 {
		t.Errorf("byte-identical pin PASSed in %d package(s), want 2 (scout + triage)", n)
	}
}

func TestC784_003_eval_file_passes_quality_check(t *testing.T) {
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
