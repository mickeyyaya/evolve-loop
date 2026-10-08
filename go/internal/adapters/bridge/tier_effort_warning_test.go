package bridge

import (
	"bytes"
	"strings"
	"testing"
)

func TestReportTierEffortWarnings_WritesOneLinePerIgnoredEntry(t *testing.T) {
	var stderr bytes.Buffer
	reportTierEffortWarnings(&stderr, []string{"bridge.tier_effort.deep: unknown effort \"hihg\", keeping \"high\""})
	want := "[bridge] WARN policy bridge.tier_effort.deep: unknown effort \"hihg\", keeping \"high\"\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	stderr.Reset()
	reportTierEffortWarnings(&stderr, nil)
	if strings.TrimSpace(stderr.String()) != "" {
		t.Fatalf("no warnings wrote %q", stderr.String())
	}
}
