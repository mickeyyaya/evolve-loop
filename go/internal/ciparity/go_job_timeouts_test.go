package ciparity

import "testing"

func TestGoWorkflow_EveryJobHasABoundedTimeout(t *testing.T) {
	t.Parallel()
	const maxJobTimeoutMinutes = 90
	for id, job := range readWorkflowContract(t, "go.yml").Jobs {
		if job.TimeoutMinutes < 1 || job.TimeoutMinutes > maxJobTimeoutMinutes {
			t.Errorf("go.yml job %s has timeout-minutes %d; want 1 to %d, because a hung job otherwise keeps the run of main in progress for the runner default of 360 minutes and GitHub cancels each pending run of main behind it", id, job.TimeoutMinutes, maxJobTimeoutMinutes)
		}
	}
}
