//go:build acs

package cycle2529

import (
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type suiteRun struct {
	once   sync.Once
	stdout string
	code   int
}

var contract suiteRun

func contractSuite(t *testing.T) (string, int) {
	t.Helper()
	contract.once.Do(func() {
		moduleDir := acsassert.GoModuleDir(t)
		contract.stdout, _, contract.code, _ = acsassert.SubprocessOutput(
			"go", "-C", moduleDir, "test", "-count=1", "-v", "-run", "^TestGitContract", "./internal/bridge")
	})
	return contract.stdout, contract.code
}

func requirePass(t *testing.T, path string) {
	t.Helper()
	out, _ := contractSuite(t)
	if !strings.Contains(out, "--- PASS: "+path+" (") {
		t.Errorf("%s did not pass:\n%s", path, acsassert.OutputTail(out, 40))
	}
}

func requireCaseOnBothSides(t *testing.T, name string) {
	t.Helper()
	requirePass(t, "TestGitContract/fake/"+name)
	requirePass(t, "TestGitContract/real/"+name)
}

func TestC2529_001_ContractTableRunsOnTheFakeAndOnRealGit(t *testing.T) {
	out, code := contractSuite(t)
	if code != 0 {
		t.Fatalf("go test -run ^TestGitContract exited %d:\n%s", code, acsassert.OutputTail(out, 60))
	}
	requirePass(t, "TestGitContract/fake")
	requirePass(t, "TestGitContract/real")
	if strings.Contains(out, "--- SKIP") {
		t.Errorf("a contract half skipped; git is always present, so both halves must run:\n%s", acsassert.OutputTail(out, 40))
	}
}

func TestC2529_002_FailedGitIsAnExitCodeWithANilErrorOnTheFakeAndOnRealGit(t *testing.T) {
	for _, name := range []string{
		"rev_parse_verify_unknown_ref_exits_one",
		"rev_list_unknown_base_exits_128",
		"log_unknown_rev_exits_128",
		"rev_parse_head_without_commits_exits_128",
	} {
		requireCaseOnBothSides(t, name)
	}
}

func TestC2529_003_SuccessfulGitStillReturnsItsStdoutOnTheFakeAndOnRealGit(t *testing.T) {
	for _, name := range []string{
		"rev_parse_head_prints_one_sha",
		"rev_list_one_commit_ahead_prints_one_sha",
		"rev_list_empty_range_prints_nothing",
		"log_head_prints_the_message",
	} {
		requireCaseOnBothSides(t, name)
	}
}

func TestC2529_004_AFakeThatFailsThroughErrFailsTheContractOnEveryNonZeroCase(t *testing.T) {
	for _, name := range []string{
		"rev_parse_verify_unknown_ref_exits_one",
		"rev_list_unknown_base_exits_128",
		"log_unknown_rev_exits_128",
		"rev_parse_head_without_commits_exits_128",
		"rev_parse_head_prints_one_sha",
	} {
		requirePass(t, "TestGitContract_ErrStyleFakeFailsEveryNonZeroCase/"+name)
	}
}

func TestC2529_005_GitCmdStillTurnsANonZeroExitIntoAnError(t *testing.T) {
	moduleDir := acsassert.GoModuleDir(t)
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "-C", moduleDir, "test", "-count=1", "-run",
		"^(TestExitingGit_|TestGitEvidence_GitCmdContract_)", "./internal/bridge")
	if err != nil || code != 0 {
		t.Fatalf("existing gitCmd exit-code tests exited %d (err=%v):\n%s", code, err, acsassert.OutputTail(out, 40))
	}
}
