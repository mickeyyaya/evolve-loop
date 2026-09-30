//go:build acs

package cycle1233

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

func TestC1233_001_ArtifactReadyRequiresCrossPollStability(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
		"TestArtifactStableTicks_IsAMeaningfulWindow",
	)
	if !ok {
		report(t, "artifactDetector.poll still completes on the first non-empty read (or its "+
			"stability key is size-only) — the cycle-1198 mid-Write→Edit truncated read is still accepted",
			missing, out)
	}
}

func TestC1233_002_CtxCancelShortCircuitsTheWindow(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
	)
	if !ok {
		report(t, "a cancelled context does not short-circuit the stability window (or it "+
			"short-circuits into a false completion with no artifact on disk)", missing, out)
	}
}

func TestC1233_003_RelocationDiagnosticSurvivesUnstableTick(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_RelocationNoteSurvivesUntilStable",
	)
	if !ok {
		report(t, "the single-shot relocation diagnostic is swallowed by the unstable tick that "+
			"observed it (or relocation bypasses the stability window entirely)", missing, out)
	}
}

func TestC1233_004_DebounceReachedFromWaitLoopAndFixturesHold(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
		"TestRunTmuxREPL_ExtendNoEscalationReport",
	)
	if !ok {
		report(t, "the stability window is not reached from the production wait loop, or the "+
			"extra tick underruns the ArtifactTimeoutS=2 fixture (the rollback cause)", missing, out)
	}
}
