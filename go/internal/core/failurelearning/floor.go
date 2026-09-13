package failurelearning

import (
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// WriteFloor is the failure floor (inbox retro-always-invariant, gap 1 /
// cycle-243): when the LLM retro cannot run or returns a non-canonical
// verdict, render the learning artifacts deterministically —
// retrospective-report.md in the cycle workspace, the failure-lesson YAML and
// the inbox remediation items — so the lesson survives instead of degrading to
// a stderr line. Best-effort: a write failure is one WARN and never masks the
// original phase failure; the recurrence closure runs LAST, unconditionally.
func (e *Engine) WriteFloor(f Failure, l Learned) {
	ev := floorEvent(f, l, e.now().UTC())
	evolveDir := paths.EvolveDirOf(f.ProjectRoot)
	lessonsDir := filepath.Join(evolveDir, "instincts", "lessons")
	// The near-duplicate bound and the remediation weight are operator config,
	// not Go literals: resolved here from ONE policy read (the composition
	// point that holds the project root) and passed as Options so faillearn
	// stays a policy-free leaf. A load failure resolves to the compiled
	// defaults, never to a disarmed or suppress-everything gate.
	pol := e.loadPolicy(f)
	opts := []faillearn.Option{faillearn.WithNoveltyThreshold(pol.ResearchConfig().NoveltyThreshold)}
	// F1(ii): only SELF-REPORTED structured defects reach the queue — the
	// synthesized summary echo is a restatement of the failure, not an item.
	// The filter is faillearn.StructuredDefects, the one rule the lesson writer
	// already applies (a `structured != nil` proxy filed classed-but-defectless
	// blocks as priority-H bugs).
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

// floorEvent projects the failure and what was learned onto the lesson event:
// the supervisor's defaults, then — ADR-0039 §7 — the failed phase's own
// structured block (validated and capped once by the recorder) for the class,
// the defects and the evidence, with the workspace appended LAST.
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

// loadPolicy is the ONE read of <root>/.evolve/policy.json for a floor:
// absence is silent (policy.Load returns the zero policy), a read or parse
// error is one CodePolicyLoadFailed and the zero policy — the compiled defaults.
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
