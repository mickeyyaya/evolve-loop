package orchestrator

import (
	"slices"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
)

type FamilyHolds struct {
	mu   sync.Mutex
	held map[string]string
}

func NewFamilyHolds() *FamilyHolds {
	return &FamilyHolds{held: map[string]string{}}
}

func (h *FamilyHolds) Record(d admission.Decision) {
	if h == nil {
		return
	}
	i := slices.IndexFunc(d.Cells, func(c string) bool { return strings.HasPrefix(c, admission.FamilyCellPrefix) })
	h.mu.Lock()
	defer h.mu.Unlock()
	if i < 0 {
		delete(h.held, d.ID)
		return
	}
	h.held[d.ID] = d.ID + " (" + d.Cells[i] + ", behind " + d.BlockedBy + ")"
}

func (h *FamilyHolds) forget(todo string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.held, todo)
}

func (h *FamilyHolds) Summary() string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	lines := make([]string, 0, len(h.held))
	for _, line := range h.held {
		lines = append(lines, line)
	}
	slices.Sort(lines)
	return strings.Join(lines, "; ")
}
