// Package bridgechain makes the CLI/tier fallback chain a property of the
// bridge HANDLE, not of one caller.
// See docs/architecture/packages/internal-bridgechain.md.
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

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const exitQuota = 85

type PlanResolver func(req core.BridgeRequest) (llmroute.Plan, error)

type BenchFunc func(Escalation)

type Escalation struct {
	ProjectRoot   string
	Workspace     string
	CLI           string
	Tier          string
	DispatchStart time.Time
	Env           map[string]string
}

type Logf func(format string, args ...any)

type Walking struct {
	inner   core.Bridge
	resolve PlanResolver
	bench   BenchFunc
	now     func() time.Time
	logf    Logf
}

type Option func(*Walking)

func WithBench(b BenchFunc) Option { return func(w *Walking) { w.bench = b } }

func WithClock(now func() time.Time) Option { return func(w *Walking) { w.now = now } }

func WithLog(l Logf) Option { return func(w *Walking) { w.logf = l } }

func New(inner core.Bridge, resolve PlanResolver, opts ...Option) *Walking {
	w := &Walking{inner: inner, resolve: resolve, now: time.Now, logf: func(string, ...any) {}}
	for _, o := range opts {
		o(w)
	}
	return w
}

func (w *Walking) Probe(ctx context.Context) (core.BridgeProbe, error) { return w.inner.Probe(ctx) }

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
		last, lastErr = core.LaunchWithFreshSession(ctx, w.inner, attempt)
		keeper.Observe(cli+"@"+tier, last, lastErr)
		attempts = append(attempts, fmt.Sprintf("%s@%s=%d", cli, tier, last.ExitCode))
		if last.ExitCode == exitQuota && w.bench != nil {
			w.bench(Escalation{ProjectRoot: req.ProjectRoot, Workspace: req.Workspace, CLI: cli, Tier: tier, DispatchStart: start, Env: req.Env})
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
			NeedsTools: req.Worktree != "",
		})
		return d.Plan, err
	}
}

func CLIHealthEnabled(env map[string]string) bool {
	return envchain.BoolValue(envchain.Resolve("EVOLVE_CLI_HEALTH", env, "", "1"), true)
}

