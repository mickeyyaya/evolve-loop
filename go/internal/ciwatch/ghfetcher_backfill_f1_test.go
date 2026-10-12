package ciwatch

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExecCaptureBackfillF1_RunsARealProcessInTheGivenDirectory(t *testing.T) {
	dir := t.TempDir()
	out, err := execCapture(context.Background(), dir, "pwd")
	if err != nil {
		t.Fatalf("pwd: %v", err)
	}
	got := strings.TrimSpace(string(out))
	if !strings.HasSuffix(got, strings.TrimPrefix(dir, "/private")) && got != dir {
		t.Fatalf("pwd printed %q, want the requested directory %q", got, dir)
	}
	if _, err := execCapture(context.Background(), dir, "sh", "-c", "exit 3"); err == nil {
		t.Fatal("a non-zero exit must be an error")
	}
	if _, err := execCapture(context.Background(), t.TempDir(), "f1-no-such-binary"); err == nil {
		t.Fatal("a missing binary must be an error")
	}
}

func TestLatestRunBackfillF1_ReplyHandling(t *testing.T) {
	t.Run("a reply that is not JSON is a parse error", func(t *testing.T) {
		fakeGH(t, func([]string) ([]byte, error) { return []byte("not json"), nil })
		_, err := latestRun(context.Background(), ".", "ci.yml", "--commit", "abc")
		if err == nil || !strings.Contains(err.Error(), "gh run list output:") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a gh failure names the filter it was asked", func(t *testing.T) {
		boom := errors.New("auth required")
		fakeGH(t, func([]string) ([]byte, error) { return nil, boom })
		_, err := latestRun(context.Background(), ".", "ci.yml", "--branch", "main")
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "gh run list --branch main") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("no run yet reads as queued", func(t *testing.T) {
		fakeGH(t, func([]string) ([]byte, error) { return []byte("[]"), nil })
		st, err := latestRun(context.Background(), ".", "ci.yml", "--commit", "abc")
		if err != nil || st.Status != "queued" || st.RunID != 0 {
			t.Fatalf("status = %+v, err = %v", st, err)
		}
	})
	t.Run("an unfinished run pulls no log", func(t *testing.T) {
		calls := fakeGH(t, func([]string) ([]byte, error) {
			return []byte(`[{"status":"in_progress","conclusion":"","url":"u","databaseId":4}]`), nil
		})
		st, err := latestRun(context.Background(), ".", "ci.yml", "--commit", "abc")
		if err != nil || st.Status != "in_progress" || st.FailingTest != "" || len(*calls) != 1 {
			t.Fatalf("status = %+v, err = %v, calls = %d", st, err, len(*calls))
		}
	})
}

func TestFailedLogSummaryBackfillF1(t *testing.T) {
	t.Run("a failing log fetch degrades to empty values", func(t *testing.T) {
		fakeGH(t, func([]string) ([]byte, error) { return nil, errors.New("log gone") })
		test, excerpt, jobs := failedLogSummary(context.Background(), ".", 9)
		if test != "" || excerpt != "" || jobs != nil {
			t.Fatalf("got (%q, %q, %v), want empty values so the escalation still files", test, excerpt, jobs)
		}
	})
	t.Run("a long log keeps only its tail", func(t *testing.T) {
		head := strings.Repeat("H", 100)
		tail := strings.Repeat("T", maxLogExcerpt)
		fakeGH(t, func([]string) ([]byte, error) { return []byte(head + tail), nil })
		_, excerpt, _ := failedLogSummary(context.Background(), ".", 9)
		if len(excerpt) != maxLogExcerpt || excerpt != tail {
			t.Fatalf("excerpt has %d bytes, want exactly the last %d", len(excerpt), maxLogExcerpt)
		}
	})
	t.Run("a log exactly at the bound is kept whole", func(t *testing.T) {
		body := strings.Repeat("x", maxLogExcerpt)
		fakeGH(t, func([]string) ([]byte, error) { return []byte(body), nil })
		if _, excerpt, _ := failedLogSummary(context.Background(), ".", 9); excerpt != body {
			t.Fatalf("a log at the bound must not be cut, got %d bytes", len(excerpt))
		}
	})
	t.Run("the first FAIL line names the test and the tab-prefixed names are the jobs", func(t *testing.T) {
		log := "lint\tstep\tclean\ntest (ubuntu)\tRun\t--- FAIL: TestFirst (0.1s)\ntest (macos)\tRun\t--- FAIL: TestSecond (0.1s)\nlint\tstep\tdone\n"
		fakeGH(t, func([]string) ([]byte, error) { return []byte(log), nil })
		test, _, jobs := failedLogSummary(context.Background(), ".", 9)
		if test != "TestFirst" || strings.Join(jobs, "|") != "lint|test (ubuntu)|test (macos)" {
			t.Fatalf("got (%q, %v)", test, jobs)
		}
	})
	t.Run("a FAIL marker with no name is skipped for the next one", func(t *testing.T) {
		log := "j\ts\t--- FAIL: \nj\ts\t--- FAIL: TestReal (0s)\n"
		fakeGH(t, func([]string) ([]byte, error) { return []byte(log), nil })
		if test, _, _ := failedLogSummary(context.Background(), ".", 9); test != "TestReal" {
			t.Fatalf("test = %q, want TestReal", test)
		}
	})
}
