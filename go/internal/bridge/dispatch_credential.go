package bridge

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var defaultDispatchStore = dispatchcred.Default

var ErrNoDispatchCredential = errors.New("the LLM launch is refused: this process does not hold the pipeline dispatch credential, and no agent cell holds it")

func (deps Deps) dispatchStore() func() (dispatchcred.Store, error) {
	if deps.DispatchStore != nil {
		return deps.DispatchStore
	}
	return defaultDispatchStore
}

func (deps Deps) requireDispatchCredential(purpose LaunchPurpose, target string) error {
	v := dispatchcred.Admit(deps.dispatchStore())
	if !v.Violation() {
		return nil
	}
	action := "logged"
	if v.Refused() {
		action = "refused"
	}
	deps.Signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleIsolation, Origin: "bridge.requireDispatchCredential",
		Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: isolation.ViolationCode(3),
		Reason: fmt.Sprintf("a %s launch of %s ran without the dispatch credential: %v", purpose, target, v.Cause),
		Fields: map[string]string{
			"invariant": "I3", "stage": v.Stage, "action": action, "target": target, "purpose": string(purpose),
			"rule": "launch.credential", "source": "dispatch-credential",
		},
	})
	if !v.Refused() {
		return nil
	}
	return fmt.Errorf("%w (%s %s: %v)", ErrNoDispatchCredential, purpose, target, v.Cause)
}

func (deps Deps) dispatchCredentialDeny(stage string) (read, write []string) {
	if stage == envStageOff {
		return nil, nil
	}
	s, err := deps.dispatchStore()()
	if err != nil {
		deps.warnDispatchCredential(err, "the cell gets no deny for it")
		return nil, nil
	}
	if err := s.Ensure(stage); err != nil {
		deps.warnDispatchCredential(err, "the cell still gets the deny")
	}
	dir := s.Dir
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	return []string{filepath.Join(dir, filepath.Base(s.HeldDir()))}, []string{dir}
}

func (deps Deps) warnDispatchCredential(err error, outcome string) {
	if deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN dispatch credential: %v (%s)\n", err, outcome)
	}
}
