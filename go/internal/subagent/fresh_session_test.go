package subagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type deadShellFirstBridge struct {
	fakeBridge
	t   *testing.T
	now time.Time
}

func (b *deadShellFirstBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.calls = append(b.calls, req)
	if len(b.calls) == 1 {
		return core.BridgeResponse{ExitCode: 81, CauseCode: "dead_shell", SessionRecoverable: true}, errors.New("bridge: launch exit=81")
	}
	writeArtifact(b.t, req.ArtifactPath, "<!-- challenge-token: "+strings.Repeat("ab", ChallengeTokenBytes)+" -->\nbuild ok\n", b.now)
	return core.BridgeResponse{CostUSD: 0.05}, nil
}

func TestRun_ASubagentSurvivesADeadShellFirstSession(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	b := &deadShellFirstBridge{t: t, now: now}
	r := newRunner(t, b, &fakeLedger{}, now)

	res, err := r.Run(context.Background(), Request{
		Agent: "builder", Cycle: 3, ProjectRoot: tmp,
		Workspace: filepath.Join(tmp, ".evolve/runs/cycle-3"), Prompt: "do the build",
	})

	if err != nil || res.Verdict != VerdictPASS {
		t.Fatalf("Run = %q, %v; want PASS from the fresh session (diags %v)", res.Verdict, err, res.Diagnostics)
	}
	if len(b.calls) != 2 || !b.calls[1].FreshSession || b.calls[1].CLI != b.calls[0].CLI {
		t.Fatalf("calls = %d, want one fresh session of the same CLI", len(b.calls))
	}
}

func TestExecAdapter_SurvivesADeadShellFirstSession(t *testing.T) {
	b := &deadShellFirstBridge{t: t, now: time.Now()}
	prev := newAdapterBridge
	newAdapterBridge = func(gobridge.Deps) core.Bridge { return b }
	t.Cleanup(func() { newAdapterBridge = prev })
	dir := t.TempDir()
	artifact := filepath.Join(dir, "report.md")
	promptFile := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("the real prompt"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, err := execAdapter(context.Background(), map[string]string{"RESOLVED_CLI": "claude-tmux", "ARTIFACT_PATH": artifact, "PROMPT_FILE": promptFile}, nil)

	if err != nil || code != 0 {
		t.Fatalf("execAdapter = %d, %v; want the fresh session's exit 0", code, err)
	}
	if len(b.calls) != 2 || !b.calls[1].FreshSession || b.calls[1].Prompt != "the real prompt" {
		t.Fatalf("calls = %+v, want one fresh session with the prompt file's text", b.calls)
	}
}

func TestNewAdapterBridge_IsTheBridgeEngine(t *testing.T) {
	if _, ok := newAdapterBridge(gobridge.Deps{}).(*gobridge.Engine); !ok {
		t.Fatal("newAdapterBridge must build the bridge engine")
	}
}
