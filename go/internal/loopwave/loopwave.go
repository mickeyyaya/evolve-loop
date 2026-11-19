// Package loopwave is the loop's wave engine: the sequential-vs-wave gate, the
// shared dispatch body, the fleet-config loaders, the freshness-gated launcher,
// wave sizing and the plan source. See docs/architecture/packages/internal-loopwave.md.
package loopwave

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeMinWidthRepair     signalcenter.Code = "LOOP_MIN_WIDTH_REPAIR"
	CodeWaveDispatchFailed signalcenter.Code = "LOOP_WAVE_DISPATCH_FAILED"
	CodeWaveEmptyPlan      signalcenter.Code = "LOOP_WAVE_EMPTY_PLAN"
	CodeWaveAllLanesStale  signalcenter.Code = "LOOP_WAVE_ALL_LANES_STALE"
	CodeOrchAdmitWait      signalcenter.Code = "LOOP_ORCH_ADMIT_WAIT"
	CodeFleetPolicyInvalid signalcenter.Code = "LOOP_FLEET_POLICY_INVALID"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeMinWidthRepair, "the fleet shrank below its committed width and one isolated lane was dispatched instead (min-width repair)")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWaveDispatchFailed, "a wave (fields.path=wave) or min-width repair (path=repair) failed to dispatch. Steps: preflight (uncommitted control-plane edits), plan (missing triage or fewer than 2 disjoint lanes), or adapt. The batch falls back to sequential mode. fields.error holds the step error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWaveEmptyPlan, "the wave planned zero lanes and falls back to sequential. cause=empty_triage_plan when fleet.count <= 1 (one-lane repair guard unmet); cause=empty_backlog when the guard met no disjoint candidate. fields.desired = fleet.count, realized = wave count")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeOrchAdmitWait, "the admission shadow (fleet.orchestrator.stage, ADR-0139 O3) held a planned todo whose package cell or id family overlaps an earlier-ranked todo. In shadow, it records the hold and changes no lane. Fields: todo, rank, cells, blocked_by, blocked_by_lane, stage")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeFleetPolicyInvalid, "the loop failed to load .evolve/policy.json at batch start (unreadable, malformed, or strict-schema error). Fleet config fell back to defaults and runs sequential (count 1) until fixed. The reason holds the load error; fields.path")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWaveAllLanesStale, "every planned wave lane was stale at the freshness gate (consumed or unmet dep) and backlog refill found nothing. The wave launched nothing (never a doomed lane); fleet logs per-lane skips. fields.planned, skipped")
}

type Roots struct {
	ProjectRoot string
	EvolveDir   string
}

func RootsOf(projectRoot string) Roots {
	if projectRoot == "" {
		return Roots{}
	}
	return Roots{ProjectRoot: projectRoot, EvolveDir: paths.EvolveDirOf(projectRoot)}
}

type ShrinkFn func(count int, benched map[string]string, minLanes int, warn io.Writer) int

type ChainsFn func(block *policy.CLIRouting) [][]string

type Ports struct {
	LastCycle func(ctx context.Context) (int, error)
	Workspace func(cycle int) string
	Protected func(path string) bool
	Shrink    ShrinkFn
	Chains    ChainsFn
}

type Engine struct {
	roots     Roots
	ports     Ports
	stderr    io.Writer
	signals   func() *signalcenter.Center
	goal      string
	admission AdmissionFn
	selector  Selector
}

type Option func(*Engine)

func New(roots Roots, ports Ports, warn io.Writer, opts ...Option) *Engine {
	e := &Engine{roots: roots, ports: ports, stderr: warn}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(e *Engine) { e.signals = c }
}

func WithGoal(hash string) Option {
	return func(e *Engine) { e.goal = hash }
}

func (e *Engine) SignalsWired() bool { return e.center() != nil }

func (e *Engine) center() *signalcenter.Center {
	if e.signals == nil {
		return nil
	}
	return e.signals()
}

func (e *Engine) warn(origin string, wave int, code signalcenter.Code, reason string, fields map[string]string) {
	EmitWave(e.center(), wave, origin, code, reason, fields)
}

