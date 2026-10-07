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
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// exitQuota is the bridge's escalate exit (ExitUnknownPrompt): a quota wall,
// a rejected model, an unanswerable prompt. DispatchTiered steps down a tier
// only when every CLI at the tier exits with it.
const exitQuota = 85

type PlanResolver func(req core.BridgeRequest) (llmroute.Plan, error)

// BenchFunc records a wall after an attempt exited 85 (see BenchOnEscalation).
type BenchFunc func(Escalation)

type Escalation struct {
	ProjectRoot   string
	Workspace     string
	CLI           string
	DispatchStart time.Time
	Env           map[string]string
}

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
// a caller's own walk) is handed to the inner bridge untouched. The caller sees
// what the runner's walk returns (WallKeeper): the final attempt, or the quota
// wall of a walk that ran out after meeting one; a legitimate FAIL (a
// non-trigger exit) never routes to another CLI.
func (w *Walking) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if req.ChainAttempt || w.resolve == nil {
		return w.inner.Launch(ctx, req)
	}
	plan, err := w.resolve(req)
	if err != nil {
		return core.BridgeResponse{}, fmt.Errorf("bridge-chain: agent %s: %w", req.Agent, err)
	}
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
	var keeper WallKeeper
	walk := llmroute.DispatchTiered(plan, func(cli, tier string) (int, error) {
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
		keeper.Observe(cli+"@"+tier, last, lastErr)
		attempts = append(attempts, fmt.Sprintf("%s@%s=%d", cli, tier, last.ExitCode))
		if last.ExitCode == exitQuota && w.bench != nil {
			w.bench(Escalation{ProjectRoot: req.ProjectRoot, Workspace: req.Workspace, CLI: cli, DispatchStart: start, Env: req.Env})
		}
		return last.ExitCode, lastErr
	}, func(from, to string) {
		w.logf("[bridge-chain] agent=%s tier step-down %s -> %s (every CLI at %s exited %d)\n", req.Agent, from, to, from, exitQuota)
	})
	if err := Unlaunched(walk, plan); err != nil {
		return core.BridgeResponse{}, fmt.Errorf("bridge-chain: agent %s: %w", req.Agent, err)
	}
	return keeper.Surface(walk, last, lastErr)
}

func DefaultPlanResolver(router *cliroute.Router) PlanResolver {
	return func(req core.BridgeRequest) (llmroute.Plan, error) {
		d, err := router.Resolve(cliroute.Request{
			Agent: req.Agent, ProjectRoot: req.ProjectRoot, DefaultModel: req.Model, Env: req.Env, CallerCLI: req.CLI,
		})
		return d.Plan, err
	}
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
func BenchOnEscalation(e Escalation, now func() time.Time, logf Logf) {
	if !CLIHealthEnabled(e.Env) {
		return
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	raw, err := os.ReadFile(filepath.Join(e.Workspace, "escalation-report.json"))
	if err != nil {
		return // no report — generic 85, nothing to classify
	}
	var rep escalationReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return
	}
	if !clihealth.Benchable(rep.Pattern) || rep.CLI != e.CLI || rep.CapturedAt.Before(e.DispatchStart) {
		return
	}
	family := llmroute.Family(e.CLI)
	store := clihealth.NewStore(e.ProjectRoot, now)
	if benchedFor(store, family, rep.CapturedAt) {
		logf("cli-health: family %s is already benched for this wall; not benched again\n", family)
		return
	}
	entry, err := store.BenchWall(family, rep.Pattern, rep.PaneTail)
	if err != nil {
		logf("WARN cli-health bench write failed: %v\n", err)
		return
	}
	logf("cli-health: benched family %s until %s (pattern=%s strikes=%d)%s\n",
		family, entry.BenchedUntil.Format(time.RFC3339), rep.Pattern, entry.Strikes, operatorSuffix(entry))
}

func benchedFor(store *clihealth.Store, family string, wallSeen time.Time) bool {
	benches, _ := store.Load()
	prev, ok := benches[family]
	return ok && !prev.BenchedAt.Before(wallSeen)
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
