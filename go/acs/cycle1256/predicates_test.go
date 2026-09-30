//go:build acs

package cycle1256

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

func TestC1256_001_CrossPollStabilityWindowHolds(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
		"TestArtifactStableTicks_IsAMeaningfulWindow",
		"TestArtifactDetector_Poll",
	)
	if !ok {
		report(t, "the cross-poll (size, mtime) stability window in artifactDetector.poll no "+
			"longer holds — a deliverable was declared finished on a single non-empty "+
			"observation, or a file that changes on every tick was allowed to settle "+
			"(scout AC-1/AC-4, cycle-1198/1233 regression)", missing, out)
	}
}

func TestC1256_002_DebounceReachableFromProductionWaitLoop(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
	)
	if !ok {
		report(t, "the artifact stability window is no longer reached from the production "+
			"wait loop (driver_tmux_repl.go) — a churning deliverable completed the "+
			"phase, i.e. the debounce is dead code from the caller's point of view",
			missing, out)
	}
}

func TestC1256_003_RelocationStaysGatedByTheWindow(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing",
		"TestArtifactDetector_RelocationHappensOnceFallbackSettles",
		"TestArtifactDetector_RelocatedCompleteFallbackStillCompletes",
		"TestArtifactDetector_RelocationNoteSurvivesUntilStable",
	)
	if !ok {
		report(t, "a non-canonical fallback artifact is relocated before the stability "+
			"window closes (artifactDetector.poll → complete → artifactReady). On "+
			"relocateFile's copy+remove branch that truncates the deliverable "+
			"permanently — the window must gate the MOVE, not follow it (cycle-1249)",
			missing, out)
	}
}

// acs-predicate: config-check — WAIVER RATIONALE. This AC is a documentation
func TestC1256_004_DeliverableDocDescribesTheRealMechanism(t *testing.T) {
	root := acsassert.RepoRoot(t)
	deliverable := filepath.Join(root, "go", "internal", "deliverable", "deliverable.go")
	completion := filepath.Join(root, "go", "internal", "bridge", "completion.go")

	concepts := []struct {
		what     string
		variants []string
	}{
		{"the mtime half of the stability key (a size-only window is blind to an equal-length fix-up Edit)",
			[]string{"mtime", "modtime", "modification time"}},
		{"that stability is measured across CONSECUTIVE poll ticks, not within one poll",
			[]string{"consecutive", "successive", "across polls", "cross-poll tick"}},
		{"the named window constant, so the doc points at the real code",
			[]string{"artifactStableTicks"}},
	}
	for _, c := range concepts {
		if !acsassert.FileContainsAny(deliverable, c.variants...) {
			t.Errorf("RED: go/internal/deliverable/deliverable.go still only NAMES the "+
				"cross-poll debounce without describing %s. The comment is byte-identical "+
				"to main's, where the mechanism did not exist — a reader cannot tell a "+
				"true claim from the false one scout found (Finding 1). Expected one of "+
				"%v in the LAYERING comment", c.what, c.variants)
		}
	}

	if !acsassert.LineContainsAll(deliverable, "size", "mtime") {
		t.Errorf("RED: go/internal/deliverable/deliverable.go never states the stability " +
			"KEY. The debounce compares (size, mtime) together — a size-only window is " +
			"blind to an equal-length fix-up Edit, which is why mtime is in the key at " +
			"all (completion.go). Name both on one line")
	}

	if !acsassert.FileContains(t, completion, "artifactStableTicks") {
		t.Errorf("RED: deliverable.go is required to cite artifactStableTicks, but " +
			"go/internal/bridge/completion.go does not define it — the doc would be " +
			"describing a mechanism that does not exist, which is the cycle-1256 defect " +
			"class inverted rather than fixed")
	}
}
