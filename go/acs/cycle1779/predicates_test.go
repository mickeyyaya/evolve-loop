//go:build acs

// Package cycle1779 encodes cycle 1779's acceptance: the cycle-run quota
// closeout parity, the cmd/evolve silent-policy and count-pin repairs, and the
// topngate TDD scope fail-open. Each behavioral predicate runs the named
// frozen tests of ONE package, narrowed with -run.
package cycle1779

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTest(t *testing.T, pkg, run string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", pkg, "-run", run)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestC1779_001_TDDScopeGateReconcilesWithoutTriageReportAndFailsLoud(t *testing.T) {
	out, err := goTest(t, "./internal/topngate", `^TestTDDScopeGate_(ReconcilesFrom|MatchingDeclaration|NoCommitmentRecord)|^TestReviewer_GateAppliesToTDDOnly`)
	if err != nil {
		t.Errorf("RED: topngate scope tests failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("the -run filter matched nothing:\n%s", out)
	}
}

func TestC1779_002_CycleRunRootPausesOnQuotaAndStillWalksOtherFailures(t *testing.T) {
	out, err := goTest(t, "./cmd/evolve", `^TestCycleRunRoot_`)
	if err != nil {
		t.Errorf("RED: cycle-run closeout parity tests failed: %v\n%s", err, out)
	}
}

func TestC1779_003_CmdEvolveReportsPolicyErrorsAbsolutizesRootAndCountsCalls(t *testing.T) {
	out, err := goTest(t, "./cmd/evolve", `^(TestConsensusDispatch_|TestCycleHealth_RelativeProjectRootIsAbsolutized|TestWireOrchestratorDeps_MalformedPolicyWarnsOnTheGivenConsole|TestCountCallExprs_|TestNilSignalCenterRootsPin_UsesTheASTCount)`)
	if err != nil {
		t.Errorf("RED: cmd/evolve silent-policy and count-pin tests failed: %v\n%s", err, out)
	}
}

// acs-predicate: config-check — the criterion is that stale prose is gone.
func TestC1779_004_NoTextSaysAnOutOfLaneBuildAborts(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stale := regexp.MustCompile(`(?is)outside\s+(//\s*)?triage.{0,160}abort`)
	for _, rel := range []string{
		"docs/operations/runtime-reference.md",
		"docs/architecture/packages/internal-config.md",
		"go/cmd/evolve/cmd_cycle.go",
	} {
		body, err := readFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if loc := stale.FindStringIndex(body); loc != nil {
			t.Errorf("RED: %s still says a build outside triage top_n aborts (advisory since 2026-07-22; only the TDD gate blocks): %q", rel, body[loc[0]:loc[1]])
		}
	}
}

func TestC1779_005_ReleaseBinaryGoEvolveIsNotDeleted(t *testing.T) {
	root := acsassert.RepoRoot(t)
	const rel = "go/evolve"
	if info, err := os.Stat(filepath.Join(root, rel)); err != nil || !info.Mode().IsRegular() {
		t.Errorf("RED: %s is missing on disk (stat err=%v); restore it with git checkout HEAD -- %s", rel, err, rel)
	}
	if _, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s is not in the index (deletion staged): code=%d err=%v", rel, code, err)
	}
	if _, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "cat-file", "-e", "HEAD:"+rel); code != 0 {
		t.Errorf("RED: %s is absent from HEAD's tree: code=%d err=%v", rel, code, err)
	}
	if out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--cached", "--name-status", "HEAD", "--", rel); code != 0 || strings.TrimSpace(out) != "" {
		t.Errorf("RED: the staged diff changes the release binary %s (out-of-scope churn): %q code=%d err=%v", rel, strings.TrimSpace(out), code, err)
	}
}

// acs-predicate: config-check — the criterion is that the explanation stops
// calling the release binary a stray build output.
func TestC1779_006_ExplanationDoesNotClaimGoEvolveIsStray(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docs, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1779-*.md"))
	if err != nil {
		t.Fatalf("glob explanation documents: %v", err)
	}
	if len(docs) == 0 {
		t.Fatalf("no docs/explain/builds/cycle-1779-*.md to check")
	}
	token := regexp.MustCompile(`(^|[^\w/.-])go/evolve([^\w/-]|$)`)
	claim := regexp.MustCompile(`(?i)stray|build output|delet|remov|untrack|leak`)
	bullet := regexp.MustCompile("^\\s*[-*]\\s+`go/evolve`")
	for _, doc := range docs {
		body, err := readFile(doc)
		if err != nil {
			t.Fatalf("read %s: %v", doc, err)
		}
		for i, line := range strings.Split(body, "\n") {
			if token.MatchString(line) && (claim.MatchString(line) || bullet.MatchString(line)) {
				t.Errorf("RED: %s:%d still lists go/evolve as a removed stray build output (ADR-0094: keep): %q", filepath.Base(doc), i+1, line)
			}
		}
	}
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}
