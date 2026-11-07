package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func readCLIFrame(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("panestream", "testdata", rel))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return string(b)
}

func TestChannelE2E_RealFixtures_ClaudeSpan(t *testing.T) {
	t.Parallel()
	runClaudeSpanE2E(t, hostLag{})
}
