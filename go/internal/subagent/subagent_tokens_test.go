package subagent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// TestSubagentRun_RecordsUsage — S5, token-telemetry. Run() must copy the
// bridge response's token usage into Result.Tokens (covers the adapter-bypass
// path). Previously only CostUSD survived; per-run token counts were dropped.
func TestSubagentRun_RecordsUsage(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	want := core.TokenUsage{Input: 900, Output: 120, CacheRead: 40, CacheWrite: 8}
	bridge := &fakeBridge{response: core.BridgeResponse{ExitCode: 0, Tokens: want}}
	ledger := &fakeLedger{}
	loader := profiles.NewFromFS(fstest.MapFS{
		"builder.json": &fstest.MapFile{Data: []byte(`{
			"name":"builder","role":"builder","cli":"claude-p",
			"model_tier_default":"sonnet",
			"output_artifact":".evolve/runs/cycle-{cycle}/build-report.md"
		}`)},
	})
	r, err := New(Config{
		Profiles: loader, Bridge: bridge, Ledger: ledger,
		Now:  func() time.Time { return now },
		Rand: deterministicRand(0xAB),
		GitState: func(context.Context, string) (string, string, error) {
			return "", "", errors.New("not a git repo")
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	token := strings.Repeat("ab", ChallengeTokenBytes)
	tmp := t.TempDir()
	bridge.onLaunch = func(req core.BridgeRequest) error {
		writeArtifact(t, req.ArtifactPath, "<!-- challenge-token: "+token+" -->\nok\n", now)
		return nil
	}
	res, err := r.Run(context.Background(), Request{
		Agent: "builder", Cycle: 3, ProjectRoot: tmp,
		Workspace: filepath.Join(tmp, ".evolve/runs/cycle-3"), Prompt: "go",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Tokens != want {
		t.Fatalf("Result.Tokens = %+v, want %+v", res.Tokens, want)
	}
}
