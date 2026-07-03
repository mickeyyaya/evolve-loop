package budgethistory

// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise every exported budgethistory symbol by identifier (apicover counts
// field access as "uses", not "names"). Each test asserts a REAL contract.

import "testing"

// TestThroughput_CollectNamed names the Throughput type and Collect by
// identifier, pinning the core contract: a single 1h cycle yields SampleCount 1,
// a 1h median, and a per-lane pace of exactly 1.0 cycles/hour.
func TestThroughput_CollectNamed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRun(t, root, 1, oneHour(), 0)

	var tp Throughput = Collect(root, []int{1})
	if tp.SampleCount != 1 {
		t.Errorf("SampleCount=%d, want 1", tp.SampleCount)
	}
	if tp.MedianCycleDurationMS != 3_600_000 {
		t.Errorf("MedianCycleDurationMS=%d, want 3600000", tp.MedianCycleDurationMS)
	}
	if tp.CyclesPerHour != 1.0 {
		t.Errorf("CyclesPerHour=%v, want 1.0", tp.CyclesPerHour)
	}
}
