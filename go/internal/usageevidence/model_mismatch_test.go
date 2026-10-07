package usageevidence

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func launchAModelMismatch(t *testing.T, screen string) (map[string]clihealth.Entry, []Line) {
	t.Helper()
	root, ws := t.TempDir(), t.TempDir()
	pane, queried := usageFixture(t, screen), 0
	explain := New(Options{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve"), Act: true, Probe: func(context.Context, string) (string, error) {
		queried++
		return pane, nil
	}})
	inner := &fixtures.FakeBridge{Resp: core.BridgeResponse{ExitCode: bridge.ExitModelMismatch}}

	res, _ := Wrap(inner, explain, nil).Launch(context.Background(), core.BridgeRequest{CLI: "agy-claude-tmux", Agent: "auditor", ProjectRoot: root, Workspace: ws})

	if res.ExitCode != bridge.ExitModelMismatch || queried != 1 {
		t.Fatalf("exit %d, usage queries %d: a model mismatch keeps its exit and runs one usage query", res.ExitCode, queried)
	}
	return clihealth.NewStore(root, nil).Active(), readRecords(t, ws)
}

func TestBridge_AModelMismatchOnADrainedClaudeGroupBenchesAgyClaude(t *testing.T) {
	active, records := launchAModelMismatch(t, "agy_usage_claude_drained.txt")

	if _, benched := active["agy-claude"]; !benched {
		t.Fatalf("benches %v: agy boots its Gemini default when the Claude group is exhausted, so the usage verdict benches agy-claude", active)
	}
	if _, gemini := active["agy"]; gemini {
		t.Errorf("agy's Gemini family was benched for the Claude group's exhaustion")
	}
	if len(records) != 1 || records[0].ExitCode != bridge.ExitModelMismatch {
		t.Errorf("records %+v: the mismatch is recorded as usage evidence", records)
	}
}

func TestBridge_AModelMismatchWithHealthyQuotaBenchesNothing(t *testing.T) {
	active, _ := launchAModelMismatch(t, "agy_usage_groups.txt")

	if len(active) != 0 {
		t.Fatalf("benches %v: with quota ruled out, the mismatch itself benches nothing; the walk alone moves on", active)
	}
}
