// Package phaseio is the dependency-free contract leaf for the unified phase
// I/O envelope: the typed input a phase receives and the typed output it emits.
package phaseio

import "github.com/mickeyyaya/evolve-loop/go/internal/phasespec"

type ErrorContext struct {
	Code  string
	Class string
	Stage string
	Debug string
}

type CorrectionState struct {
	Directive string
	Attempt   int
}

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

type PhaseInput struct {
	Cycle            int
	RunID            string
	GoalHash         string
	ProjectRoot      string
	Workspace        string
	Worktree         string
	Phase            string
	PreviousPhase    string
	WorktreeWritable bool

	env         map[string]string
	spec        *phasespec.PhaseSpec
	upstream    Handoffs
	cycleInputs CycleInputs
	errorCtx    *ErrorContext
	correction  *CorrectionState
}

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

func cloneErrorContext(e *ErrorContext) *ErrorContext {
	if e == nil {
		return nil
	}
	c := *e
	return &c
}

func cloneCorrectionState(c *CorrectionState) *CorrectionState {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

func (in PhaseInput) Env(key string) (string, bool) {
	v, ok := in.env[key]
	return v, ok
}

func (in PhaseInput) EnvCopy() map[string]string {
	out := cloneStringMap(in.env)
	if out == nil {
		out = map[string]string{}
	}
	return out
}

func (in PhaseInput) Spec() *phasespec.PhaseSpec { return in.spec }

func (in PhaseInput) Upstream() Handoffs { return in.upstream }

func (in PhaseInput) CycleInputs() CycleInputs { return in.cycleInputs }

func (in PhaseInput) Active() bool { return in.Phase != "" }

func (in PhaseInput) ErrorContext() (ErrorContext, bool) {
	if in.errorCtx == nil {
		return ErrorContext{}, false
	}
	return *in.errorCtx, true
}

func (in PhaseInput) Correction() (CorrectionState, bool) {
	if in.correction == nil {
		return CorrectionState{}, false
	}
	return *in.correction, true
}
