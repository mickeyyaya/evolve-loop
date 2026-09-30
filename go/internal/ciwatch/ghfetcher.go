package ciwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

// execCapture is the subprocess seam for NewGHFetcher; tests replace it with
// an in-process fake so no test ever invokes the live gh CLI.
var execCapture = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.Output()
}

// ghRun mirrors the fields requested from `gh run list --json`.
type ghRun struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
	DatabaseID int64  `json:"databaseId"`
}

// NewGHFetcher returns the production Fetcher: it observes the newest run of
// ciparity.RequiredWorkflow for the pushed SHA via the gh CLI (existing gh
// auth), and on a red completed run pulls a bounded failed-job log excerpt so
// the escalation item can name the failing test. No run visible yet reads as
// status "queued" (the watch keeps polling until the timeout).
func NewGHFetcher(repoRoot string) Fetcher {
	return func(ctx context.Context, sha string) (RunStatus, error) {
		return latestRequiredRun(ctx, repoRoot, "--commit", sha)
	}
}

func LatestRequiredRunOnBranch(ctx context.Context, repoRoot, branch string) (RunStatus, error) {
	return latestRequiredRun(ctx, repoRoot, "--branch", branch)
}

func latestRequiredRun(ctx context.Context, repoRoot, filter, value string) (RunStatus, error) {
	out, err := execCapture(ctx, repoRoot, "gh", "run", "list",
		"--workflow", ciparity.RequiredWorkflow, filter, value, "--limit", "1",
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
	st := RunStatus{Status: r.Status, Conclusion: r.Conclusion, RunURL: r.URL}
	if r.Status == StatusCompleted && r.Conclusion != ConclusionSuccess {
		st.FailingTest, st.LogExcerpt, st.FailingJobs = failedLogSummary(ctx, repoRoot, r.DatabaseID)
	}
	return st, nil
}

// failedLogSummary best-effort extracts the first failing test name plus a
// bounded excerpt from the run's failed-job logs. Any gh error degrades to
// empty values — the escalation item still files, pointing at the run URL.
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
