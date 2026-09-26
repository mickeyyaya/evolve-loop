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

// The loop.wave signal codes this package registers and emits.
const (
	CodeMinWidthRepair     signalcenter.Code = "LOOP_MIN_WIDTH_REPAIR"
	CodeWaveDispatchFailed signalcenter.Code = "LOOP_WAVE_DISPATCH_FAILED"
	CodeWaveEmptyPlan      signalcenter.Code = "LOOP_WAVE_EMPTY_PLAN"
	CodeWaveAllLanesStale  signalcenter.Code = "LOOP_WAVE_ALL_LANES_STALE"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeMinWidthRepair, "the fleet shrank below its committed width and one isolated lane was dispatched instead (min-width repair)")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWaveDispatchFailed, "a wave (fields.path=wave) or the min-width repair (path=repair) could not dispatch: the control-plane preflight refused (step=preflight — uncommitted control-plane edits in the main checkout), the triage plan could not be produced (step=plan — no prior triage decision and the inbox seed found fewer than 2 file-disjoint lanes, or the last-cycle read failed) or the plan could not be adapted into disjoint lanes (step=adapt); the batch falls back to the sequential path, the only unisolated execution mode; fields.error is the step's own error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWaveEmptyPlan, "the wave planned zero lanes and the batch falls back to sequential: cause=empty_triage_plan — fleet.count <= 1 so the one-lane repair guard is not met (the operator wanted one lane; fires whatever the dispatcher's own reason was); cause=empty_backlog — the guard was met but the one-lane repair found no disjoint candidate; fields.desired = fleet.count, realized = the wave-sized count")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWaveAllLanesStale, "every planned lane of the wave was stale at the last-moment freshness gate (consumed, or a declared dep still unmet) and the backlog refill found nothing, so the wave launched nothing — a shorter wave, never a doomed lane; the per-lane skips are fleet's own freshness-gate lines; fields.planned, skipped")
}

// Roots carries the project root and the evolve dir, which --evolve-dir can move.
// Set both or neither: an empty half would resolve CWD-relative.
type Roots struct {
	ProjectRoot string
	EvolveDir   string
}

// RootsOf derives both roots from projectRoot; an empty projectRoot yields the zero Roots.
func RootsOf(projectRoot string) Roots {
	if projectRoot == "" {
		return Roots{}
	}
	return Roots{ProjectRoot: projectRoot, EvolveDir: paths.EvolveDirOf(projectRoot)}
}

// ShrinkFn is the quota-bench shrink's shape (fleet.QuotaAwareCount).
type ShrinkFn func(count int, benched map[string]string, minLanes int, warn io.Writer) int

// Ports are the engine's required collaborators; a nil port panics at first use.
type Ports struct {
	LastCycle func(ctx context.Context) (int, error)
	Workspace func(cycle int) string
	Protected func(path string) bool
	Shrink    ShrinkFn
}

// Engine is the wave engine; build it with New.
type Engine struct {
	roots   Roots
	ports   Ports
	stderr  io.Writer
	signals func() *signalcenter.Center
}

// Option configures an Engine at construction.
type Option func(*Engine)

// New builds an Engine; warn receives the report lines that are not signals.
func New(roots Roots, ports Ports, warn io.Writer, opts ...Option) *Engine {
	e := &Engine{roots: roots, ports: ports, stderr: warn}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// WithSignals installs the Signal Center accessor. It is read at every use
// because the batch Center is built per batch; a nil accessor or Center emits nothing.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(e *Engine) { e.signals = c }
}

// SignalsWired reports whether the engine currently reaches a Center.
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

// EmitWave emits a batch-level loop.wave event, INFO without a code and WARN
// with one, stamping the wave on a copy of the caller's fields.
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

// ShouldRunWave reports whether a batch iteration dispatches a wave instead of
// the sequential path. Pool scheduling is excluded so one iteration never fires both gates.
func ShouldRunWave(fc policy.FleetConfig) bool {
	return fc.Count > 1 && fc.PlanSource == "triage" && fc.Scheduling != "pool"
}

// LoadFleetConfig loads the batch-start fleet block; an absent or malformed policy yields the defaults.
func LoadFleetConfig(evolveDir string) policy.FleetConfig {
	pol, err := policy.Load(paths.PolicyPath(evolveDir))
	if err != nil {
		return policy.Policy{}.FleetConfig()
	}
	return pol.FleetConfig()
}

// ReloadFleetConfig re-resolves the fleet block at a wave boundary. Unlike
// LoadFleetConfig, an unreadable policy holds prev, the operator's standing width.
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

// FailedLanes counts the lanes that errored or exited non-zero.
func FailedLanes(results []fleet.Result) int {
	failed := 0
	for _, r := range results {
		if r.Err != nil || r.ExitCode != 0 {
			failed++
		}
	}
	return failed
}
