package bridge

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
)

var defaultDispatchStore = dispatchcred.Default

func (deps Deps) dispatchCredentialDeny(stage string) (read, write []string) {
	if stage == envStageOff {
		return nil, nil
	}
	open := deps.DispatchStore
	if open == nil {
		open = defaultDispatchStore
	}
	s, err := open()
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
