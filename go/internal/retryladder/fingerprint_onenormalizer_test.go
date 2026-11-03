package retryladder_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurefp"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func TestFingerprint_HashesTheOneSharedNormalizer(t *testing.T) {
	t.Parallel()

	for _, reason := range []string{
		"phase scout: artifact-timeout phase=scout cycle=2071 driver=claude-tmux",
		"the STE lint: 2 finding(s) (first: docs/explain/builds/cycle-2051-01m5xwqdjqyc9mhva9bxdvs52e.md:8)",
		"coverage floor failed in cycle 1946 after 0.31s on attempt 2/3",
	} {
		sum := sha256.Sum256([]byte(failurefp.Normalize(reason)))
		want := "text:" + hex.EncodeToString(sum[:])[:12]
		if got := retryladder.Fingerprint(reason); got != want {
			t.Errorf("Fingerprint(%q) = %q, want the hash of failurefp.Normalize %q", reason, got, want)
		}
	}
}

func TestFingerprint_MeasuredCycleAssignPairShareOneFingerprint(t *testing.T) {
	t.Parallel()

	a := retryladder.Fingerprint("artifact-timeout phase=scout cycle=2071 driver=claude-tmux")
	b := retryladder.Fingerprint("artifact-timeout phase=scout cycle=2072 driver=claude-tmux")
	if a != b {
		t.Errorf("cycle=2071 and cycle=2072 got %q and %q, want one fingerprint", a, b)
	}
}
