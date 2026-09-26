// Package bridgechain makes the CLI/tier fallback chain a property of the
// bridge HANDLE, not of one caller.
//
// The shared phase runner has walked a chain since WS-G1 (llmroute.Dispatch)
// and WS-876 (DispatchTiered): on a trigger exit — 80 REPL boot timeout, 81
// artifact timeout, 124 command timeout, 127 missing binary — it advances to
// the next CLI; when every CLI at a tier exits 85 (a quota wall) it steps down
// a tier; a classified wall benches the family (clihealth). Every launcher that
// called the bridge itself had none of it: the retro, the failure advisor, the
// phase judge, the retry adjudicator, the swarm launcher. Lanes 1676 and 1677
// (2026-09-14) paid for that: the retro launched on codex, codex timed out
// after 1800 s, the retro emitted FAIL with no disposition and the cycle sealed
// abnormally — one CLI's timeout costing the whole cycle's work.
//
// Walking is a Decorator (GoF) over core.Bridge. Wrapped ONCE at the
// composition root (cmd/evolve wireOrchestratorDeps) it gives every Launch the
// walk by construction; a caller that walks its own chain (the runner, the
// advisor) marks each attempt with BridgeRequest.ChainAttempt and is passed
// straight through, so no launch is walked twice. CLIHealthEnabled,
// ApplyCLIHealthBench and BenchOnEscalation are the runner's cli-health
// projections hoisted here so both walks share one implementation.
package bridgechain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// exitQuota is the bridge's escalate exit (ExitUnknownPrompt): a quota wall,
// a rejected model, an unanswerable prompt. DispatchTiered steps down a tier
// only when every CLI at the tier exits with it.
const exitQuota = 85

// PlanResolver yields the chain one launch walks: CLI candidates (primary
// first), trigger exits, tier chain. DefaultPlanResolver builds it from the
// same inputs the runner's resolveDispatchPlan uses.
type PlanResolver func(req core.BridgeRequest) llmroute.Plan

// BenchFunc records a wall after an attempt exited 85 (see BenchOnEscalation).
type BenchFunc func(projectRoot, workspace, cli string, dispatchStart time.Time, env map[string]string)

// Logf is the walk's diagnostic sink; the composition root points it at the
// loop's stderr so a fallback is never silent.
type Logf func(format string, args ...any)

// Walking is the chain-walking bridge handle.
type Walking struct {
	inner   core.Bridge
	resolve PlanResolver
	bench   BenchFunc
	now     func() time.Time
	logf    Logf
}

// Option configures a Walking bridge.
type Option func(*Walking)

// WithBench installs the wall recorder consulted after every exit-85 attempt.
func WithBench(b BenchFunc) Option { return func(w *Walking) { w.bench = b } }

// WithClock replaces the clock (the bench's staleness guard reads it).
func WithClock(now func() time.Time) Option { return func(w *Walking) { w.now = now } }

// WithLog installs the diagnostic sink.
func WithLog(l Logf) Option { return func(w *Walking) { w.logf = l } }

// New wraps inner so every Launch that is not already an attempt of a chain
// walk resolves its plan and walks it.
func New(inner core.Bridge, resolve PlanResolver, opts ...Option) *Walking {
	w := &Walking{inner: inner, resolve: resolve, now: time.Now, logf: func(string, ...any) {}}
	for _, o := range opts {
		o(w)
	}
	return w
}

// Probe delegates to the inner bridge.
func (w *Walking) Probe(ctx context.Context) (core.BridgeProbe, error) { return w.inner.Probe(ctx) }

