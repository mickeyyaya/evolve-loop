package fleet

import (
	"strings"
	"testing"
)

func TestQuotaAwareCount_NonPositiveMinLanesIsRaisedToOne(t *testing.T) {
	t.Parallel()
	var warn strings.Builder

	got := QuotaAwareCount(3, map[string]string{"codex": "rate_limit"}, 0, &warn)

	if got != 2 {
		t.Fatalf("QuotaAwareCount(3, one bench, min 0) = %d, want 2 (one lane shed, floor of one)", got)
	}
	if !strings.Contains(warn.String(), "wave count 3 -> 2 (min 1)") {
		t.Errorf("warning = %q, want the shrink message with min 1", warn.String())
	}
}
