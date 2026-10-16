package budgethistory

import "testing"

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
