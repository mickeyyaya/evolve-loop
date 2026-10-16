package failurelearning

import (
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func (e *Engine) WriteFloor(f Failure, l Learned) {
	ev := floorEvent(f, l, e.now().UTC())
	evolveDir := paths.EvolveDirOf(f.ProjectRoot)
	lessonsDir := filepath.Join(evolveDir, "instincts", "lessons")
	pol := e.loadPolicy(f)
	opts := []faillearn.Option{faillearn.WithNoveltyThreshold(pol.ResearchConfig().NoveltyThreshold)}
	if defects := faillearn.StructuredDefects(ev); len(defects) > 0 {
		if items := e.remediationItems(f, defects, pol.RetroAutofileDefaultWeight()); len(items) > 0 {
			opts = append(opts, faillearn.WithInbox(filepath.Join(evolveDir, "inbox"), items))
		}
	}
	if err := faillearn.WriteArtifacts(ev, f.Workspace, lessonsDir, opts...); err != nil {
		e.warn(f, CodeFloorWriteFailed, "deterministic fallback write: "+err.Error(),
			map[string]string{"step": "floor", "lessons_dir": lessonsDir, "workspace": f.Workspace})
	}
	e.recurrenceClosure(f, ev.Classification)
}

func floorEvent(f Failure, l Learned, now time.Time) faillearn.FailureEvent {
	ev := faillearn.FailureEvent{
		Cycle:          f.Cycle,
		FailedPhase:    string(f.Phase),
		Scope:          faillearn.ScopePhase,
		Classification: cyclestate.ClassificationMidExecutionFail,
		Verdict:        cyclestate.VerdictFAIL,
		Summary:        l.Summary,
		Defects:        []string{l.Summary},
		EvidencePaths:  []string{f.Workspace},
		Now:            now,
	}
	if l.Structured == nil {
		return ev
	}
	ev.Classification = l.Structured.Class
	if len(l.Structured.Defects) > 0 {
		ev.Defects = l.Structured.Defects
	}
	if len(l.Structured.EvidencePaths) > 0 {
		ev.EvidencePaths = append(l.Structured.EvidencePaths, f.Workspace)
	}
	return ev
}

func (e *Engine) loadPolicy(f Failure) policy.Policy {
	path := filepath.Join(paths.EvolveDirOf(f.ProjectRoot), "policy.json")
	pol, err := policy.Load(path)
	if err != nil {
		e.warn(f, CodePolicyLoadFailed, "policy load failed (using compiled defaults): "+err.Error(),
			map[string]string{"step": "floor", "path": path})
		return policy.Policy{}
	}
	return pol
}
