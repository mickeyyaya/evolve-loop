package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type BuildFloorCheck struct {
	Name string
	Run  BuildFloorCheckFn
}

type BuildHandoffFloor []BuildFloorCheck

const (
	explanationFloorCheckName = "explanation-documentation"
	composedFloorCheckName    = "composed-floor"
)

const (
	floorReasonHead       = "build handoff floor: "
	floorReasonCount      = " deterministic check failure(s)"
	floorFailureSeparator = "\n  "
)

func MandatoryBuildHandoffFloor() BuildHandoffFloor {
	return BuildHandoffFloor{{Name: explanationFloorCheckName, Run: explanationDocumentationFailures}}
}

func WholeBuildHandoffFloor(composed BuildHandoffFloor) BuildHandoffFloor {
	return append(MandatoryBuildHandoffFloor(), composed...)
}

func (f BuildHandoffFloor) Failures(ctx context.Context, in ReviewInput) []string {
	var out []string
	for _, check := range f {
		out = append(out, check.Run(ctx, in)...)
	}
	return out
}

func (f BuildHandoffFloor) Names() []string {
	names := make([]string, 0, len(f))
	for _, check := range f {
		names = append(names, check.Name)
	}
	return names
}

func (f BuildHandoffFloor) Review(ctx context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) {
		return ReviewResult{Approve: true}
	}
	return floorRejection(f.Failures(ctx, in), in.Protected)
}

func floorRejection(found []string, protected func(string) bool) ReviewResult {
	failures := withProtectedFileNotes(found, protected)
	if len(failures) == 0 {
		return ReviewResult{Approve: true}
	}
	reason := fmt.Sprintf(floorReasonHead+"%d"+floorReasonCount+" — fix these exactly before handoff:"+floorFailureSeparator+"%s",
		len(failures), strings.Join(failures, floorFailureSeparator))
	fmt.Fprintf(os.Stderr, "[build-floor] REJECT: %s\n", reason)
	return ReviewResult{Approve: false, Retry: true, Reason: reason, Failures: failures}
}

func (o *Orchestrator) BuildHandoffFloorNames() []string {
	return floorCheckNames(o.reviewer)
}

func floorCheckNames(r DeliverableReviewer) []string {
	return floorChecks(r).Names()
}

func floorChecks(r DeliverableReviewer) BuildHandoffFloor {
	switch v := r.(type) {
	case mandatoryExplanationReviewer:
		return append(append(BuildHandoffFloor{}, v.floor...), floorChecks(v.next)...)
	case chainReviewer:
		var checks BuildHandoffFloor
		for _, member := range v {
			checks = append(checks, floorChecks(member)...)
		}
		return checks
	case BuildHandoffFloor:
		return v
	default:
		return nil
	}
}

func ReviewInputFor(cs CycleState, phase Phase, projectRoot string) ReviewInput {
	return ReviewInput{
		Cycle:                           cs.CycleID,
		RunID:                           cs.RunID,
		ExplanationDocumentationVersion: cs.ExplanationDocumentationVersion,
		Phase:                           string(phase),
		WorktreeBaseSHA:                 cs.WorktreeBaseSHA,
		Workspace:                       cs.WorkspacePath,
		Worktree:                        cs.ActiveWorktree,
		ProjectRoot:                     projectRoot,
	}
}

func ReadRunCycleState(workspace, evolveDir string) (CycleState, error) {
	path := ResolveCycleStatePath(evolveDir)
	if workspace != "" {
		path = filepath.Join(workspace, RunStateFile)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return CycleState{}, fmt.Errorf("%s unreadable (%w)", path, err)
	}
	var state CycleState
	if err := json.Unmarshal(raw, &state); err != nil {
		return CycleState{}, fmt.Errorf("%s unparseable (%w)", path, err)
	}
	return state, nil
}

type BuildHandoffProbe struct {
	Workspace   string
	EvolveDir   string
	ProjectRoot string
	Worktree    string
}

func (p BuildHandoffProbe) Input() (ReviewInput, error) {
	cs, err := ReadRunCycleState(p.Workspace, p.EvolveDir)
	if err == nil {
		err = p.requireBoundWorktree(cs.ActiveWorktree)
	}
	if err != nil {
		return ReviewInput{Phase: string(PhaseBuild), Workspace: p.Workspace, Worktree: p.Worktree, ProjectRoot: p.ProjectRoot}, err
	}
	return ReviewInputFor(cs, PhaseBuild, p.ProjectRoot), nil
}

func (p BuildHandoffProbe) requireBoundWorktree(bound string) error {
	if p.Worktree == "" || sameDirectory(p.Worktree, bound) {
		return nil
	}
	return fmt.Errorf("the cycle state binds worktree %q, not %q", bound, p.Worktree)
}
