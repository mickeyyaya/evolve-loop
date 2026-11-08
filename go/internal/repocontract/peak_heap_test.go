package repocontract

import "testing"

const (
	peakHeapLimitBytes       = 400 << 20
	onePackageHeapLimitBytes = 200 << 20
)

func TestRepoContract_PeakHeapUnder400MB(t *testing.T) {
	scan := sharedTreeScan(t)
	if scan.HeapSamples < 3 || scan.PeakHeapBytes == 0 {
		t.Fatalf("the scan must sample the live heap while units are held: samples=%d peak=%d", scan.HeapSamples, scan.PeakHeapBytes)
	}
	t.Logf("peak heap %d MB over %d samples", scan.PeakHeapBytes>>20, scan.HeapSamples)
	if scan.PeakHeapBytes >= peakHeapLimitBytes {
		t.Errorf("peak heap %d MB is not below %d MB", scan.PeakHeapBytes>>20, peakHeapLimitBytes>>20)
	}
	if scan.PeakHeapBytes >= onePackageHeapLimitBytes {
		t.Errorf("peak heap %d MB is not below %d MB: holding every package of ASTs at once peaks near 350 MB, one package at a time near 70 MB", scan.PeakHeapBytes>>20, onePackageHeapLimitBytes>>20)
	}
}

func TestRepoContract_ScanSamplesTheHeapWhileAUnitIsLive(t *testing.T) {
	dir, rels := writeTree(t, map[string]string{"a/a_test.go": "package a\n"})
	scan, err := scanTestTree(dir, rels)
	if err != nil {
		t.Fatal(err)
	}
	if scan.HeapSamples < 1 || scan.PeakHeapBytes == 0 {
		t.Errorf("a one-directory scan must still sample once: samples=%d peak=%d", scan.HeapSamples, scan.PeakHeapBytes)
	}
}
