package ciwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var execCapture = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := sysexec.Command(ctx, name, args...)
	cmd.Dir = dir
	return cmd.Output()
}

type ghRun struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
	DatabaseID int64  `json:"databaseId"`
}

func NewGHFetcher(repoRoot string) Fetcher {
	return NewGHWorkflowFetcher(repoRoot, ciparity.RequiredWorkflow)
}

func NewGHWorkflowFetcher(repoRoot, workflow string) Fetcher {
	return func(ctx context.Context, sha string) (RunStatus, error) {
		return latestRun(ctx, repoRoot, workflow, "--commit", sha)
	}
}

func LatestRequiredRunOnBranch(ctx context.Context, repoRoot, branch string) (RunStatus, error) {
	return latestRun(ctx, repoRoot, ciparity.RequiredWorkflow, "--branch", branch)
}

func latestRun(ctx context.Context, repoRoot, workflow, filter, value string) (RunStatus, error) {
	out, err := execCapture(ctx, repoRoot, "gh", "run", "list",
		"--workflow", workflow, filter, value, "--limit", "1",
		"--json", "status,conclusion,url,databaseId")
	if err != nil {
		return RunStatus{}, fmt.Errorf("gh run list %s %s: %w", filter, value, err)
	}
	var runs []ghRun
	if err := json.Unmarshal(out, &runs); err != nil {
		return RunStatus{}, fmt.Errorf("gh run list output: %w", err)
	}
	if len(runs) == 0 {
		return RunStatus{Status: "queued"}, nil
	}
	r := runs[0]
	st := RunStatus{Status: r.Status, Conclusion: r.Conclusion, RunURL: r.URL, RunID: r.DatabaseID}
	if r.Status == StatusCompleted && r.Conclusion != ConclusionSuccess {
		st.FailingTest, st.LogExcerpt, st.FailingJobs = failedLogSummary(ctx, repoRoot, r.DatabaseID)
	}
	return st, nil
}

func failedLogSummary(ctx context.Context, repoRoot string, runID int64) (string, string, []string) {
	out, err := execCapture(ctx, repoRoot, "gh", "run", "view",
		fmt.Sprintf("%d", runID), "--log-failed")
	if err != nil {
		return "", "", nil
	}
	excerpt := string(out)
	if len(excerpt) > maxLogExcerpt {
		excerpt = excerpt[len(excerpt)-maxLogExcerpt:]
	}
	var failing string
	for _, line := range strings.Split(string(out), "\n") {
		if i := strings.Index(line, "--- FAIL: "); i >= 0 {
			if fields := strings.Fields(line[i+len("--- FAIL: "):]); len(fields) > 0 {
				failing = fields[0]
				break
			}
		}
	}
	return failing, excerpt, failedJobNames(string(out))
}

func failedJobNames(log string) []string {
	var jobs []string
	seen := map[string]bool{}
	for _, line := range strings.Split(log, "\n") {
		name, _, tabbed := strings.Cut(line, "\t")
		if !tabbed || name == "" || seen[name] {
			continue
		}
		seen[name] = true
		jobs = append(jobs, name)
	}
	return jobs
}
