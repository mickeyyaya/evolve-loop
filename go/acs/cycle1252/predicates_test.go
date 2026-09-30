//go:build acs

package cycle1252

import (
	"os"
	"path/filepath"
	"regexp"
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

func TestC1252_001_CrossPollStabilityWindowGatesReady(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
	)
	if !ok {
		report(t, "artifactDetector no longer requires a cross-poll (size, mtime) stability "+
			"window before ready=true — a still-growing or mid-Edit deliverable can complete "+
			"the phase on first sight, which is the cycle-1198 truncated-read defect this "+
			"cycle exists to close", missing, out)
	}
}

func TestC1252_002_RelocationIsGatedByTheWindow(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing",
		"TestArtifactDetector_RelocationHappensOnceFallbackSettles",
		"TestArtifactDetector_RelocatedCompleteFallbackStillCompletes",
		"TestArtifactDetector_RelocationNoteSurvivesUntilStable",
	)
	if !ok {
		report(t, "a non-canonical fallback artifact is relocated before the stability window "+
			"closes. artifactReady (the destructive mover) must be reachable ONLY from "+
			"artifactDetector.complete(); relocating on first sight snapshots a partial file "+
			"into the canonical path and deletes the source still being written",
			missing, out)
	}
}

func TestC1252_003_WindowReachableFromProductionWaitLoop(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
		"TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout",
		"TestTmuxREPL_StdoutContract_CancelWhileStreaming_StillTimesOut",
		"TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout",
		"TestTmuxREPL_GitContract_CancelWithoutEvidenceCommit_StillTimesOut",
	)
	if !ok {
		report(t, "the artifact stability window is not reached from BOTH production poll sites "+
			"(driver_tmux_repl.go:613 main loop, :586 post-cancel final poll), or the final "+
			"poll regressed: a delivered session must complete at the buzzer instead of "+
			"exiting ExitArtifactTimeout, and an undelivered one must still time out",
			missing, out)
	}
}

func TestC1252_004_StableTicksIsAMeaningfulWindow(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactStableTicks_IsAMeaningfulWindow",
	)
	if !ok {
		report(t, "artifactStableTicks is no longer a real window (>= 2 consecutive unchanged "+
			"observations). At 1 the detector is byte-equivalent to the legacy first-sight "+
			"path and the debounce is decorative", missing, out)
	}
}

var staleFinalityRef = regexp.MustCompile(`Checked AFTER\s*(?://\s*)?artifactReady`)

var correctFinalityRef = regexp.MustCompile(`Checked AFTER\s*(?://\s*)?artifactLocate`)

func TestC1252_005_FinalityShortCircuitDocumentsTheRightGuard(t *testing.T) {
	path := filepath.Join(goDir(t), "internal", "bridge", "completion.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	if staleFinalityRef.Match(src) {
		t.Errorf("RED: completion.go's finality short-circuit still claims it is checked after "+
			"artifactReady. poll() checks finality after artifactLocate (:226); artifactReady "+
			"is reached only via complete() (:276). Correct the reference — a comment asserting "+
			"a guarantee through a function the code does not call there is the same stale "+
			"forward-reference defect as deliverable.go:179-181, which this cycle closes (%s)", path)
	}
	if !correctFinalityRef.Match(src) {
		t.Errorf("RED: completion.go's finality short-circuit does not state which guard "+
			"actually precedes it. The sentence must name artifactLocate, whose found result is "+
			"what proves a non-empty artifact exists before finality short-circuits. Deleting "+
			"the explanation is not a fix: the ordering invariant (locate during the window, "+
			"relocate only on close) is load-bearing and must stay documented at the seam (%s)", path)
	}
}
