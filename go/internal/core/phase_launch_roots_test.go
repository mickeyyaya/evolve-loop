package core

import (
	"context"
	"testing"
)

func TestDecisionLaunchPreservesTrustedRoots(t *testing.T) {
	for _, active := range []bool{false, true} {
		for _, role := range []string{"advisor", "judge"} {
			t.Run(role+map[bool]string{false: "/before-worktree", true: "/active-worktree"}[active], func(t *testing.T) {
				in := baseRouteInput()
				in.ProjectRoot = t.TempDir()
				in.Workspace = t.TempDir()
				in.ActiveWorktree = ""
				wantCWD := in.Workspace
				if active {
					in.ActiveWorktree = t.TempDir()
					wantCWD = in.ActiveWorktree
				}
				fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"inspect"}]`}
				if role == "advisor" {
					if _, err := NewPhaseAdvisor(fb).Plan(in); err != nil {
						t.Fatal(err)
					}
				} else {
					fb.stdout = `{"score":0.8,"rationale":"ok"}`
					if got := NewPlanJudge(fb).GradePlan(context.Background(), in, samplePlan()); got.Score != 0.8 {
						t.Fatalf("grade: %+v", got)
					}
				}
				if fb.gotReq.ProjectRoot != in.ProjectRoot || fb.gotReq.Worktree != wantCWD {
					t.Fatalf("launch roots: project=%q cwd=%q; want project=%q cwd=%q", fb.gotReq.ProjectRoot, fb.gotReq.Worktree, in.ProjectRoot, wantCWD)
				}
			})
		}
	}
}
