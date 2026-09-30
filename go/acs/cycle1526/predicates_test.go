//go:build acs

package cycle1526

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

const redContractFile = "go/internal/bridge/driver_tmux_repl_submitverify_test.go"

func assertSuiteTestsPass(t *testing.T, extraFlags []string, names ...string) {
	t.Helper()
	args := append([]string{"test", "-run", "^(" + strings.Join(names, "|") + ")$", "-count=1", "-v"}, extraFlags...)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", append(args, bridgePkg)...)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", bridgePkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("contract test %s did not pass in %s (missing, renamed, or failing). exit=%d\ngo test output:\n%s",
				name, bridgePkg, code, out)
		}
	}
}

func TestC1526_001_NudgeUnsubmittedResendsEnter(t *testing.T) {
	assertSuiteTestsPass(t, nil, "TestTmuxREPL_NudgeUnsubmitted_ResendsEnter")
}

func TestC1526_002_NudgeSubmittedNoResend(t *testing.T) {
	assertSuiteTestsPass(t, nil, "TestTmuxREPL_NudgeSubmitted_NoResend")
}

func TestC1526_003_ResendIsBounded(t *testing.T) {
	assertSuiteTestsPass(t, nil, "TestTmuxREPL_NudgeUnsubmitted_ResendBounded")
}

func TestC1526_004_PromptPasteSubmitVerified(t *testing.T) {
	assertSuiteTestsPass(t, nil, "TestTmuxREPL_PromptPasteUnsubmitted_ResendsEnter")
}

func TestC1526_005_ContractGreenUnderRace(t *testing.T) {
	assertSuiteTestsPass(t, []string{"-race"},
		"TestTmuxREPL_NudgeUnsubmitted_ResendsEnter",
		"TestTmuxREPL_NudgeSubmitted_NoResend",
		"TestTmuxREPL_NudgeUnsubmitted_ResendBounded",
		"TestTmuxREPL_PromptPasteUnsubmitted_ResendsEnter",
	)
}

func TestC1526_006_RedContractTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, redContractFile)) {
		t.Fatalf("RED contract %s missing on disk", redContractFile)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", redContractFile); code != 0 {
		t.Errorf("%s is untracked — it would be dropped at ship, taking every predicate above with it", redContractFile)
	}
}
