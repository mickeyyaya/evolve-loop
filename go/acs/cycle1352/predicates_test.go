//go:build acs

package cycle1352

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

func TestC1352_001_boundary_refresh_stops_chain_before_next_batch_never_midbatch(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch",
		"TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch")
	if !ok {
		t.Errorf("chain-boundary staleness stop/never-mid-batch wiring regressed (missing PASS receipts: %v). This is the functional core of Task 1 (chain-boundary-binary-refresh-stop) — a chained `evolve loop` must detect binary staleness and stop before the NEXT batch, never mid-batch.\n%s", missing, tail(out, 25))
	}
}

func TestC1352_002_refresh_event_surfaces_in_chain_summary_json(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent",
		"TestChainResult_MarshalOmitsBoundaryRefreshWhenNil",
		"TestRunLoopChain_SetsBoundaryRefreshOnReExecStop")
	if !ok {
		t.Errorf("chain-summary refresh-event surfacing regressed (missing PASS receipts: %v). This is Task 2 (chain-summary-refresh-event-field) — chainResult must carry a populated refresh-event field on a boundary-refresh stop and omit it on every ordinary stop.\n%s", missing, tail(out, 25))
	}
}

func TestC1352_003_no_superseded_stop_only_design_reintroduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	chainSrc := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_chain.go")
	for _, needle := range []string{"chain_binary_stale", "StaleAtBoundary", "StaleBinaryCommit"} {
		if !acsassert.FileNotContains(t, chainSrc, needle) {
			t.Errorf("found %q in cmd_loop_chain.go — a second, superseded staleness-stop code path has been (re-)introduced alongside the shipped chain_boundary_refresh_reexec design; centralize on the one shipped path instead of duplicating it", needle)
		}
	}
}