func ApplyCLIHealthBench(projectRoot, phase string, plan llmroute.Plan, env map[string]string, now func() time.Time, logf Logf) llmroute.Plan {
	if !CLIHealthEnabled(env) {
		return plan
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if now == nil {
		now = time.Now
	}
	active := clihealth.NewStore(projectRoot, now).Active()
	if len(active) == 0 {
		return plan
	}
	bootDrivers := make(map[string]time.Time, len(active))
	familyBenched := make(map[string]time.Time, len(active))
	for key, e := range active {
		switch {
		case e.Reason == clihealth.BootTimeoutPattern:
			bootDrivers[key] = e.BenchedAt
		case e.Scope == "" || scopeCoversSeat(e, plan):
			benchFamily(familyBenched, e)
		}
	}
	out := llmroute.ApplyDriverBench(plan, bootDrivers)
	if !slices.Equal(plan.Candidates, out.Candidates) {
		logf("phase=%s cli-health driver-bench reordered chain: %v -> %v (boot-benched: %v)\n",
			phase, plan.Candidates, out.Candidates, benchedSummary(active, now()))
	}
	afterDriver := out
	out = llmroute.ApplyBench(out, familyBenched)
	if !slices.Equal(afterDriver.Candidates, out.Candidates) {
		logf("phase=%s cli-health bench reordered chain: %v -> %v (benched: %v)\n",
			phase, afterDriver.Candidates, out.Candidates, benchedSummary(active, now()))
	} else if allBenched(out.Candidates, familyBenched) {
		logf("WARN phase=%s ALL candidates benched (%v) — dispatching least-recently-benched first; bench is advice, not a veto\n",
			phase, benchedSummary(active, now()))
	}
	return out
}

func benchFamily(benched map[string]time.Time, e clihealth.Entry) {
	if prev, ok := benched[e.Family]; !ok || e.BenchedAt.Before(prev) {
		benched[e.Family] = e.BenchedAt
	}
}

func scopeCoversSeat(e clihealth.Entry, plan llmroute.Plan) bool {
	tier := plan.Model
	if len(plan.Tiers) > 0 {
		tier = plan.Tiers[0]
	}
	for _, cli := range plan.Candidates {
		if llmroute.Family(cli) != e.Family {
			continue
		}
		if clihealth.ModelGroup(seatModel(cli, tier)) == e.Scope {
			return true
		}
	}
	return false
}

func seatModel(cli, tier string) string {
	if m, err := bridge.LoadManifest(cli); err == nil && m.ModelTierMap[tier] != "" {
		return m.ModelTierMap[tier]
	}
	return tier
}

var perModelQuotaFamilies = map[string]bool{"agy": true}

func wallScope(cli, tier string) string {
	if !perModelQuotaFamilies[llmroute.Family(cli)] {
		return ""
	}
	m, err := bridge.LoadManifest(cli)
	if err != nil {
		return ""
	}
	return clihealth.ModelGroup(m.ModelTierMap[tier])
}

func allBenched(candidates []string, benched map[string]time.Time) bool {
	for _, cli := range candidates {
		if _, hit := benched[llmroute.Family(cli)]; !hit {
			return false
		}
	}
	return len(candidates) > 0
}

func benchedSummary(active map[string]clihealth.Entry, now time.Time) []string {
	out := make([]string, 0, len(active))
	for fam, e := range active {
		out = append(out, fmt.Sprintf("%s until %s (%s)", fam, benchEnd(e.BenchedUntil, now), e.Reason))
	}
	sort.Strings(out)
	return out
}

func benchEnd(until, now time.Time) string {
	until = until.In(now.Location())
	if y, m, d := until.Date(); y == now.Year() && m == now.Month() && d == now.Day() {
		return until.Format("15:04")
	}
	return until.Format("2006-01-02 15:04")
}

type escalationReport struct {
	CapturedAt time.Time `json:"captured_at"`
	CLI        string    `json:"cli"`
	Pattern    string    `json:"pattern_name"`
	PaneTail   string    `json:"pane_tail"`
}

func BenchOnEscalation(e Escalation, now func() time.Time, logf Logf) {
	if !CLIHealthEnabled(e.Env) {
		return
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	raw, err := os.ReadFile(filepath.Join(e.Workspace, "escalation-report.json"))
	if err != nil {
		return
	}
	var rep escalationReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return
	}
	if !clihealth.Benchable(rep.Pattern) || rep.CLI != e.CLI || rep.CapturedAt.Before(e.DispatchStart) {
		return
	}
	target := clihealth.Entry{Family: llmroute.Family(e.CLI), Scope: wallScope(e.CLI, e.Tier)}
	store := clihealth.NewStore(e.ProjectRoot, now)
	if benchedFor(store, target.Key(), rep.CapturedAt) {
		logf("cli-health: family %s is already benched for this wall; not benched again\n", target.Key())
		return
	}
	entry, err := store.BenchScopedWall(target.Family, target.Scope, rep.Pattern, rep.PaneTail)
	if err != nil {
		logf("WARN cli-health bench write failed: %v\n", err)
		return
	}
	logf("cli-health: benched family %s until %s (pattern=%s strikes=%d)%s\n",
		entry.Key(), entry.BenchedUntil.Format(time.RFC3339), rep.Pattern, entry.Strikes, operatorSuffix(entry))
}

func benchedFor(store *clihealth.Store, key string, wallSeen time.Time) bool {
	benches, _ := store.Load()
	prev, ok := benches[key]
	return ok && !prev.BenchedAt.Before(wallSeen)
}

func (w *Walking) Signals() *signalcenter.Center {
	if src, ok := w.inner.(interface{ Signals() *signalcenter.Center }); ok {
		return src.Signals()
	}
	return nil
}

func (w *Walking) SignalsWired() bool {
	if src, ok := w.inner.(interface{ SignalsWired() bool }); ok {
		return src.SignalsWired()
	}
	return false
}

func operatorSuffix(entry clihealth.Entry) string {
	if entry.OperatorAction != "" {
		return " — " + entry.OperatorAction
	}
	return ""
}
