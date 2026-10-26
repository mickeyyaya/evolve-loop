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

type credentialDenies struct {
	read, write, literals []string
}

func (deps Deps) dispatchCredentialDeny(stage string) credentialDenies {
	if stage == envStageOff {
		return credentialDenies{}
	}
	s, err := deps.dispatchStore()()
	if err != nil {
		deps.warnDispatchCredential(err, "the cell gets no deny for it")
		return credentialDenies{}
	}
	if err := s.Ensure(stage); err != nil {
		deps.warnDispatchCredential(err, "the cell still gets the deny")
	}
	dir := s.Dir
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	parent := filepath.Dir(dir)
	return credentialDenies{
		read:     []string{filepath.Join(dir, filepath.Base(s.HeldDir()))},
		write:    []string{parent},
		literals: ancestorsOf(parent),
	}
}

func ancestorsOf(path string) []string {
	var out []string
	for dir := filepath.Dir(path); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		out = append(out, dir)
	}
	return out
}

func (deps Deps) warnDispatchCredential(err error, outcome string) {
	if deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN dispatch credential: %v (%s)\n", err, outcome)
	}
}

func (deps Deps) warnCredentialCoversCell(denies []string, req SandboxWrapRequest) {
	for _, cell := range []string{req.Worktree, req.Workspace} {
		if cell == "" {
			continue
		}
		real, err := canonicalSandboxPath(cell)
		if err != nil {
			continue
		}
		for _, deny := range denies {
			if withinDir(deny, real) {
				deps.reportCredentialCoversCell(deny, real)
			}
		}
	}
}

func (deps Deps) reportCredentialCoversCell(deny, cell string) {
	err := fmt.Errorf("the write deny %s covers the cell path %s", deny, cell)
	deps.warnDispatchCredential(err, "the cell keeps the deny and cannot write there")
	deps.Signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleIsolation, Origin: "bridge.warnCredentialCoversCell",
		Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: isolation.ViolationCode(3),
		Reason: err.Error(),
		Fields: map[string]string{
			"invariant": "I3", "action": "logged", "deny": deny, "cell": cell,
			"rule": "launch.credential.covers-cell", "source": "dispatch-credential",
		},
	})
}
