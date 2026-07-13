//go:build acs

// Package cycle784 materializes the cycle-784 acceptance criteria for this
// fleet lane's sole committed inbox item, chronicle-s3-digest-wiring
// (triage-report.md ## top_n: seed-digest-at-cycle-start +
// inject-recent-outcomes-prompts; per R9.3 no predicates bind to any other
// lane's items).
//
// AC map (1:1, from scout-report.md Selected Tasks verifiableBy + Acceptance
// Criteria Summary):
//
//	AC1 seed-digest-at-cycle-start behavior (shadow seeds the artifact, off
//	    writes nothing, enforce injects Context["recent_outcomes"], digest
//	    failure WARNs and never aborts)
//	    → C784_001 runs the four TestNewCycleRun_* unit tests as a -race
//	      subprocess and requires each named "--- PASS:" marker (exit-0
//	      alone could hide a renamed/skipped test).
//	AC2 inject-recent-outcomes-prompts behavior (scout + triage render the
//	    injected digest; absent key ⇒ byte-identical prompts)
//	    → C784_002 same subprocess pattern over the scout + triage prompt
//	      tests, counting the byte-identical pin once per package.
//	AC3 permanent regression entry (.evolve/evals/chronicle-s3-digest-wiring.md)
//	    asserts real command output, not existence checks
//	    → C784_003 runs the SSOT checker (internal/evalqualitycheck — the
//	      exact code behind `evolve eval quality-check`) and requires
//	      Overall==PASS over a NON-EMPTY (≥2) command set, closing the
//	      vacuous-empty-eval hole.
//
// Adversarial axes: negative (off-stage write-nothing + byte-identical pins
// inside the C784_001/002 suites; C784_003 rejects the vacuous PASS), edge
// (digest-failure directory collision exercised by the unit suite), semantic
// (seeding vs injection vs prompt rendering vs eval rigor are distinct
// behaviors). No source-grep predicates (cycle-85 rule): every predicate
// executes the system under test as a subprocess or runs the SSOT checker.
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

// coreTestNames is the committed AC surface of seed-digest-at-cycle-start
// (go/internal/core/cyclerun_chronicle_test.go). Each must report an explicit
// verbose PASS — "4/4" is counted, never assumed from exit 0.
var coreTestNames = []string{
	"TestNewCycleRun_SeedsRecentOutcomesDigestAtShadow",
	"TestNewCycleRun_OffStageWritesNoDigest",
	"TestNewCycleRun_EnforceInjectsRecentOutcomesContext",
	"TestNewCycleRun_DigestFailureWarnsNotAborts",
}

// AC1: the digest-seeding unit contract is green under -race in THIS tree.
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

// AC2: the prompt-injection unit contract (scout + triage render + the
// byte-identical-when-absent pin in BOTH packages) is green under -race.
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
	// The byte-identical pin exists once per package — both must PASS.
	if n := strings.Count(stdout, "--- PASS: TestComposePrompt_NoDigestContextKeyIsByteIdentical"); n < 2 {
		t.Errorf("byte-identical pin PASSed in %d package(s), want 2 (scout + triage)", n)
	}
}

// AC3: the permanent eval entry passes the SSOT quality checker with a
// NON-EMPTY command set (an eval with no classifiable commands PASSes
// vacuously — that hole is closed here).
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
