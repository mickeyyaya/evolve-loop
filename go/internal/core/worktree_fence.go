package core

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

type WorktreeFence struct {
	Writable []string `json:"writable,omitempty"`
}

func FenceFor(writesSource bool, writable []string) *WorktreeFence {
	if writesSource {
		return nil
	}
	return &WorktreeFence{Writable: slices.Clone(writable)}
}

func (f *WorktreeFence) WritablePaths() []string {
	if f == nil {
		return nil
	}
	return f.Writable
}

func PhaseWritesSource(p Phase, cat phasespec.Catalog) bool {
	if WorktreePhase(p) {
		return true
	}
	spec, ok := cat.Get(string(p))
	return ok && spec.WritesSource
}
