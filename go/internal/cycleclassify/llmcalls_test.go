package cycleclassify

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

func TestDriverExitedZero_LatestUnavailableUsageAttemptStillOwnsExit(t *testing.T) {
	workspace := t.TempDir()
	success, failure := 0, 81
	duration := int64(100)
	for _, rec := range []llmcalls.Record{
		{
			SchemaVersion: llmcalls.SchemaVersion, CallID: "first", Phase: "audit", CLI: "claude-tmux",
			UsageStatus: llmcalls.UsageMeasured, DurationMS: &duration, ExitCode: &success,
		},
		{
			SchemaVersion: llmcalls.SchemaVersion, CallID: "last", Phase: "audit", CLI: "codex-tmux",
			UsageStatus: llmcalls.UsageUnavailable, DurationMS: &duration, ExitCode: &failure,
		},
	} {
		if err := llmcalls.AppendWorkspace(workspace, rec); err != nil {
			t.Fatal(err)
		}
	}
	if driverExitedZero(workspace, "audit") {
		t.Fatal("latest failed attempt inherited an earlier exit-zero veto")
	}
}
