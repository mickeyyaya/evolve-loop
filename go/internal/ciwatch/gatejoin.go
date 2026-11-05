package ciwatch

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

const stepFailure = "failure"

type ghJobsDoc struct {
	Jobs []struct {
		DatabaseID int64  `json:"databaseId"`
		Name       string `json:"name"`
		Steps      []struct {
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
		} `json:"steps"`
	} `json:"jobs"`
}

func (s ghClassifySource) withGateRows(ctx context.Context, st RunStatus) RunStatus {
	args := []string{"run", "view", strconv.FormatInt(st.RunID, 10)}
	if st.Attempt > 0 {
		args = append(args, "--attempt", strconv.Itoa(st.Attempt))
	}
	var doc ghJobsDoc
	if err := s.ghJSON(ctx, &doc, append(args, "--json", "jobs")...); err != nil {
		st.Warnings = append(st.Warnings, fmt.Sprintf("run %d: the job API was not read, so gate failures keep the text rows: %v", st.RunID, err))
		return st
	}
	attempt := cmp.Or(st.Attempt, 1)
	gated := map[string]FailingTest{}
	for _, job := range doc.Jobs {
		for _, step := range job.Steps {
			gate, ok := ciparity.LookupGateStep(job.Name, step.Name)
			if ok && step.Conclusion == stepFailure {
				gated[job.Name] = gateRow(gate, job.DatabaseID, attempt)
				break
			}
		}
	}
	st.FailingTests = replaceJobRows(st.FailingTests, gated)
	return st
}

func gateRow(gate ciparity.GateStep, jobID int64, attempt int) FailingTest {
	return FailingTest{
		Step: gate.Gate, JobID: jobID, Attempt: attempt,
		Detail: fmt.Sprintf("step %q failed; the message was not read; next: gh run view --job %d --attempt %d --log-failed | tail -n %d (the end of the job log)",
			gate.Step, jobID, attempt, gate.Bound),
	}
}

func replaceJobRows(rows []FailingTest, gated map[string]FailingTest) []FailingTest {
	if len(gated) == 0 {
		return rows
	}
	p := &failureParse{found: map[string]*FailingTest{}, named: map[string]bool{}}
	for _, row := range rows {
		row.Jobs = slices.DeleteFunc(slices.Clone(row.Jobs), func(job string) bool { _, hit := gated[job]; return hit })
		if len(row.Jobs) > 0 {
			p.found[failureKey(row)] = &row
		}
	}
	for job, row := range gated {
		p.add(row, job)
	}
	return sortedFailures(p.found)
}