// Launch walks the chain for req. A request marked ChainAttempt (one attempt of
// a caller's own walk) is handed to the inner bridge untouched. The final
// attempt's response and error are what the caller sees, exactly as the runner
// returns its final attempt; a legitimate FAIL (a non-trigger exit) never
// routes to another CLI.
func (w *Walking) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if req.ChainAttempt || w.resolve == nil {
		return w.inner.Launch(ctx, req)
	}
	plan := w.resolve(req)
	if len(plan.Candidates) == 0 {
		plan.Candidates = []string{req.CLI}
	}
	if len(plan.Triggers) == 0 {
		plan.Triggers = llmroute.DefaultTriggers()
	}
	if len(plan.Tiers) == 0 {
		plan.Tiers = []string{req.Model}
	}
	var last core.BridgeResponse
	var lastErr error
	var attempts []string
	llmroute.DispatchTiered(plan, func(cli, tier string) (int, error) {
		attempt := req
		attempt.CLI, attempt.ChainAttempt = cli, true
		if tier != "" {
			attempt.Model = tier
		}
		if n := len(attempts); n > 0 {
			w.logf("[bridge-chain] agent=%s fallback %d: trying cli=%s tier=%s (previous=%s exit=%d)\n",
				req.Agent, n+1, cli, tier, attempts[n-1], last.ExitCode)
		}
		start := w.now()
		last, lastErr = w.inner.Launch(ctx, attempt)
		attempts = append(attempts, fmt.Sprintf("%s@%s=%d", cli, tier, last.ExitCode))
		if last.ExitCode == exitQuota && w.bench != nil {
			w.bench(req.ProjectRoot, req.Workspace, cli, start, req.Env)
		}
		return last.ExitCode, lastErr
	}, func(from, to string) {
		w.logf("[bridge-chain] agent=%s tier step-down %s -> %s (every CLI at %s exited %d)\n", req.Agent, from, to, from, exitQuota)
	})
	return last, lastErr
}

// DefaultPlanResolver resolves a launch's chain the way the runner resolves a
// phase's: the agent's profile (primary, cli_fallback, tier chain, triggers —
// llmroute.Resolve), the caller's own CLI kept as the primary so wrapping never
// changes which CLI runs FIRST, a capability probe, the cli-health bench, and
// the universal fallback when the whole configured chain is absent. profilesDir
// is <evolveDir>/profiles; discover may be nil (no universal fallback);
// lookPath nil means exec.LookPath.
func DefaultPlanResolver(profilesDir string, discover func() []string, lookPath func(string) (string, error), now func() time.Time, logf Logf) PlanResolver {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if now == nil {
		now = time.Now
	}
	loader := profiles.NewFromDir(profilesDir)
	return func(req core.BridgeRequest) llmroute.Plan {
		var prof *profiles.Profile
		if loader != nil && req.Agent != "" {
			if p, err := loader.Get(req.Agent); err == nil {
				prof = &p
			}
		}
		plan := llmroute.Resolve(req.Agent, req.Agent, req.Model, req.Env, prof, nil, nil)
		plan.Candidates = leadWith(req.CLI, plan.Candidates)
		plan = llmroute.Probe(plan, lookPath)
		plan = ApplyCLIHealthBench(req.ProjectRoot, req.Agent, plan, req.Env, now, logf)
		if discover != nil {
			plan = llmroute.ApplyUniversalFallback(plan, llmroute.AllowedDiscovered(discover(), prof), lookPath)
		}
		return plan
	}
}

func leadWith(primary string, chain []string) []string {
	if primary == "" {
		return chain
	}
	out := []string{primary}
	for _, c := range chain {
		if c != primary {
			out = append(out, c)
		}
	}
	return out
}

// CLIHealthEnabled reports whether the cli-health bench applies (EVOLVE_CLI_HEALTH,
// default on; 0 disables).
func CLIHealthEnabled(env map[string]string) bool {
	return envchain.BoolValue(envchain.Resolve("EVOLVE_CLI_HEALTH", env, "", "1"), true)
}

