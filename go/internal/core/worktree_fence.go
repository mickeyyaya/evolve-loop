package core

import (
	"encoding/json"
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

type phaseRequestFields PhaseRequest

type legacyFenceKeys struct {
	ReadOnly bool     `json:"worktree_read_only,omitempty"`
	Writable []string `json:"worktree_writable_paths,omitempty"`
}

type phaseRequestWire struct {
	phaseRequestFields
	legacyFenceKeys
}

func (r PhaseRequest) MarshalJSON() ([]byte, error) {
	wire := phaseRequestWire{phaseRequestFields: phaseRequestFields(r)}
	if r.Fence != nil {
		wire.legacyFenceKeys = legacyFenceKeys{ReadOnly: true, Writable: r.Fence.Writable}
	}
	return json.Marshal(wire)
}

func (r *PhaseRequest) UnmarshalJSON(raw []byte) error {
	var wire phaseRequestWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	*r = PhaseRequest(wire.phaseRequestFields)
	if r.Fence == nil && (wire.ReadOnly || len(wire.Writable) > 0) {
		r.Fence = &WorktreeFence{Writable: wire.Writable}
	}
	return nil
}
