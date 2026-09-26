package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type quotaExhaustedBridge struct {
	attempts []string // "cli@model", in the order the runner attempted them
}

func (q *quotaExhaustedBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	q.attempts = append(q.attempts, req.CLI+"@"+req.Model)
	return core.BridgeResponse{ExitCode: 85, Stderr: "provider quota exhausted"},
		errors.New("bridge: launch exit=85")
}

func (q *quotaExhaustedBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// writeQuotaExhaustionProfile takes the tier explicitly: writeFallbackProfile's "sonnet" is the floor and cannot step down.
func writeQuotaExhaustionProfile(t *testing.T, agentName, primaryCLI, modelTier string, fallback []string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fb := ""
	if len(fallback) > 0 {
		fb = `, "cli_fallback": ["` + strings.Join(fallback, `","`) + `"]`
	}
	body := `{"name":"` + agentName + `","cli":"` + primaryCLI + `","model_tier_default":"` + modelTier + `"` + fb + `}`
	profileBase := strings.TrimPrefix(agentName, "evolve-")
	if err := os.WriteFile(filepath.Join(dir, profileBase+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

// The name records the defect this reproduced; the test asserts a quota-exhausted tier steps down and re-walks the chain.
func TestRun_QuotaExhaustedAcrossChain_NeverStepsDownTier(t *testing.T) {
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "opus", prompt: "x"}
	qb := &quotaExhaustedBridge{}
	root := writeQuotaExhaustionProfile(t, "evolve-auditor", "codex-tmux", "opus", []string{"claude-tmux"})
	r := New(Options{
		Hooks:   hooks,
		Bridge:  qb,
		Prompts: fakePromptsFS("evolve-auditor", "x"),
	})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("expected the exhausted chain to surface an error, got nil")
	}

	want := []string{"codex-tmux@opus", "claude-tmux@opus", "codex-tmux@balanced", "claude-tmux@balanced"}
	if len(qb.attempts) != len(want) {
		t.Fatalf("BUG (wire-tier-fallback-chain still inert at runner.go): expected the CLI chain to "+
			"re-walk at the stepped-down tier once exhausted at the resolved tier (%d attempts: %v), "+
			"got %d attempt(s): %v — the runner still dispatches via the CLI-only llmroute.Dispatch "+
			"with a static plan.Model, ignoring plan.Tiers/llmroute.DispatchTiered added this cycle "+
			"(build-report.md Discovery #1: \"production dispatch ... still walks Dispatch (CLI-only)\")",
			len(want), want, len(qb.attempts), qb.attempts)
	}
	for i := range want {
		if qb.attempts[i] != want[i] {
			t.Errorf("attempt[%d] = %q, want %q (full=%v)", i, qb.attempts[i], want[i], qb.attempts)
		}
	}
}