// ApplyCLIHealthBench demotes candidates whose family has an ACTIVE bench so
// the chain starts at a healthy CLI (lazy expiry: a past-due bench stops
// demoting, giving the family its canary shot). Boot-timeout entries are
// driver-keyed (ApplyDriverBench), all others family-keyed (ApplyBench);
// driver-bench runs first. Each reorder is one logf line; when EVERY candidate
// is benched the order is kept and a WARN says so — the bench is advice, not
// a veto. No-op under EVOLVE_CLI_HEALTH=0.
func ApplyCLIHealthBench(projectRoot, phase string, plan llmroute.Plan, env map[string]string, now func() time.Time, logf Logf) llmroute.Plan {
	if !CLIHealthEnabled(env) {
		return plan
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	active := clihealth.NewStore(projectRoot, now).Active()
	if len(active) == 0 {
		return plan
	}
	bootDrivers := make(map[string]time.Time, len(active))
	familyBenched := make(map[string]time.Time, len(active))
	for key, e := range active {
		if e.Reason == clihealth.BootTimeoutPattern {
			bootDrivers[key] = e.BenchedAt
		} else {
			familyBenched[key] = e.BenchedAt
		}
	}
	out := llmroute.ApplyDriverBench(plan, bootDrivers)
	if !slices.Equal(plan.Candidates, out.Candidates) {
		logf("phase=%s cli-health driver-bench reordered chain: %v -> %v (boot-benched: %v)\n",
			phase, plan.Candidates, out.Candidates, benchedSummary(active))
	}
	afterDriver := out
	out = llmroute.ApplyBench(out, familyBenched)
	if !slices.Equal(afterDriver.Candidates, out.Candidates) {
		logf("phase=%s cli-health bench reordered chain: %v -> %v (benched: %v)\n",
			phase, afterDriver.Candidates, out.Candidates, benchedSummary(active))
	} else if allBenched(out.Candidates, familyBenched) {
		logf("WARN phase=%s ALL candidates benched (%v) — dispatching least-recently-benched first; bench is advice, not a veto\n",
			phase, benchedSummary(active))
	}
	return out
}

func allBenched(candidates []string, benched map[string]time.Time) bool {
	for _, cli := range candidates {
		if _, hit := benched[llmroute.Family(cli)]; !hit {
			return false
		}
	}
	return len(candidates) > 0
}

func benchedSummary(active map[string]clihealth.Entry) []string {
	out := make([]string, 0, len(active))
	for fam, e := range active {
		out = append(out, fmt.Sprintf("%s until %s (%s)", fam, e.BenchedUntil.Format("15:04"), e.Reason))
	}
	sort.Strings(out) // deterministic log lines (transcript-diff friendly)
	return out
}

// escalationReport mirrors the fields bridge/autorespond.go writeEscalation
// persists (escalation-report.json in the workspace).
type escalationReport struct {
	CapturedAt time.Time `json:"captured_at"`
	CLI        string    `json:"cli"`
	Pattern    string    `json:"pattern_name"`
	PaneTail   string    `json:"pane_tail"`
}

// BenchOnEscalation benches cli's family when the workspace escalation report
// classifies a benchable wall for THIS dispatch. Staleness guard: the report
// must name the CLI that just exited 85 AND be captured at/after this
// dispatch's start (the workspace is shared across phases; a leftover report
// must never bench). benched_until comes from the pane's own reset hint when
// parseable, else the strike-scaled cooldown.
func BenchOnEscalation(projectRoot, workspace, cli string, dispatchStart time.Time, env map[string]string, now func() time.Time, logf Logf) {
	if !CLIHealthEnabled(env) {
		return
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	raw, err := os.ReadFile(filepath.Join(workspace, "escalation-report.json"))
	if err != nil {
		return // no report — generic 85, nothing to classify
	}
	var rep escalationReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return
	}
	if !clihealth.Benchable(rep.Pattern) || rep.CLI != cli || rep.CapturedAt.Before(dispatchStart) {
		return
	}
	family := llmroute.Family(cli)
	entry, err := clihealth.NewStore(projectRoot, now).BenchWall(family, rep.Pattern, rep.PaneTail)
	if err != nil {
		logf("WARN cli-health bench write failed: %v\n", err)
		return
	}
	logf("cli-health: benched family %s until %s (pattern=%s strikes=%d)%s\n",
		family, entry.BenchedUntil.Format(time.RFC3339), rep.Pattern, entry.Strikes, operatorSuffix(entry))
}

// Signals forwards the inner adapter's Signal Center so the verdict engine of
// every runner built off the wrapped handle still reaches the production
// Center (phases/runner verdict_engine.go asserts this method on its bridge);
// nil when the inner bridge carries none.
func (w *Walking) Signals() *signalcenter.Center {
	if src, ok := w.inner.(interface{ Signals() *signalcenter.Center }); ok {
		return src.Signals()
	}
	return nil
}

// SignalsWired forwards the inner adapter's wiring proof (the swarm decorator
// and the orchestrator ask it).
func (w *Walking) SignalsWired() bool {
	if src, ok := w.inner.(interface{ SignalsWired() bool }); ok {
		return src.SignalsWired()
	}
	return false
}

// operatorSuffix appends the operator's fix to a bench line when the wall needs one.
func operatorSuffix(entry clihealth.Entry) string {
	if entry.OperatorAction != "" {
		return " — " + entry.OperatorAction
	}
	return ""
}
