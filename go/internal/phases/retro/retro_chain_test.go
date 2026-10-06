package retro

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type chainFake struct {
	calls []core.BridgeRequest
}

func (f *chainFake) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.calls = append(f.calls, req)
	if req.CLI == "codex-tmux" {
		return core.BridgeResponse{ExitCode: 81}, fmt.Errorf("bridge: launch exit=81: artifact-timeout: cause=review_pause")
	}
	_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
	body := "# Retrospective\n\n## Root Cause\nThe ship gate ran in the lane env.\n\n## Lessons\nScrub it.\n"
	_ = os.WriteFile(req.ArtifactPath, []byte(body), 0o644)
	_ = os.WriteFile(filepath.Join(req.Workspace, "failure-lesson.yaml"), []byte("id: gate-env\nlesson: scrub\n"), 0o644)
	return core.BridgeResponse{ExitCode: 0, Stdout: body}, nil
}

func (f *chainFake) Probe(context.Context) (core.BridgeProbe, error) { return core.BridgeProbe{}, nil }

func TestRun_TimeoutOnThePrimaryCLIFallsBackThroughTheChain(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	profDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prof, _ := json.Marshal(map[string]any{"name": "retrospective", "cli": "codex-tmux", "cli_fallback": []string{"claude-tmux"}, "model_tier_default": "deep"})
	if err := os.WriteFile(filepath.Join(profDir, "retrospective.json"), prof, 0o644); err != nil {
		t.Fatal(err)
	}
	inner := &chainFake{}
	var logs []string
	router := retroRouter(t, root, policy.Policy{})
	walked := bridgechain.New(inner,
		bridgechain.DefaultPlanResolver(router),
		bridgechain.WithLog(func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }))
	phase := New(Config{
		Router:  router,
		Bridge:  walked,
		Prompts: fakePromptsFS("# Retro body"),
		NowFn:   fixtures.FixedClock(time.Unix(1_700_000_000, 0), 90*time.Millisecond),
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1677, ProjectRoot: root, Workspace: ws,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict=%q, want PASS after the fallback (diagnostics %v)", resp.Verdict, resp.Diagnostics)
	}
	var clis []string
	for _, c := range inner.calls {
		clis = append(clis, c.CLI)
		if !c.ChainAttempt {
			t.Errorf("attempt not marked: %+v", c.CLI)
		}
	}
	if strings.Join(clis, " ") != "codex-tmux claude-tmux" {
		t.Fatalf("attempts = %v", clis)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "agent=retrospective fallback 2: trying cli=claude-tmux") {
		t.Fatalf("fallback must be one readable line: %q", logs)
	}
}
