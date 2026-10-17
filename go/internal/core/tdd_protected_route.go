package core

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acspathscan"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func WithProtectedSurface(member func(string) bool) Option {
	return func(o *Orchestrator) { o.protectedSurface = member }
}

func (cr *cycleRun) routeProtectedPredicates() bool {
	if cr.o.protectedSurface == nil || cr.cs.ActiveWorktree == "" || cr.lastVerdict == VerdictFAIL {
		return false
	}
	committed, decided := committedset.Committed(cr.cs.WorkspacePath)
	if !decided || len(committed) != 1 {
		return false
	}
	dir := filepath.Join(cr.cs.ActiveWorktree, "go", "acs", fmt.Sprintf("cycle%d", cr.cycle))
	needs, err := acspathscan.Needs(dir, cr.o.protectedSurface)
	if err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle %d: predicate scan fault (the paths found are still judged): %v\n", cr.cycle, err)
	}
	if len(needs) == 0 {
		return false
	}
	if err := cr.recordProtectedRoute(dir, committed[0]); err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle %d: protected-predicate route not recorded (the cycle continues): %v\n", cr.cycle, err)
		return false
	}
	cr.protectedRouteDiags = []Diagnostic{{
		Severity: cyclestate.SeverityError,
		Code:     cyclestate.DiagCodeProtectedPredicateRoute,
		Subject:  committed[0],
		Message:  ProtectedRouteMessage(needs),
	}}
	return true
}

func (cr *cycleRun) recordProtectedRoute(predicateDir, itemID string) error {
	if err := acspathscan.Snapshot(predicateDir, filepath.Join(cr.cs.WorkspacePath, committedset.ProtectedPredicatesDir)); err != nil {
		return err
	}
	return committedset.WriteProtectedRoute(cr.cs.WorkspacePath, committedset.ProtectedRoute{Cycle: cr.cycle, ItemID: itemID})
}

func ProtectedRouteMessage(needs []acspathscan.Need) string {
	named := make([]string, 0, len(needs))
	for _, n := range needs {
		named = append(named, fmt.Sprintf("%s (read by %s in %s)", n.Path, n.Test, n.File))
	}
	return "tdd wrote a predicate that reads protected surface " + strings.Join(named, ", ") + " — a lane cannot edit it, so the item is console work"
}
