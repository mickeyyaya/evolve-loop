package ciwatch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
