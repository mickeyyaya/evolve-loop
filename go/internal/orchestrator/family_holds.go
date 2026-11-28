package orchestrator

import (
	"cmp"
	"slices"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
)

type FamilyHolds struct {
	mu      sync.Mutex
	refused map[string]admission.Decision
}

func NewFamilyHolds() *FamilyHolds {
	return &FamilyHolds{refused: map[string]admission.Decision{}}
}

func (h *FamilyHolds) Record(d admission.Decision) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.refused[d.ID] = d
}

func (h *FamilyHolds) forget(todo string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.refused, todo)
}

func (h *FamilyHolds) Refused(limit int) []admission.Decision {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]admission.Decision, 0, len(h.refused))
	for _, d := range h.refused {
		out = append(out, d)
	}
	slices.SortFunc(out, func(a, b admission.Decision) int {
		return cmp.Or(cmp.Compare(a.Rank, b.Rank), strings.Compare(a.ID, b.ID))
	})
	return out[:min(len(out), limit)]
}

func (h *FamilyHolds) refusedCount() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.refused)
}

func (h *FamilyHolds) Summary() string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	lines := make([]string, 0, len(h.refused))
	for _, d := range h.refused {
		if i := slices.IndexFunc(d.Cells, func(c string) bool { return strings.HasPrefix(c, admission.FamilyCellPrefix) }); d.FamilyHold && i >= 0 {
			lines = append(lines, d.ID+" ("+d.Cells[i]+", behind "+d.BlockedBy+")")
		}
	}
	slices.Sort(lines)
	return strings.Join(lines, "; ")
}
