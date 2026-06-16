// Package phaseio is the dependency-free contract leaf for the unified phase
// I/O envelope (ADR-0050 Phase 3). It defines the typed input/output a phase
// filter communicates through — a true pipes-and-filters contract (P4): each
// phase is given sufficient, sealed context (PhaseInput) and emits a typed
// result (PhaseOutput) without reaching for sibling artifacts or sharing a
// mutable map.
//
// Dependency rule (P3): this package imports only internal/phasespec (itself a
// leaf). It must NEVER import internal/core or internal/router — those depend on
// phaseio, never the reverse. The assembler that maps router.RoutingSignals →
// phaseio.Handoffs therefore lives in a higher package (Phase 3.3/3.4), not here.
//
// As of Phase 3.1 nothing reads these types in the live loop; they ship dormant
// behind EVOLVE_PHASE_IO=off (Phase 3.2) and become load-bearing only after the
// shadow-equivalence harness proves them equivalent to the legacy path.
package phaseio

import "github.com/mickeyyaya/evolve-loop/go/internal/phasespec"

// ErrorContext is the typed error channel piped to a recovery phase (e.g.
// debugger), replacing the ad-hoc ship_error_code/class/stage/debug entries
// injected into the mutable Context map. nil on PhaseInput means no upstream
// error.
type ErrorContext struct {
	Code  string // ship_error_code (e.g. "E_PUSH_NONFF")
	Class string // ship_error_class (transient|terminal|…)
	Stage string // the phase/stage that produced the error (e.g. "ship")
	Debug string // free-form diagnostic detail
}

// CorrectionState is the typed re-dispatch channel for the contract-correction
// loop, replacing the "## Correction" markdown blob. Attempt carries the retry
// count so a phase can be idempotent on retry (P5).
type CorrectionState struct {
	Directive string // the correction directive handed to the re-dispatched phase
	Attempt   int    // 1-based re-dispatch attempt number
}

// PhaseInputInit is the construction DTO for NewPhaseInput. Identity/roots map
// to plain fields; the reference-typed channels (Env/Spec/Upstream/CycleInputs/
// Error/Correction) are sealed into the resulting PhaseInput.
type PhaseInputInit struct {
	Cycle            int
	RunID            string
	GoalHash         string
	ProjectRoot      string
	Workspace        string
	Worktree         string
	Phase            string
	PreviousPhase    string
	WorktreeWritable bool

	Env         map[string]string
	Spec        *phasespec.PhaseSpec
	Upstream    Handoffs
	CycleInputs CycleInputs
	Error       *ErrorContext
	Correction  *CorrectionState
}

// PhaseInput is the unified, sealed input envelope to a phase (P4/P5). Identity
// and roots are exported, immutable scalars (value semantics make a local copy
// harmless); the mutable-by-nature channels are sealed behind accessors so no
// phase can mutate the context a sibling observes. Build it with NewPhaseInput.
type PhaseInput struct {
	// Identity & roots — immutable scalars.
	Cycle            int
	RunID            string
	GoalHash         string
	ProjectRoot      string // the MAIN repo root (RUNTIME-DATA root; all .evolve/ state lives here)
	Workspace        string // this cycle's per-phase scratch dir under .evolve/runs/cycle-<N>/
	Worktree         string // the per-cycle git worktree (SHIPPED-TREE root); empty only when provisioning failed
	Phase            string
	PreviousPhase    string
	WorktreeWritable bool // whether THIS phase may write source in the worktree (per-phase isolation seam)

	// Sealed channels.
	env         map[string]string
	spec        *phasespec.PhaseSpec
	upstream    Handoffs
	cycleInputs CycleInputs
	errorCtx    *ErrorContext
	correction  *CorrectionState
}

// NewPhaseInput builds a sealed PhaseInput, deep-copying the Env map so later
// mutation of the caller's map cannot leak into a phase's observed context.
func NewPhaseInput(init PhaseInputInit) PhaseInput {
	return PhaseInput{
		Cycle:            init.Cycle,
		RunID:            init.RunID,
		GoalHash:         init.GoalHash,
		ProjectRoot:      init.ProjectRoot,
		Workspace:        init.Workspace,
		Worktree:         init.Worktree,
		Phase:            init.Phase,
		PreviousPhase:    init.PreviousPhase,
		WorktreeWritable: init.WorktreeWritable,
		env:              cloneStringMap(init.Env),
		spec:             init.Spec,
		upstream:         init.Upstream,
		cycleInputs:      init.CycleInputs,
		errorCtx:         cloneErrorContext(init.Error),
		correction:       cloneCorrectionState(init.Correction),
	}
}

// cloneErrorContext deep-copies an *ErrorContext so a caller mutating the init
// pointer after construction cannot leak into the sealed input. ErrorContext is
// scalar-only, so a value copy is a full deep copy.
func cloneErrorContext(e *ErrorContext) *ErrorContext {
	if e == nil {
		return nil
	}
	c := *e
	return &c
}

// cloneCorrectionState deep-copies a *CorrectionState (scalar-only) for the same
// sealing reason as cloneErrorContext.
func cloneCorrectionState(c *CorrectionState) *CorrectionState {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

// Env returns the value of a single sealed env key and ok=false when absent.
func (in PhaseInput) Env(key string) (string, bool) {
	v, ok := in.env[key]
	return v, ok
}

// EnvCopy returns a fresh copy of the sealed env map; mutating it does not
// affect the input.
func (in PhaseInput) EnvCopy() map[string]string {
	out := cloneStringMap(in.env)
	if out == nil {
		out = map[string]string{}
	}
	return out
}

// Spec returns the phase's declarative spec (nil for built-in Go phases that
// ignore it). The pointer is read-only by contract.
func (in PhaseInput) Spec() *phasespec.PhaseSpec { return in.spec }

// Upstream returns the sealed typed view of prior phases' outputs.
func (in PhaseInput) Upstream() Handoffs { return in.upstream }

// CycleInputs returns the sealed cycle-scoped inputs (goal/strategy/…).
func (in PhaseInput) CycleInputs() CycleInputs { return in.cycleInputs }

// Active reports whether this is a populated (authoritative) envelope rather than
// the zero value. The dispatch seam assembles a PhaseInput only at
// EVOLVE_PHASE_IO>=enforce, always stamping the Phase field; the zero value (off/
// shadow/advisory) leaves it empty. Phases gate on Active() to read the typed
// envelope at enforce while falling back to the legacy Context map below it —
// byte-identical until the cutover. (Phase is the populated-marker because it is
// always set on assembly and never on the zero value — a positive signal, not a
// fragile whole-struct zero comparison.)
func (in PhaseInput) Active() bool { return in.Phase != "" }

// ErrorContext returns the upstream error channel by value and ok=false when no
// error was piped in.
func (in PhaseInput) ErrorContext() (ErrorContext, bool) {
	if in.errorCtx == nil {
		return ErrorContext{}, false
	}
	return *in.errorCtx, true
}

// Correction returns the re-dispatch correction state by value and ok=false on
// a first (non-correction) dispatch.
func (in PhaseInput) Correction() (CorrectionState, bool) {
	if in.correction == nil {
		return CorrectionState{}, false
	}
	return *in.correction, true
}
