//go:build acs

package cycle1236

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "./internal/bridge/"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", bridgePkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", bridgePkg, code, err, tail(out, 30))
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			missing = append(missing, n)
		}
	}
	return code == 0 && len(missing) == 0, missing, out
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func report(t *testing.T, what string, missing []string, out string) {
	t.Helper()
	if len(missing) > 0 {
		t.Errorf("RED: %s — and these contract tests did not report PASS (deleted or skipped): %v\n%s",
			what, missing, tail(out, 40))
		return
	}
	t.Errorf("RED: %s\n%s", what, tail(out, 40))
}

func TestC1236_001_StdoutContractGetsBenignTeardownGrace(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout",
		"TestTmuxREPL_StdoutContract_CancelWhileStreaming_StillTimesOut",
	)
	if !ok {
		report(t, "the stdout completion contract is still starved on the wait loop's final "+
			"post-cancel poll — CapturePane cannot fork tmux on a dead ctx, so a finished "+
			"router/advisor turn is laundered into ExitArtifactTimeout (or the parity fix "+
			"manufactured completion for a turn that never settled)", missing, out)
	}
}

func TestC1236_002_GitEvidenceContractGetsBenignTeardownGrace(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout",
		"TestTmuxREPL_GitContract_CancelWithoutEvidenceCommit_StillTimesOut",
	)
	if !ok {
		report(t, "the git-evidence completion contract is still starved on the final post-cancel "+
			"poll — deps.Runner cannot fork git on a dead ctx, so a verified evidence commit is "+
			"reported as no-completion (or the fix completes a phase that committed nothing)", missing, out)
	}
}

func TestC1236_003_ArtifactContractNotRegressedByTheFix(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout",
		"TestTmuxREPL_CancelWithoutDeliverable_StillTimesOut",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
	)
	if !ok {
		report(t, "fixing the parity gap regressed the contract that already worked: the artifact "+
			"benign-teardown short-circuit no longer fires (a live final ctx disarmed the "+
			"ctx.Err() key without an explicit finality signal replacing it)", missing, out)
	}
}

func TestC1236_004_ArtifactCrossPollDebounceStillHolds(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
	)
	if !ok {
		report(t, "the already-landed artifact cross-poll stability window no longer holds — this "+
			"cycle's edits to the shared completion.go disturbed the sibling contract "+
			"(first-sight completion is back, the stability key went size-only, or the window "+
			"is no longer reached from the production wait loop)", missing, out)
	}
}