func EmitWave(c *signalcenter.Center, wave int, origin string, code signalcenter.Code, reason string, fields map[string]string) {
	stamped := make(map[string]string, len(fields)+1)
	maps.Copy(stamped, fields)
	stamped["wave"] = strconv.Itoa(wave)
	e := signalcenter.Event{
		Module: signalcenter.ModuleLoop, Origin: origin, Kind: signalcenter.KindLoopWave,
		Severity: signalcenter.SeverityInfo, Reason: reason, Fields: stamped,
	}
	if code != "" {
		e.Severity, e.Code = signalcenter.SeverityWarn, code
	}
	c.Emit(e)
}

func ShouldRunWave(fc policy.FleetConfig) bool {
	return fc.Count > 1 && fc.PlanSource == "triage" && !ShouldRunPool(fc)
}

func ShouldRunPool(fc policy.FleetConfig) bool {
	return fc.Count > 1 && fc.PlanSource == "triage" && (fc.Scheduling == "pool" || fc.Orchestrator.Stage == "enforce")
}

func PoolSupply(fc policy.FleetConfig) int {
	if fc.Orchestrator.Stage != "enforce" {
		return fc.Count
	}
	return max(fc.Count, fc.Orchestrator.ScanK)
}

func ShouldRunPlan(fc policy.FleetConfig) bool {
	return ShouldRunWave(fc) || ShouldRunPool(fc)
}

func LoadFleetConfig(evolveDir string, warn io.Writer, c *signalcenter.Center) policy.FleetConfig {
	path := paths.PolicyPath(evolveDir)
	pol, err := policy.Load(path)
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: fleet: .evolve/policy.json is not valid (%v); the loop runs sequential until it is fixed\n", err)
		c.Emit(signalcenter.Event{
			Module: signalcenter.ModuleLoop, Origin: "loopwave.LoadFleetConfig", Kind: signalcenter.KindLoopWarning,
			Severity: signalcenter.SeverityWarn, Code: CodeFleetPolicyInvalid,
			Reason: fmt.Sprintf("fleet: %s is not valid (%v); the loop runs sequential until it is fixed", path, err),
			Fields: map[string]string{"path": path},
		})
		return policy.Policy{}.FleetConfig()
	}
	return pol.FleetConfig()
}

func LoadRouting(evolveDir string, warn io.Writer) *policy.CLIRouting {
	pol, err := policy.Load(paths.PolicyPath(evolveDir))
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: fleet: the routing chains are unknown (%v): every bench costs a lane\n", err)
		return nil
	}
	return pol.CLIRouting
}

func ReloadFleetConfig(evolveDir string, prev policy.FleetConfig, warn io.Writer) policy.FleetConfig {
	pol, err := policy.Load(paths.PolicyPath(evolveDir))
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: fleet: policy.json unreadable at wave boundary (%v) — holding fleet config count=%d min_lanes=%d\n", err, prev.Count, prev.MinLanes)
		return prev
	}
	got := pol.FleetConfig()
	if got.Count != prev.Count || got.MinLanes != prev.MinLanes ||
		got.PlanSource != prev.PlanSource || got.Scheduling != prev.Scheduling {
		for _, w := range got.Warnings {
			fmt.Fprintf(warn, "[loop] WARN: fleet: %s\n", w)
		}
		fmt.Fprintf(warn, "[loop] fleet config reloaded: count=%d min_lanes=%d\n", got.Count, got.MinLanes)
	}
	return got
}

type LaneTally struct {
	OK       int
	Deferred int
	Failed   int
}

func Tally(results []fleet.Result) LaneTally {
	var t LaneTally
	for _, r := range results {
		if errors.Is(r.Err, fleet.ErrNeverLaunched) {
			continue
		}
		switch r.Status() {
		case fleet.LaneOK:
			t.OK++
		case fleet.LaneDeferred:
			t.Deferred++
		default:
			t.Failed++
		}
	}
	return t
}

func (t LaneTally) Total() int { return t.OK + t.Deferred + t.Failed }

func (t LaneTally) DeferredSuffix() string {
	if t.Deferred == 0 {
		return ""
	}
	return fmt.Sprintf(", %d deferred", t.Deferred)
}

func WaveSummary(wave int, t LaneTally) (string, map[string]string) {
	fields := map[string]string{"lanes_ok": strconv.Itoa(t.OK), "lanes": strconv.Itoa(t.Total())}
	if t.Deferred > 0 {
		fields["lanes_deferred"] = strconv.Itoa(t.Deferred)
	}
	return fmt.Sprintf("wave %d: %d/%d lanes ok%s", wave, t.OK, t.Total(), t.DeferredSuffix()), fields
}
