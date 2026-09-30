//go:build acs

package cycle593

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const tokenusagePkg = "github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC593_001_TranscriptScanSumsUsageWithinWindow(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestTranscriptScan_SumsUsageWithinWindow")
	if !ok {
		t.Errorf("internal/tokenusage transcript-sum scan missing or failing:\n%s", out)
	}
}

func TestC593_002_TranscriptScanDedupsStreamedUsage(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestTranscriptScan_DeduplicatesStreamedUsageByMessageID")
	if !ok {
		t.Errorf("internal/tokenusage streamed-usage dedup missing or failing:\n%s", out)
	}
}

func TestC593_003_TranscriptScanContentVerifiesConcurrentSessions(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted")
	if !ok {
		t.Errorf("internal/tokenusage concurrent-session content verification missing or failing:\n%s", out)
	}
}

func TestC593_004_TranscriptScanMissingDirYieldsSourceNone(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestTranscriptScan_MissingDirYieldsSourceNone")
	if !ok {
		t.Errorf("internal/tokenusage missing-dir SourceNone handling missing or failing:\n%s", out)
	}
}
