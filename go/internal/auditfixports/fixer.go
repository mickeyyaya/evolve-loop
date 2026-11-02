package auditfixports

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

type Fixer struct {
	Run         AgentRun
	Route       Route
	ProjectRoot string
}

var _ auditfix.Auditor = Fixer{}

func (f Fixer) Seat() (auditfix.Seat, error) {
	return freshSeat(f.Route, FixerProfile)
}

func (f Fixer) ApplyFix(ctx context.Context, req auditfix.FixRequest) (map[string]string, error) {
	args := []string{FixerProfile, "--json", "--project-root", f.ProjectRoot, "--worktree", req.Worktree, "--file", filepath.Join(req.Workspace, auditReport)}
	for _, path := range req.Cited {
		args = append(args, "--writable", path)
	}
	if _, err := f.Run(ctx, args, fixBrief(req)); err != nil {
		return nil, fmt.Errorf("auditfixports: fixer: %w", err)
	}
	return reasonsByPath(req.Prescriptions), nil
}

func reasonsByPath(prescriptions []auditfix.Prescription) map[string]string {
	out := map[string]string{}
	for _, p := range prescriptions {
		for _, path := range retryladder.CitedPaths(p.Defect, p.Text) {
			if _, seen := out[path]; !seen {
				out[path] = p.Text
			}
		}
	}
	return out
}

func fixBrief(req auditfix.FixRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "The audit of cycle %d failed, and each finding has a prescription. Apply each prescription below in the worktree. Change nothing else, and write no test that a prescription does not ask for.\n\n", req.Cycle)
	fmt.Fprintf(&b, "Bounds: at most %d files, %d changed lines and %d lines of production code.", req.Bounds.MaxFiles, req.Bounds.MaxLines, req.Bounds.MaxCodeLines)
	if req.Bounds.NoNewProductionLogic {
		b.WriteString(" Add no new production logic.")
	}
	b.WriteString(" You can change only the writable files. The attached audit report has the full findings.\n\n## Prescriptions\n")
	for _, p := range req.Prescriptions {
		fmt.Fprintf(&b, "- %s\n  Prescription: %s\n", cmp.Or(p.Defect, p.Finding), p.Text)
	}
	return b.String()
}
