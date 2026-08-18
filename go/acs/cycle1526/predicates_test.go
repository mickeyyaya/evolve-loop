//go:build acs

// Package cycle1526 encodes the acceptance criteria for cycle-1526's single
// committed task, `submit-verify-retro-paste` (triage-decision.json top_n[0]).
//
// The task's root-cause evidence — three recorded panes (cycles 1505, 1510,
// 1517) in which the driver's one-shot nudge sat UNSUBMITTED at the `❯` input
// line while every interaction record read "result":"no_effect" — is fixed by a
// submit-verify step on the driver's send path: verify the input line cleared,
// re-send Enter when it did not, bounded and loud.
//
// Every predicate here is BEHAVIORAL: it runs the real driver through
// Engine.LaunchArgs (the production entry point) inside the default Go suite and
// asserts on the `--- PASS:` line, never on source text. A source grep would go
// green the moment someone typed the word "submit-verify" into a comment.
package cycle1526

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// bridgePkg is the module-qualified package under test. Module-qualified (not
// "./internal/bridge") because a `go test` binary runs with its own package
// directory as cwd, where the relative path does not resolve.
const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

// redContractFile is the frozen RED contract authored by the TDD phase
// (handoff: doNotModifyTests=true).
const redContractFile = "go/internal/bridge/driver_tmux_repl_submitverify_test.go"

// assertSuiteTestsPass shells `go test -run '^(names)$' -count=1 -v pkg` and
// requires EVERY name to print `--- PASS: <name>`.
//
// Asserting the PASS line rather than exit 0 is load-bearing: `go test -run` on
// a pattern that matches nothing exits 0 with "no tests to run", so a deleted or
// renamed contract test would otherwise false-GREEN. -count=1 defeats the cache.
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

// TestC1526_001_NudgeUnsubmittedResendsEnter — AC-1. The evidence-licensed case:
// the nudge is sent, the next capture still shows it parked at the `❯` input
// line, and the driver must re-send Enter and log the attempt loudly.
func TestC1526_001_NudgeUnsubmittedResendsEnter(t *testing.T) {
	assertSuiteTestsPass(t, nil, "TestTmuxREPL_NudgeUnsubmitted_ResendsEnter")
}

// TestC1526_002_NudgeSubmittedNoResend — AC-2, the anti-double-submit control.
// When the input line DID clear, no extra Enter may be sent: a spurious Enter
// re-submits whatever the agent typed next and desyncs the pane. This predicate
// passes on today's code (the driver never re-sends at all) and only becomes
// load-bearing once AC-1 lands — it is what stops AC-1 being satisfied by an
// unconditional second Enter.
func TestC1526_002_NudgeSubmittedNoResend(t *testing.T) {
	assertSuiteTestsPass(t, nil, "TestTmuxREPL_NudgeSubmitted_NoResend")
}

// TestC1526_003_ResendIsBounded — AC-3. An input line that never clears must not
// spin: re-sends are capped and the run still terminates on the normal
// artifact-timeout path.
func TestC1526_003_ResendIsBounded(t *testing.T) {
	assertSuiteTestsPass(t, nil, "TestTmuxREPL_NudgeUnsubmitted_ResendBounded")
}

// TestC1526_004_PromptPasteSubmitVerified — AC-4. The same verification covers
// the prompt-delivery site the cycle committed to (driver_tmux_repl.go:368-376),
// so the fix is one shared submit path rather than a nudge-only patch.
func TestC1526_004_PromptPasteSubmitVerified(t *testing.T) {
	assertSuiteTestsPass(t, nil, "TestTmuxREPL_PromptPasteUnsubmitted_ResendsEnter")
}

// TestC1526_005_ContractGreenUnderRace — AC-5. The whole contract re-run with
// -race: the submit-verify state is touched from the driver goroutine while the
// liveness/pane pollers run, so a data race here is a real defect, not noise.
// Scoped to the four contract tests in ONE named package (never a ./... sweep).
func TestC1526_005_ContractGreenUnderRace(t *testing.T) {
	assertSuiteTestsPass(t, []string{"-race"},
		"TestTmuxREPL_NudgeUnsubmitted_ResendsEnter",
		"TestTmuxREPL_NudgeSubmitted_NoResend",
		"TestTmuxREPL_NudgeUnsubmitted_ResendBounded",
		"TestTmuxREPL_PromptPasteUnsubmitted_ResendsEnter",
	)
}

// TestC1526_006_RedContractTracked — AC-6. The RED contract must be committed,
// not merely present on disk: an untracked (or gitignored) test file is silently
// dropped at ship and the whole gate evaporates (the cycle-92/93 lesson).
func TestC1526_006_RedContractTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, redContractFile)) {
		t.Fatalf("RED contract %s missing on disk", redContractFile)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", redContractFile); code != 0 {
		t.Errorf("%s is untracked — it would be dropped at ship, taking every predicate above with it", redContractFile)
	}
}
