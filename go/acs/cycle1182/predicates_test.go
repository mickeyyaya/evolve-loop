//go:build acs

package cycle1182

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	wavePkg      = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	triagecapPkg = "github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1182_001_WidenSeamPrunesConsumedCommittedIDs(t *testing.T) {
	ok, out := runGoTest(t, wavePkg,
		"TestWidenNarrowDecision_DropsConsumedCommittedAtFleetWidth|TestWidenNarrowDecision_ConsumedIDDroppedEvenWithNoBacklogReplacement|TestWidenNarrowDecision_PrunesEveryIDALaneCannotTake")
	if !ok {
		t.Errorf("widenNarrowDecision still carries lifecycle-consumed committed ids forward "+
			"(or over-prunes ids it cannot resolve) — a consumed id will be re-pinned into the next wave's lane-scope.json:\n%s", out)
	}
}

func TestC1182_002_PruneConsumedIsExportedAndSingleSourced(t *testing.T) {
	ok, out := runGoTest(t, triagecapPkg,
		"TestPruneUndispatchable_KeepsExactlyTheIDsALaneMayTake|TestPruneUndispatchable_EmptyInputIsIdentity")
	if !ok {
		t.Errorf("triagecap.PruneUndispatchable is not callable from outside the package with the one "+
			"dispatchability rule's semantics — the widen seam cannot reuse the SSOT:\n%s", out)
	}
}

func TestC1182_003_WaveNPlusOneAndFloorsPassthroughIntact(t *testing.T) {
	ok, out := runGoTest(t, wavePkg,
		"TestWaveNPlusOneExcludesConsumedScope|TestWidenNarrowDecision_CommittedFloorsShortCircuit|TestWidenNarrowDecision_ExpandsCommittedLaneWithClusterMates")
	if !ok {
		t.Errorf("wave N+1 still inherits the consumed scope, or the prune disturbed the committed_floors "+
			"byte-identical passthrough / cluster-mate deepening:\n%s", out)
	}
}
