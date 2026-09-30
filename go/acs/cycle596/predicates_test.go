//go:build acs

package cycle596

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	tokenusagePkg = "github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	cyclecostPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
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

func TestC596_001_FidelityOrderFirstNonEmptyWins(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg,
		"TestCollectorChain_FidelityOrderFirstNonEmptyWins|TestChain_RealAdaptersPreferHigherFidelity")
	if !ok {
		t.Errorf("collector chain does not return the first non-empty tier in fidelity order (transcript>eventsResult>scrollbackPeak):\n%s", out)
	}
}

func TestC596_002_AllEmptyYieldsNone(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestCollectorChain_AllEmptyYieldsNone")
	if !ok {
		t.Errorf("all-empty chain does not yield SourceNone (degenerate 'first tier always wins' impl):\n%s", out)
	}
}

func TestC596_003_EventsResultReusesCyclecostExtraction(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg,
		"TestEventsResultCollector_ExtractsResultEnvelopeTokens|TestEventsResultCollector_NoResultEnvelopeIsEmpty")
	if !ok {
		t.Errorf("eventsResult tier does not reuse the cyclecost result-envelope extraction (duplicated/forked parser or wrong tokens):\n%s", out)
	}
}

func TestC596_004_ScrollbackPeakOutputOnlyFloor(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg,
		"TestScrollbackPeakCollector_OutputOnlyFloorFromPane|TestScrollbackPeakCollector_NoTokensIsEmpty")
	if !ok {
		t.Errorf("scrollbackPeak tier is not an output-only floor over ExtractResponseTokens:\n%s", out)
	}
}

func TestC596_005_CyclecostRegressionGreen(t *testing.T) {
	ok, out := runGoTest(t, cyclecostPkg, ".*")
	if !ok {
		t.Errorf("cyclecost suite regressed after the shared-extraction refactor:\n%s", out)
	}
}
