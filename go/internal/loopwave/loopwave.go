// Package loopwave is the loop's wave engine: the sequential-vs-wave gate, the
// shared dispatch body, the fleet-config loaders, the freshness-gated launcher,
// wave sizing and the plan source. See docs/architecture/packages/internal-loopwave.md.
package loopwave

import (
	"context"
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
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWaveDispatchFailed, "a wave (fields.path=wave) or the min-width repair (path=repair) could not dispatch: the control-plane preflight refused (step=preflight — uncommitted control-plane edits in the main checkout), the triage plan could not be produced (step=plan — no prior triage decision and the inbox seed found fewer than 2 file-disjoint lanes, or the last-cycle read failed) or the plan could not be adapted into disjoint lanes (step=adapt); the batch falls back to the sequential path, the only unisolated execution mode; fields.error is the step's own error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWaveEmptyPlan, "the wave planned zero lanes and the batch falls back to sequential: cause=empty_triage_plan — fleet.count <= 1 so the one-lane repair guard is not met (the operator wanted one lane; fires whatever the dispatcher's own reason was); cause=empty_backlog — the guard was met but the one-lane repair found no disjoint candidate; fields.desired = fleet.count, realized = the wave-sized count")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeOrchAdmitWait, "the admission shadow (fleet.orchestrator.stage, ADR-0139 O3) found a planned todo whose package cell overlaps the cell of a todo ranked before it, so admission by cell holds it back; in shadow the event only records the hold and no lane changes; fields.todo, rank, cells, blocked_by, blocked_by_lane (the partition lane of the blocker, or none), stage")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeFleetPolicyInvalid, "the loop could not load .evolve/policy.json at batch start (unreadable, malformed, or a key the strict schema rejects such as a misspelt fleet.orchestrator field), so the fleet config fell back to its defaults and the loop runs sequential (count 1) until the file is fixed; the reason carries the load error; fields.path")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWaveAllLanesStale, "every planned lane of the wave was stale at the last-moment freshness gate (consumed, or a declared dep still unmet) and the backlog refill found nothing, so the wave launched nothing — a shorter wave, never a doomed lane; the per-lane skips are fleet's own freshness-gate lines; fields.planned, skipped")
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

type Ports struct {
	LastCycle func(ctx context.Context) (int, error)
	Workspace func(cycle int) string
	Protected func(path string) bool
	Shrink    ShrinkFn
}

type Engine struct {
	roots     Roots
	ports     Ports
	stderr    io.Writer
	signals   func() *signalcenter.Center
	goal      string
	admission AdmissionFn
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
