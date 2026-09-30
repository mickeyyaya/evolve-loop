//go:build acs

package cycle1343

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const mainPkg = "./cmd/evolve"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runNamed(t *testing.T, names ...string) (ok bool, missing []string, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", mainPkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", mainPkg, code, err, tail(out, 30))
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

func TestC1343_001_chain_boundary_refresh_fires_before_every_batch(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch",
		"TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch")
	if !ok {
		t.Errorf("chain-boundary refresh wiring/call-count regressed (missing PASS receipts: %v). This is the functional core of the auto-refresh-binary-at-boundary AC set — a chained `evolve loop` must re-check binary staleness at every boundary, never mid-batch.\n%s", missing, tail(out, 25))
	}
}

func TestC1343_002_wave_boundary_refresh_covers_non_chain_runs(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary",
		"TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers")
	if !ok {
		t.Errorf("wave/fleet-boundary refresh wiring regressed (missing PASS receipts: %v). A chain-less evolve loop run must stay covered by the same boundary refresh, checked once per wave and never mid-lane.\n%s", missing, tail(out, 25))
	}
}
