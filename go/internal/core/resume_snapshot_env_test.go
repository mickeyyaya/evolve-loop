package core

import "testing"

func TestSnapshotResumeInputs_CopiesTheRequestEnvAndAddsNoResumeKey(t *testing.T) {
	t.Parallel()
	req := CycleRequest{Env: map[string]string{"EVOLVE_PROJECT_ROOT": "/repo"}}
	boot := &resumeBootstrap{request: req, cycle: 7, cycleState: CycleState{PreCycleHEAD: "abc"}}

	got := (&Orchestrator{}).snapshotResumeInputs(boot)

	if len(got.env) != 1 || got.env["EVOLVE_PROJECT_ROOT"] != "/repo" {
		t.Fatalf("env = %v, want only the copied request env: the dead resume key is gone (X18e)", got.env)
	}
	got.env["EVOLVE_PROJECT_ROOT"] = "/changed"
	if req.Env["EVOLVE_PROJECT_ROOT"] != "/repo" {
		t.Fatal("the snapshot must be a copy, not the request map")
	}
}
