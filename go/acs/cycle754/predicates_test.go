//go:build acs

package cycle754

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	tokenusagePkg = "github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	bridgePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

func TestC754_001_EventsTierWinsWhenNoTranscript(t *testing.T) {
	runGoTest(t, tokenusagePkg, "TestDefaultResolver_EventsLogTier_WinsWhenNoTranscript")
}

func TestC754_002_ScrollbackTierOutputOnlyFloor(t *testing.T) {
	runGoTest(t, tokenusagePkg, "TestDefaultResolver_ScrollbackTier_OutputOnlyFloor")
}

func TestC754_003_TranscriptFidelityStillWins(t *testing.T) {
	runGoTest(t, tokenusagePkg, "TestDefaultResolver_TranscriptTier_StillWinsOverLowerTiers")
}

func TestC754_004_AllTiersEmptySourceNone(t *testing.T) {
	runGoTest(t, tokenusagePkg, "TestDefaultResolver_AllTiersEmpty_SourceNoneNilError")
}

func TestC754_005_MalformedEventsLogFallsThrough(t *testing.T) {
	runGoTest(t, tokenusagePkg, "TestDefaultResolver_MalformedEventsLog_FallsThroughCleanly")
}

func TestC754_006_EngineRecordsEventsResultSource(t *testing.T) {
	runGoTest(t, bridgePkg, "TestRecordTokenUsage_EventsLogFallback_EndToEnd")
}

func TestC754_007_EngineNoSourcesRecordsNone(t *testing.T) {
	runGoTest(t, bridgePkg, "TestRecordTokenUsage_NoSources_RecordsSourceNoneZeroTokens")
}
