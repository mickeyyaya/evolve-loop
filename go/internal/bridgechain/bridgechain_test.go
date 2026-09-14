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
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// scripted answers each (cli@model) with a scripted exit code; anything not
// scripted answers 0. A non-zero exit comes with an error, as the real bridge's
// does — DispatchTiered advances only on err != nil AND a trigger exit.
type scripted struct {
	exits map[string]int
	calls []core.BridgeRequest
}

func (s *scripted) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	s.calls = append(s.calls, req)
	code, ok := s.exits[req.CLI+"@"+req.Model]
	if !ok {
		code = s.exits[req.CLI]
	}
	if code != 0 {
		return core.BridgeResponse{ExitCode: code}, fmt.Errorf("bridge: exit %d", code)
	}
	return core.BridgeResponse{ExitCode: 0}, nil
}

func (s *scripted) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{Version: "scripted"}, nil
}

func attempts(calls []core.BridgeRequest) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.CLI + "@" + c.Model
	}
	return out
}

func fixedPlan(clis, tiers []string) bridgechain.PlanResolver {
	return func(core.BridgeRequest) llmroute.Plan {
		return llmroute.Plan{Candidates: clis, Triggers: llmroute.DefaultTriggers(), Tiers: tiers}
	}
}

// TestWalking_ArtifactTimeoutAdvancesToTheNextCLI reproduces lanes 1676/1677
// (2026-09-14): the retro launched the bridge itself, codex exited 81 after the
// artifact window, and the cycle sealed with no disposition. Through the walk
// the next CLI in the chain runs, every attempt is marked, and the fallback is
// one log line — the runner's own contract, now the bridge handle's.
func TestWalking_ArtifactTimeoutAdvancesToTheNextCLI(t *testing.T) {
	inner := &scripted{exits: map[string]int{"codex-tmux": 81}}
	var logs []string
	w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux", "claude-tmux"}, []string{"deep"}),
		bridgechain.WithLog(func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }))
	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep", Agent: "retrospective"})
	if err != nil || resp.ExitCode != 0 {
		t.Fatalf("the walk must end on the CLI that answered: resp=%+v err=%v", resp, err)
	}
	if got := attempts(inner.calls); strings.Join(got, " ") != "codex-tmux@deep claude-tmux@deep" {
		t.Fatalf("attempts = %v", got)
	}
	for _, c := range inner.calls {
		if !c.ChainAttempt {
			t.Fatalf("every attempt the walk hands the bridge carries ChainAttempt: %+v", c)
		}
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "agent=retrospective fallback 2: trying cli=claude-tmux tier=deep (previous=codex-tmux@deep=81 exit=81)") {
		t.Fatalf("fallback log = %q", logs)
	}
}

// TestWalking_ChainAttemptPassesThrough pins the no-double-walk rule: the
// runner (and the advisor) walk their own chains and mark each attempt; the
// decorator hands a marked request straight to the inner bridge without
// resolving a plan.
func TestWalking_ChainAttemptPassesThrough(t *testing.T) {
	inner := &scripted{exits: map[string]int{"codex-tmux": 81}}
	resolved := 0
	w := bridgechain.New(inner, func(core.BridgeRequest) llmroute.Plan {
		resolved++
		return llmroute.Plan{Candidates: []string{"codex-tmux", "claude-tmux"}, Triggers: llmroute.DefaultTriggers()}
	})
	_, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep", ChainAttempt: true})
	if err == nil || len(inner.calls) != 1 || resolved != 0 {
		t.Fatalf("a marked attempt passes through untouched: calls=%d resolved=%d err=%v", len(inner.calls), resolved, err)
	}
}

// TestWalking_RealFailureStopsTheWalk — a model's legitimate FAIL (a non-trigger
// exit) never routes to another CLI: the classifier must see it.
func TestWalking_RealFailureStopsTheWalk(t *testing.T) {
	inner := &scripted{exits: map[string]int{"codex-tmux": 1}}
	w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux", "claude-tmux"}, nil))
	_, _ = w.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep"})
	if got := attempts(inner.calls); len(got) != 1 {
		t.Fatalf("a non-trigger exit stops the walk: %v", got)
	}
}

