package audit

import (
	"cmp"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditcascade"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func recordCascadeShadow(req core.PhaseRequest, verdict string) []core.Diagnostic {
	return recordCascadeShadowWith(req, verdict, changedpkgs.ChangedFilesChecked)
}

func recordCascadeShadowWith(req core.PhaseRequest, verdict string, files func(string, string) ([]changedpkgs.ChangedFile, bool)) []core.Diagnostic {
	if req.Workspace == "" {
		return nil
	}
	cfg := policy.AuditCascadeConfigFor(req.ProjectRoot)
	if cfg.Stage != policy.AuditCascadeShadow {
		return nil
	}
	changed, known := files(cmp.Or(req.Worktree, req.ProjectRoot), "HEAD")
	paths := make([]string, len(changed))
	for i, f := range changed {
		paths[i] = f.Path
	}
	decision := auditcascade.Decide(auditcascade.Inputs{Changed: paths, Verdict: verdict, Known: known}, cfg.Rule)
	rec := auditcascade.Record{Stage: cfg.Stage, Cycle: req.Cycle, Verdict: verdict, ChangedFiles: len(changed), Decision: decision}
	if err := atomicwrite.JSON(filepath.Join(req.Workspace, auditcascade.RecordFile), rec); err != nil {
		return []core.Diagnostic{{Severity: "warning", Message: "audit cascade shadow: write " + auditcascade.RecordFile + ": " + err.Error() + " — the soak loses this cycle's record; the verdict stands"}}
	}
	return nil
}
