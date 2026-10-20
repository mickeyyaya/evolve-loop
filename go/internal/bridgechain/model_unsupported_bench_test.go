package bridgechain_test

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
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

var unsupportedNow = func() time.Time { return time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC) }

func benchUnsupported(t *testing.T, cli, tier string) (map[string]clihealth.Entry, []string) {
	t.Helper()
	root, ws := t.TempDir(), t.TempDir()
	b, _ := json.Marshal(map[string]any{"captured_at": unsupportedNow(), "cli": cli, "pattern_name": "model_unsupported",
		"pane_tail": "The model gpt-5.6-sol is not supported for this account"})
	if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	var logs []string
	bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: root, Workspace: ws, CLI: cli, Tier: tier, DispatchStart: unsupportedNow().Add(-time.Minute), Env: map[string]string{}},
		unsupportedNow, func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })
	return clihealth.NewStore(root, unsupportedNow).Active(), logs
}

func TestBenchOnEscalation_AModelUnsupportedExitBenchesOnlyThatModelOfTheFamily(t *testing.T) {
	t.Parallel()
	active, logs := benchUnsupported(t, "codex-tmux", "deep")

	entry, ok := active["codex@gpt-5.6-sol"]
	if !ok || len(active) != 1 {
		t.Fatalf("benches = %v, want only codex@gpt-5.6-sol: the family keeps its other models", active)
	}
	if !strings.Contains(strings.Join(logs, ""), "re-pin") {
		t.Errorf("the bench line must name the operator action: %q", logs)
	}
	if entry.Reason != clihealth.ModelUnsupportedPattern {
		t.Errorf("reason = %q", entry.Reason)
	}
}

func TestBenchOnEscalation_AModelUnsupportedExitWithNoTierModelBenchesNothing(t *testing.T) {
	t.Parallel()
	active, logs := benchUnsupported(t, "codex-tmux", "")

	if len(active) != 0 {
		t.Fatalf("benches = %v: without the model of the tier, a model bench would bench the whole family", active)
	}
	if !strings.Contains(strings.Join(logs, ""), "names no model") {
		t.Errorf("the skip must be logged: %q", logs)
	}
}

func TestApplyCLIHealthBench_AModelBenchClearsWhenTheTierResolvesToAnotherModel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := clihealth.NewStore(root, unsupportedNow)
	if _, err := store.BenchScopedWall("codex", "gpt-5.6-sol", clihealth.ModelUnsupportedPattern, "not supported"); err != nil {
		t.Fatal(err)
	}
	plan := func(tier string) llmroute.Plan {
		return llmroute.Plan{Candidates: []string{"codex-tmux", "claude-tmux"}, Tiers: []string{tier}}
	}

	deep := bridgechain.ApplyCLIHealthBench(root, "audit", plan("deep"), map[string]string{}, unsupportedNow, nil)
	balanced := bridgechain.ApplyCLIHealthBench(root, "audit", plan("balanced"), map[string]string{}, unsupportedNow, nil)

	if deep.Candidates[0] != "claude-tmux" {
		t.Errorf("deep chain = %v: codex deep resolves to the benched gpt-5.6-sol, so it goes last", deep.Candidates)
	}
	if balanced.Candidates[0] != "codex-tmux" {
		t.Errorf("balanced chain = %v: codex balanced resolves to another model, so the bench does not apply", balanced.Candidates)
	}
}

func TestWalking_AnUnknownPromptAtEveryCLIDoesNotStepDownTheTier(t *testing.T) {
	t.Parallel()
	inner := &causedExit{cause: "unknown_prompt"}
	var logs []string
	w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux", "claude-tmux"}, []string{"deep", "balanced"}),
		bridgechain.WithLog(func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }))

	_, _ = w.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep", Agent: "scout"})

	if got := strings.Join(attempts(inner.calls), " "); got != "codex-tmux@deep claude-tmux@deep" {
		t.Fatalf("attempts = %s: an undelivered prompt is no quota wall, so the tier holds", got)
	}
	if strings.Contains(strings.Join(logs, ""), "tier step-down") {
		t.Errorf("logs = %q, want no step-down", logs)
	}
}

type causedExit struct {
	cause string
	calls []core.BridgeRequest
}

func (c *causedExit) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	c.calls = append(c.calls, req)
	return core.BridgeResponse{ExitCode: 85, CauseCode: c.cause}, fmt.Errorf("bridge: exit 85")
}

func (c *causedExit) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{Version: "caused"}, nil
}
