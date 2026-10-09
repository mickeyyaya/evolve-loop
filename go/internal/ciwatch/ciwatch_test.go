package ciwatch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// fakeClock lets a test drive the poll loop without real sleeping.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time        { return c.now }
func (c *fakeClock) Sleep(d time.Duration) { c.now = c.now.Add(d) }

// watchOpts returns Options wired to temp dirs and the fake clock.
func watchOpts(t *testing.T, fetch Fetcher) (Options, string, string) {
	t.Helper()
	inbox := t.TempDir()
	workspace := t.TempDir()
	clock := &fakeClock{now: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)}
	return Options{
		SHA:          "deadbeefcafe0123",
		Cycle:        748,
		InboxDir:     inbox,
		WorkspaceDir: workspace,
		Fetch:        fetch,
		Timeout:      900 * time.Second,
		Poll:         30 * time.Second,
		Now:          clock.Now,
		Sleep:        clock.Sleep,
	}, inbox, workspace
}

func inboxFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read inbox dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestCIWatch_FailedRunFilesCriticalInboxItem pins AC1 of
// push-ci-watch-remote-parity: a push whose CI run FAILS yields a critical
// fix-forward inbox item naming the failing test (bounded log excerpt),
// through a faked gh-runner seam — no live gh call.
func TestCIWatch_FailedRunFilesCriticalInboxItem(t *testing.T) {
	calls := 0
	fetch := func(_ context.Context, sha string) (RunStatus, error) {
		calls++
		if sha != "deadbeefcafe0123" {
			t.Errorf("fetcher got sha %q", sha)
		}
		if calls == 1 {
			return RunStatus{Status: "in_progress"}, nil
		}
		return RunStatus{
			Status:      StatusCompleted,
			Conclusion:  "failure",
			RunURL:      "https://github.com/mickeyyaya/evolve-loop/actions/runs/42",
			FailingTest: "TestOrchestrator_TriageLeakRecover",
			LogExcerpt:  strings.Repeat("x", 10000), // must be bounded in the item
		}, nil
	}
	opts, inbox, workspace := watchOpts(t, fetch)

	rec, err := Watch(context.Background(), opts)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if calls != 2 {
		t.Errorf("fetch calls = %d, want 2 (in_progress then completed)", calls)
	}
	if rec.Conclusion != "failure" || rec.SHA != "deadbeefcafe0123" {
		t.Errorf("record = %+v, want failure verdict for the watched SHA", rec)
	}

	names := inboxFiles(t, inbox)
	if len(names) != 1 {
		t.Fatalf("inbox files = %v, want exactly one escalation item", names)
	}
	body, err := os.ReadFile(filepath.Join(inbox, names[0]))
	if err != nil {
		t.Fatalf("read escalation: %v", err)
	}
	var item struct {
		Priority string  `json:"priority"`
		Kind     string  `json:"kind"`
		Weight   float64 `json:"weight"`
		Title    string  `json:"title"`
		Summary  string  `json:"summary"`
		Evidence string  `json:"evidence"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatalf("escalation item is not valid JSON: %v\n%s", err, body)
	}
	if item.Priority != "critical" {
		t.Errorf("priority = %q, want critical", item.Priority)
	}
	if !strings.Contains(item.Title, "TestOrchestrator_TriageLeakRecover") {
		t.Errorf("title %q must name the failing test", item.Title)
	}
	if !strings.Contains(item.Summary, "TestOrchestrator_TriageLeakRecover") {
		t.Errorf("summary must name the failing test:\n%s", item.Summary)
	}
	if len(item.Summary) > maxLogExcerpt+1000 {
		t.Errorf("summary len = %d — log excerpt not bounded", len(item.Summary))
	}
	if item.Evidence != "https://github.com/mickeyyaya/evolve-loop/actions/runs/42" {
		t.Errorf("evidence = %q, want the run URL", item.Evidence)
	}

	// The verdict artifact for dossier ingestion is recorded too.
	vb, err := os.ReadFile(filepath.Join(workspace, dossier.CIWatchVerdictFile))
	if err != nil {
		t.Fatalf("verdict artifact missing: %v", err)
	}
	var vrec dossier.CIWatchRecord
	if err := json.Unmarshal(vb, &vrec); err != nil {
		t.Fatalf("verdict artifact not valid JSON: %v", err)
	}
	if vrec.Conclusion != "failure" || vrec.FailingTest != "TestOrchestrator_TriageLeakRecover" {
		t.Errorf("verdict artifact = %+v, want failure + failing test", vrec)
	}
}

// TestCIWatch_GreenRunFilesNoInboxItem pins AC1's negative half (the
// anti-no-op signal): a GREEN CI run files NO inbox item — a stub that
// unconditionally escalates fails here. The verdict artifact is still
// recorded (dossier evidence is unconditional).
func TestCIWatch_GreenRunFilesNoInboxItem(t *testing.T) {
	fetch := func(_ context.Context, _ string) (RunStatus, error) {
		return RunStatus{
			Status:     StatusCompleted,
			Conclusion: ConclusionSuccess,
			RunURL:     "https://github.com/mickeyyaya/evolve-loop/actions/runs/43",
		}, nil
	}
	opts, inbox, workspace := watchOpts(t, fetch)

	rec, err := Watch(context.Background(), opts)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if rec.Conclusion != ConclusionSuccess {
		t.Errorf("record conclusion = %q, want success", rec.Conclusion)
	}
	if names := inboxFiles(t, inbox); len(names) != 0 {
		t.Errorf("inbox files = %v, want none for a green run", names)
	}
	if _, err := os.Stat(filepath.Join(workspace, dossier.CIWatchVerdictFile)); err != nil {
		t.Errorf("verdict artifact should be recorded for a green run too: %v", err)
	}
}

// TestWatch_TimesOutWhenRunNeverCompletes pins the timeout bound: a run stuck
// in_progress past the policy timeout returns ErrWatchTimeout and files
// nothing (no fabricated verdict).
func TestWatch_TimesOutWhenRunNeverCompletes(t *testing.T) {
	fetch := func(_ context.Context, _ string) (RunStatus, error) {
		return RunStatus{Status: "in_progress"}, nil
	}
	opts, inbox, workspace := watchOpts(t, fetch)
	opts.Timeout = 90 * time.Second
	opts.Poll = 30 * time.Second

	_, err := Watch(context.Background(), opts)
	if !errors.Is(err, ErrWatchTimeout) {
		t.Fatalf("err = %v, want ErrWatchTimeout", err)
	}
	if names := inboxFiles(t, inbox); len(names) != 0 {
		t.Errorf("inbox files = %v, want none on timeout", names)
	}
	if _, err := os.Stat(filepath.Join(workspace, dossier.CIWatchVerdictFile)); err == nil {
		t.Errorf("verdict artifact must not be fabricated on timeout")
	}
}

// TestNewGHFetcher_ParsesRunListAndFailedLog exercises the production gh
// fetcher through the execCapture seam (no live gh): run-list JSON parsing,
// queued-when-empty, and failing-test extraction from --log-failed.
func TestNewGHFetcher_ParsesRunListAndFailedLog(t *testing.T) {
	orig := execCapture
	t.Cleanup(func() { execCapture = orig })

	t.Run("no runs yet reads as queued", func(t *testing.T) {
		execCapture = func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
			return []byte(`[]`), nil
		}
		st, err := NewGHFetcher(".")(context.Background(), "abc")
		if err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if st.Status != "queued" {
			t.Errorf("status = %q, want queued", st.Status)
		}
	})

	t.Run("red completed run extracts failing test", func(t *testing.T) {
		execCapture = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
			if args[0] == "run" && args[1] == "list" {
				return []byte(`[{"status":"completed","conclusion":"failure","url":"https://x/runs/7","databaseId":7}]`), nil
			}
			return []byte("ok\n--- FAIL: TestOrchestrator_TriageLeakRecover (0.10s)\n    boom\n"), nil
		}
		st, err := NewGHFetcher(".")(context.Background(), "abc")
		if err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if st.Conclusion != "failure" || st.FailingTest != "TestOrchestrator_TriageLeakRecover" {
			t.Errorf("status = %+v, want failure + extracted failing test", st)
		}
	})
}

func TestNewGHFetcher_ReadsTheRequiredWorkflowRun(t *testing.T) {
	orig := execCapture
	t.Cleanup(func() { execCapture = orig })
	execCapture = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		if slices.Contains(args, "view") {
			return []byte("--- FAIL: TestRequiredOnly (0.01s)\n"), nil
		}
		if i := slices.Index(args, "--workflow"); i >= 0 && i+1 < len(args) && args[i+1] == ciparity.RequiredWorkflow {
			return []byte(`[{"status":"completed","conclusion":"failure","url":"https://ci/required","databaseId":7}]`), nil
		}
		return []byte(`[{"status":"completed","conclusion":"success","url":"https://ci/landing-pages","databaseId":8}]`), nil
	}
	st, err := NewGHFetcher(".")(context.Background(), "abc")
	if err != nil || st.Conclusion != "failure" || st.RunURL != "https://ci/required" || st.FailingTest != "TestRequiredOnly" {
		t.Fatalf("fetch = %+v, %v; want the required workflow's red run, not a newer green run of another workflow", st, err)
	}
}

func TestLatestRequiredRunOnBranch_NamesTheFailingJobsOfTheBranchRun(t *testing.T) {
	orig := execCapture
	t.Cleanup(func() { execCapture = orig })
	var listArgs []string
	execCapture = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		if slices.Contains(args, "view") {
			return []byte("unit (ubuntu-latest)\trun\t--- FAIL: TestA (0.01s)\nunit (ubuntu-latest)\trun\tmore\ncontract-scan\tscan\tboom\nno tab here\n"), nil
		}
		listArgs = args
		return []byte(`[{"status":"completed","conclusion":"failure","url":"https://ci/main","databaseId":9}]`), nil
	}
	st, err := LatestRequiredRunOnBranch(context.Background(), ".", "main")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if i := slices.Index(listArgs, "--branch"); i < 0 || listArgs[i+1] != "main" || !slices.Contains(listArgs, ciparity.RequiredWorkflow) {
		t.Errorf("run list args = %v, want the required workflow filtered to --branch main", listArgs)
	}
	if want := []string{"unit (ubuntu-latest)", "contract-scan"}; !slices.Equal(st.FailingJobs, want) {
		t.Errorf("FailingJobs = %v, want %v", st.FailingJobs, want)
	}
	if st.FailingTest != "TestA" || st.RunURL != "https://ci/main" {
		t.Errorf("status = %+v, want failing test TestA on https://ci/main", st)
	}

	execCapture = func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
		return nil, errors.New("gh: not logged in")
	}
	if _, err := LatestRequiredRunOnBranch(context.Background(), ".", "main"); err == nil || !strings.Contains(err.Error(), "--branch main") {
		t.Errorf("gh failure must propagate naming the branch filter, got %v", err)
	}
}

func TestWatch_TimesOutWhenFetchHangs(t *testing.T) {
	fetch := func(ctx context.Context, _ string) (RunStatus, error) {
		<-ctx.Done()
		return RunStatus{}, ctx.Err()
	}
	opts, inbox, _ := watchOpts(t, fetch)
	opts.Timeout = 50 * time.Millisecond
	opts.Poll = 10 * time.Millisecond

	_, err := Watch(context.Background(), opts)
	if !errors.Is(err, ErrWatchTimeout) {
		t.Fatalf("err = %v, want ErrWatchTimeout", err)
	}
	if !strings.Contains(err.Error(), `last status="queued"`) {
		t.Errorf("err = %v, want last status queued", err)
	}
	if names := inboxFiles(t, inbox); len(names) != 0 {
		t.Errorf("inbox files = %v, want none on timeout", names)
	}
}

func TestWatch_TimesOutWhenFetchHangsAfterInProgress(t *testing.T) {
	calls := 0
	fetch := func(ctx context.Context, _ string) (RunStatus, error) {
		calls++
		if calls == 1 {
			return RunStatus{Status: "in_progress"}, nil
		}
		<-ctx.Done()
		return RunStatus{}, ctx.Err()
	}
	opts, _, _ := watchOpts(t, fetch)
	opts.Timeout = 60 * time.Millisecond
	opts.Poll = 10 * time.Millisecond

	_, err := Watch(context.Background(), opts)
	if !errors.Is(err, ErrWatchTimeout) {
		t.Fatalf("err = %v, want ErrWatchTimeout", err)
	}
	if !strings.Contains(err.Error(), `last status="in_progress"`) {
		t.Errorf("err = %v, want last status in_progress", err)
	}
}
