// Package lanerouting decides which declared paths a fleet lane can never change.
// See docs/architecture/packages/internal-lanerouting.md.
package lanerouting

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Forbidden returns the routing predicate: true for protected surface, or for a path the
// named source-writing profile's enforced sandbox denies, since a lane could never write either.
func Forbidden(projectRoot, writerProfile string) (func(string) bool, error) {
	if projectRoot == "" {
		return nil, errors.New("lanerouting: empty project root")
	}
	prof, err := profiles.NewFromDir(filepath.Join(projectRoot, ".evolve", "profiles")).Get(writerProfile)
	if err != nil {
		return nil, fmt.Errorf("lanerouting: %w", err)
	}
	var sandbox profiles.SandboxConfig
	if prof.Sandbox != nil && prof.Sandbox.Enabled {
		sandbox = *prof.Sandbox
	}
	return anyOf(guards.IsProtectedScope, sandbox.Denies), nil
}

func anyOf(preds ...func(string) bool) func(string) bool {
	return func(path string) bool {
		for _, pred := range preds {
			if pred(path) {
				return true
			}
		}
		return false
	}
}