// TestWalking_QuotaAtEveryCLIStepsDownATierAndBenchesEachWall — when every CLI
// at a tier exits 85 the walk steps down a tier (WS-876), and each 85 reaches
// the bench recorder with the CLI that hit the wall.
func TestWalking_QuotaAtEveryCLIStepsDownATierAndBenchesEachWall(t *testing.T) {
	inner := &scripted{exits: map[string]int{"codex-tmux@deep": 85, "claude-tmux@deep": 85}}
	var benched, logs []string
	w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux", "claude-tmux"}, []string{"deep", "balanced"}),
		bridgechain.WithBench(func(_, _ string, cli string, _ time.Time, _ map[string]string) { benched = append(benched, cli) }),
		bridgechain.WithLog(func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }),
		bridgechain.WithClock(func() time.Time { return time.Unix(0, 0) }))
	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep", Agent: "failure-advisor"})
	if err != nil || resp.ExitCode != 0 {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if got := attempts(inner.calls); strings.Join(got, " ") != "codex-tmux@deep claude-tmux@deep codex-tmux@balanced" {
		t.Fatalf("attempts = %v", got)
	}
	if strings.Join(benched, ",") != "codex-tmux,claude-tmux" {
		t.Fatalf("bench recorder saw %v", benched)
	}
	if len(logs) != 3 || !strings.Contains(logs[1], "tier step-down deep -> balanced") {
		t.Fatalf("logs = %q", logs)
	}
}

// TestWalking_NoPlanLaunchesTheCallersOwnChoiceOnce — a resolver with nothing
// to say degrades to exactly the launch the caller asked for.
func TestWalking_NoPlanLaunchesTheCallersOwnChoiceOnce(t *testing.T) {
	inner := &scripted{exits: map[string]int{"claude-p": 81}}
	w := bridgechain.New(inner, func(core.BridgeRequest) llmroute.Plan { return llmroute.Plan{} })
	_, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "claude-p", Model: "sonnet"})
	if err == nil || strings.Join(attempts(inner.calls), " ") != "claude-p@sonnet" {
		t.Fatalf("attempts=%v err=%v", attempts(inner.calls), err)
	}
	if p, err := w.Probe(context.Background()); err != nil || p.Version != "scripted" {
		t.Fatalf("Probe delegates to the inner bridge: %+v %v", p, err)
	}
}

func writeProfile(t *testing.T, dir, name string, v map[string]any) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(dir, name+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func findAll(string) (string, error) { return "/usr/bin/true", nil }

// TestDefaultPlanResolver_ProfileChainRunsBehindTheCallersPrimary — the plan
// for a launch is the agent profile's chain (llmroute.Resolve: primary,
// cli_fallback, tier chain, triggers) with the caller's own CLI kept as the
// primary when it differs, so wrapping never changes which CLI runs FIRST —
// only what happens when it fails.
func TestDefaultPlanResolver_ProfileChainRunsBehindTheCallersPrimary(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	writeProfile(t, dir, "retrospective", map[string]any{"name": "retrospective", "cli": "codex-tmux", "cli_fallback": []string{"claude-tmux"}, "model_tier_default": "deep"})
	resolve := bridgechain.DefaultPlanResolver(dir, nil, findAll, time.Now, func(string, ...any) {})
	plan := resolve(core.BridgeRequest{Agent: "retrospective", CLI: "codex-tmux", Model: "deep", ProjectRoot: root, Env: map[string]string{}})
	if strings.Join(plan.Candidates, " ") != "codex-tmux claude-tmux" || len(plan.Tiers) == 0 || plan.Tiers[0] != "deep" || !plan.TriggersFallback(81) {
		t.Fatalf("plan = %+v", plan)
	}
	plan = resolve(core.BridgeRequest{Agent: "retrospective", CLI: "claude-p", Model: "deep", ProjectRoot: root, Env: map[string]string{}})
	if strings.Join(plan.Candidates, " ") != "claude-p codex-tmux claude-tmux" {
		t.Fatalf("the caller's primary stays first: %v", plan.Candidates)
	}
	plan = resolve(core.BridgeRequest{Agent: "no-such-agent", CLI: "claude-p", Model: "sonnet", ProjectRoot: root, Env: map[string]string{}})
	if len(plan.Candidates) == 0 || plan.Candidates[0] != "claude-p" {
		t.Fatalf("no profile: the caller's choice leads: %v", plan.Candidates)
	}
}

// TestCLIHealthEnabled names the projection the runner and the decorator share.
func TestCLIHealthEnabled(t *testing.T) {
	if !bridgechain.CLIHealthEnabled(map[string]string{}) || bridgechain.CLIHealthEnabled(map[string]string{"EVOLVE_CLI_HEALTH": "0"}) {
		t.Fatal("EVOLVE_CLI_HEALTH defaults on and 0 disables")
	}
}

// TestApplyCLIHealthBench_DemotesABenchedFamily — an active family bench moves
// that family behind the healthy candidates (the runner's contract, shared).
func TestApplyCLIHealthBench_DemotesABenchedFamily(t *testing.T) {
	root := t.TempDir()
	now := func() time.Time { return time.Date(2026, 9, 14, 21, 0, 0, 0, time.UTC) }
	if _, err := clihealth.NewStore(root, now).BenchWall("codex", "rate_limit", "You've hit your usage limit"); err != nil {
		t.Fatal(err)
	}
	var logs []string
	plan := llmroute.Plan{Candidates: []string{"codex-tmux", "claude-tmux"}}
	out := bridgechain.ApplyCLIHealthBench(root, "retrospective", plan, map[string]string{}, now, func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })
	if strings.Join(out.Candidates, " ") != "claude-tmux codex-tmux" || len(logs) != 1 {
		t.Fatalf("out=%v logs=%q", out.Candidates, logs)
	}
	if same := bridgechain.ApplyCLIHealthBench(root, "retrospective", plan, map[string]string{"EVOLVE_CLI_HEALTH": "0"}, now, nil); strings.Join(same.Candidates, " ") != "codex-tmux claude-tmux" {
		t.Fatalf("disabled: %v", same.Candidates)
	}
}

