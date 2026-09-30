//go:build acs

package cycle1254

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

func TestC1254_001_ArtifactReadyCrossPollDebounceHolds(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
		"TestArtifactDetector_RelocationNoteSurvivesUntilStable",
		"TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing",
		"TestArtifactDetector_RelocationHappensOnceFallbackSettles",
		"TestArtifactDetector_RelocatedCompleteFallbackStillCompletes",
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
		"TestArtifactStableTicks_IsAMeaningfulWindow",
	)
	if !ok {
		report(t, "the artifact-ready cross-poll debounce no longer holds — an artifact is being "+
			"declared finished from a single observation (or the stability window stopped gating "+
			"relocation), which re-opens the mid-write truncated-read the deliverable.go:180 doc "+
			"claims is closed at the source", missing, out)
	}
}

func TestC1254_002_CompletionContractCancelParityHolds(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout",
		"TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout",
		"TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout",
	)
	if !ok {
		report(t, "the benign-teardown grace is not contract-agnostic — a DELIVERED phase torn down "+
			"at the finish line is being laundered into ExitArtifactTimeout on at least one "+
			"completion contract (the final post-cancel poll cannot fork tmux/git on a dead ctx, "+
			"or the finality marker artifactDetector keys on was disarmed)", missing, out)
	}
}

func TestC1254_003_CancelGraceAndDebounceRefuseFalseCompletion(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestTmuxREPL_StdoutContract_CancelWhileStreaming_StillTimesOut",
		"TestTmuxREPL_GitContract_CancelWithoutEvidenceCommit_StillTimesOut",
		"TestTmuxREPL_CancelWithoutDeliverable_StillTimesOut",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
	)
	if !ok {
		report(t, "a completion contract now MANUFACTURES completion — an unfinished phase "+
			"(streaming pane / no verifying commit / no deliverable) or an artifact still being "+
			"written is being certified as done, which is a false-PASS generator strictly worse "+
			"than the timeout the grace was added to prevent", missing, out)
	}
}
