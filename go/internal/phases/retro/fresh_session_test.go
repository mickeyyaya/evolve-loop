package retro

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type deadShellFirstFake struct {
	chainFake
	alive       bool
	emptyStdout bool
}

func (f *deadShellFirstFake) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if len(f.calls) == 0 && !f.alive {
		f.calls = append(f.calls, req)
		return core.BridgeResponse{ExitCode: 81, CauseCode: "dead_shell", SessionRecoverable: true}, errors.New("bridge: launch exit=81: artifact-timeout: cause=incomplete")
	}
	resp, err := f.chainFake.Launch(ctx, req)
	if f.emptyStdout {
		resp.Stdout = ""
	}
	return resp, err
}

func runRetro(t *testing.T, b core.Bridge, mutate func(*core.PhaseRequest)) core.PhaseResponse {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	profDir := filepath.Join(root, ".evolve", "profiles")
	for _, d := range []string{ws, profDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	prof, _ := json.Marshal(map[string]any{"name": "retrospective", "cli": "claude-tmux", "model_tier_default": "deep"})
	if err := os.WriteFile(filepath.Join(profDir, "retrospective.json"), prof, 0o644); err != nil {
		t.Fatal(err)
	}
	phase := New(Config{
		Router:  retroRouter(t, root, policy.Policy{}),
		Bridge:  b,
		Prompts: fakePromptsFS("# Retro body"),
		NowFn:   fixtures.FixedClock(time.Unix(1_700_000_000, 0), 90*time.Millisecond),
	})

	req := core.PhaseRequest{
		Cycle: 1687, ProjectRoot: root, Workspace: ws,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	}
	mutate(&req)
	resp, err := phase.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return resp
}

func TestRun_RetroSurvivesADeadShellFirstSession(t *testing.T) {
	b := &deadShellFirstFake{}

	resp := runRetro(t, b, func(*core.PhaseRequest) {})

	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Run = %q; want PASS from the fresh session (diagnostics %v)", resp.Verdict, resp.Diagnostics)
	}
	if len(b.calls) != 2 || b.calls[1].CLI != b.calls[0].CLI || !b.calls[1].FreshSession {
		t.Fatalf("calls = %d, want the dead session, then one fresh session of the same CLI", len(b.calls))
	}
}

func TestRun_AnEmptyStdoutReadsTheRetroReportFromItsArtifact(t *testing.T) {
	resp := runRetro(t, &deadShellFirstFake{alive: true, emptyStdout: true}, func(*core.PhaseRequest) {})

	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Run = %q (diagnostics %v), want PASS from the report on disk", resp.Verdict, resp.Diagnostics)
	}
}

func TestRun_AMissingExplanationReviewFailsTheRetroWithItsReason(t *testing.T) {
	resp := runRetro(t, &deadShellFirstFake{alive: true}, func(r *core.PhaseRequest) {
		r.ExplanationDocumentationVersion = 1
		r.BuildExplanationState = core.BuildExplanationInvalid
	})

	if resp.Verdict != core.VerdictFAIL || len(resp.Diagnostics) == 0 || !strings.Contains(resp.Diagnostics[len(resp.Diagnostics)-1].Message, "Explanation Documentation Review") {
		t.Fatalf("Run = %q (diagnostics %v), want FAIL that names the missing review", resp.Verdict, resp.Diagnostics)
	}
}

func TestRun_AFenceThatCannotSnapshotStillRunsTheRetro(t *testing.T) {
	b := &deadShellFirstFake{alive: true}

	resp := runRetro(t, b, func(r *core.PhaseRequest) {
		r.Worktree, r.Fence = filepath.Join(t.TempDir(), "absent"), &core.WorktreeFence{}
	})

	if len(b.calls) != 1 || resp.Verdict != core.VerdictPASS {
		t.Fatalf("calls = %d verdict = %q (diagnostics %v), want the retro to run once and pass", len(b.calls), resp.Verdict, resp.Diagnostics)
	}
}
