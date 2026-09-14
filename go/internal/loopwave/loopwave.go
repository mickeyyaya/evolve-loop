// Package loopwave is unit 13 of the component breakdown (ADR-0103): the
// loop's wave engine. One Engine owns the sequential-vs-wave gate, the ONE
// dispatch body the wave fan-out and the min-width repair share, the fleet
// config loaders, the freshness-gated launcher, the quota/budget sizing and
// the plan source (the prior cycle's triage decision, pruned then widened, or
// the inbox seed). The wave coordinator (cmd_loop_window.go), the pool
// scheduler, the budget probe and the loop's halt and escalation producers
// stay in cmd/evolve. The Engine holds the project roots, four explicit ports
// (the prior-cycle readers, the protected-surface predicate, the bench
// shrink), the warn writer the KEPT report lines still print to, and the
// Signal Center accessor; it reports its failure modes as loop.wave WARN
// under module loop (every event carries fields.wave).
//
// The leaf lives beside cmd/evolve, not under internal/core: fleet, guards,
// inboxmover and triagecap reach internal/core transitively, so a core
// sub-package would cycle. Design:
// docs/architecture/decomposition/13-loopwave.md.
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

// The unit's codes: the min-width repair (re-homed from cmd/evolve's
// signal_loop.go, doc verbatim) and the three WARN conditions that replaced
// hand-written stderr lines.
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

// Roots carries both project roots: production derives EvolveDir as
// <ProjectRoot>/.evolve but --evolve-dir can move it, and each call site keeps
// the root it read before the move (the plan and the seed read EvolveDir; the
// lifecycle probe, the refill, the bench store and the routed resolver read
// ProjectRoot/.evolve). Invariant: both halves populated, or neither — the
// zero Roots is the rootless engine of the pure dispatch facades, which reach
// no root at all; a half-populated Roots would resolve the empty half
// CWD-relative. RootsOf builds the pair from a project root alone.
type Roots struct {
	ProjectRoot string
	EvolveDir   string
}

// RootsOf derives both halves from projectRoot the production way
// (paths.EvolveDirOf) — for the host facades that carry only a project root
// (the pool scheduler, the budget wrapper), whose paths always read
// <projectRoot>/.evolve. No project root is the rootless zero Roots (never
// the CWD-relative ".evolve" half).
func RootsOf(projectRoot string) Roots {
	if projectRoot == "" {
		return Roots{}
	}
	return Roots{ProjectRoot: projectRoot, EvolveDir: paths.EvolveDirOf(projectRoot)}
}

// ShrinkFn is the quota-bench shrink's shape (fleet.QuotaAwareCount).
type ShrinkFn func(count int, benched map[string]string, minLanes int, warn io.Writer) int

// Ports are the engine's required collaborators — the two prior-cycle readers
// over the host's storage, the protected-surface predicate and the bench
// shrink. A nil port is a programming error and panics at first use.
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

// Option configures an Engine at construction (functional options).
type Option func(*Engine)

// New builds the engine over its roots, ports and the warn writer the kept
// report lines print to.
func New(roots Roots, ports Ports, warn io.Writer, opts ...Option) *Engine {
	e := &Engine{roots: roots, ports: ports, stderr: warn}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// WithSignals installs the accessor of the Signal Center the unit reports
// through — read at every use, because the batch Center is built per batch
// and the coordinator is assembled as a literal. A nil accessor, or one
// returning nil, is the Null Object.
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

// warn is the unit's one producer helper: a loop.wave WARN through EmitWave.
func (e *Engine) warn(origin string, wave int, code signalcenter.Code, reason string, fields map[string]string) {
	EmitWave(e.center(), wave, origin, code, reason, fields)
}

// EmitWave is the ONE loop.wave producer: batch-level (no cycle), the wave
// number stamped on the producer's own copy of the caller's fields; INFO
// without a code, WARN with one. cmd/evolve's emitLoopWave projects onto it.
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

// ShouldRunWave reports whether a batch iteration dispatches a wave (fans out
// disjoint lanes) instead of the sequential path: Count>1 (a fleet block is
// configured) AND the resolved PlanSource is "triage" AND the scheduling is
// not "pool" (the pool gate is mutually exclusive, so one iteration never
// fires both). This is the ONLY seam that decides sequential-vs-wave.
func ShouldRunWave(fc policy.FleetConfig) bool {
	return fc.Count > 1 && fc.PlanSource == "triage" && fc.Scheduling != "pool"
}

// LoadFleetConfig loads the fleet block from paths.PolicyPath(evolveDir).
// Absent or malformed policy falls back to the built-in defaults (Count=1 —
// the sequential path): the batch-START loader.
func LoadFleetConfig(evolveDir string) policy.FleetConfig {
	pol, err := policy.Load(paths.PolicyPath(evolveDir))
	if err != nil {
		return policy.Policy{}.FleetConfig()
	}
	return pol.FleetConfig()
}

// ReloadFleetConfig re-resolves the committed fleet block at a wave boundary
// (cycle 739) so an operator width directive committed mid-batch takes effect
// at the next wave. Same resolution as LoadFleetConfig with ONE deliberate
// divergence: an unreadable or malformed policy HOLDS prev (the operator's
// standing width commitment) and WARNs on warn — kept as a line, not a code,
// because the boundary reload runs on the pool path too (F3). The "fleet
// config reloaded" line prints only when a dispatch-relevant value changed
// (count, min_lanes, plan_source, scheduling — not concurrency). A package
// function like its batch-start sibling: the reload reads one root and never
// needed an engine.
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

// FailedLanes counts the lanes that errored or exited non-zero — the ONE
// declaration of the belief the repair reason and the coordinator's summary
// both read.
func FailedLanes(results []fleet.Result) int {
	failed := 0
	for _, r := range results {
		if r.Err != nil || r.ExitCode != 0 {
			failed++
		}
	}
	return failed
}