// TestBenchOnEscalation_BenchesTheFamilyAFreshReportNames — the runner's
// staleness guard, shared: the report must name the CLI that just exited 85 and
// be captured at/after this dispatch's start; a leftover report never benches.
func TestBenchOnEscalation_BenchesTheFamilyAFreshReportNames(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	now := func() time.Time { return time.Date(2026, 9, 14, 21, 0, 0, 0, time.UTC) }
	report := func(captured time.Time) {
		b, _ := json.Marshal(map[string]any{"captured_at": captured, "cli": "codex-tmux", "pattern_name": "rate_limit", "pane_tail": "You've hit your usage limit"})
		if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	report(now().Add(-time.Hour)) // stale: captured before this dispatch started
	bridgechain.BenchOnEscalation(root, ws, "codex-tmux", now().Add(-time.Minute), map[string]string{}, now, nil)
	if active := clihealth.NewStore(root, now).Active(); len(active) != 0 {
		t.Fatalf("a stale report must not bench: %v", active)
	}
	report(now())
	var logs []string
	bridgechain.BenchOnEscalation(root, ws, "codex-tmux", now().Add(-time.Minute), map[string]string{}, now, func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })
	active := clihealth.NewStore(root, now).Active()
	if _, ok := active["codex"]; !ok || len(logs) != 1 || !strings.Contains(logs[0], "benched family codex") {
		t.Fatalf("active=%v logs=%q", active, logs)
	}
}

// TestExports_AreNamed names every exported type of the package for apicover:
// the handle is a core.Bridge, and the three function types are what the
// composition root passes in.
func TestExports_AreNamed(t *testing.T) {
	var _ core.Bridge = (*bridgechain.Walking)(nil)
	var resolve bridgechain.PlanResolver = fixedPlan(nil, nil)
	var bench bridgechain.BenchFunc = func(string, string, string, time.Time, map[string]string) {}
	var logf bridgechain.Logf = func(string, ...any) {}
	var opts []bridgechain.Option = []bridgechain.Option{bridgechain.WithBench(bench), bridgechain.WithLog(logf), bridgechain.WithClock(time.Now)}
	if w := bridgechain.New(&scripted{}, resolve, opts...); w == nil {
		t.Fatal("New")
	}
}

type centerBearing struct {
	scripted
	center *signalcenter.Center
}

func (c *centerBearing) Signals() *signalcenter.Center { return c.center }
func (c *centerBearing) SignalsWired() bool            { return c.center != nil }

// TestWalking_ForwardsTheInnerSignalCenter — the runner's verdict engine
// reaches the production Center by asserting Signals() on the bridge handle it
// was built with (phases/runner verdict_engine.go); the swarm decorator and
// the orchestrator ask SignalsWired(). A handle that hid them would silently
// disconnect every runner from the stream — the cmd/evolve wiring pin
// (TestWireOrchestratorDeps_SignalCenterReachesEveryPhaseRunner) is what
// caught it on the first wrap.
func TestWalking_ForwardsTheInnerSignalCenter(t *testing.T) {
	center := signalcenter.New()
	w := bridgechain.New(&centerBearing{center: center}, nil)
	if w.Signals() != center || !w.SignalsWired() {
		t.Fatal("the walked handle must expose the inner adapter's Center")
	}
	bare := bridgechain.New(&scripted{}, nil)
	if bare.Signals() != nil || bare.SignalsWired() {
		t.Fatal("a Center-less inner bridge stays Center-less through the handle")
	}
}
