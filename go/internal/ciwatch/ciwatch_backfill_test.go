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

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestWatch_NonTimeoutFetchErrorIsWrappedNotATimeout(t *testing.T) {
	cause := errors.New("gh: authentication required")
	calls := 0
	fetch := func(context.Context, string) (RunStatus, error) {
		calls++
		return RunStatus{}, cause
	}
	opts, inbox, _ := watchOpts(t, fetch)

	_, err := Watch(context.Background(), opts)

	if !errors.Is(err, cause) {
		t.Fatalf("err = %v, want it to wrap the fetch cause", err)
	}
	if errors.Is(err, ErrWatchTimeout) {
		t.Errorf("err = %v, must not be classified as a timeout", err)
	}
	if !strings.Contains(err.Error(), "deadbeefcafe0123") {
		t.Errorf("err = %v, want the watched SHA named", err)
	}
	if calls != 1 {
		t.Errorf("fetch calls = %d, want 1 (no retry on a hard error)", calls)
	}
	if names := inboxFiles(t, inbox); len(names) != 0 {
		t.Errorf("inbox files = %v, want none on a fetch error", names)
	}
}

func TestWatch_NeverSleepsPastTheDeadline(t *testing.T) {
	calls := 0
	fetch := func(context.Context, string) (RunStatus, error) {
		calls++
		return RunStatus{Status: "in_progress"}, nil
	}
	clock := &fakeClock{now: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)}
	start := clock.now
	opts, _, _ := watchOpts(t, fetch)
	opts.Now, opts.Sleep = clock.Now, clock.Sleep
	opts.Timeout = 100 * time.Second
	opts.Poll = 30 * time.Second

	_, err := Watch(context.Background(), opts)

	if !errors.Is(err, ErrWatchTimeout) || !strings.Contains(err.Error(), `last status="in_progress"`) {
		t.Fatalf("err = %v, want ErrWatchTimeout with last status in_progress", err)
	}
	if calls != 4 {
		t.Errorf("fetch calls = %d, want 4 (t=0,30,60,90)", calls)
	}
	if slept := clock.now.Sub(start); slept != 90*time.Second {
		t.Errorf("clock advanced %v, want 90s: the last poll must not sleep past the 100s deadline", slept)
	}
}

func TestWatch_CompletesOnASecondPollWithinTheDeadline(t *testing.T) {
	calls := 0
	fetch := func(context.Context, string) (RunStatus, error) {
		calls++
		if calls == 2 {
			return RunStatus{Status: StatusCompleted, Conclusion: ConclusionSuccess}, nil
		}
		return RunStatus{Status: "queued"}, nil
	}
	opts, _, _ := watchOpts(t, fetch)
	opts.Timeout = 60 * time.Second
	opts.Poll = 30 * time.Second

	rec, err := Watch(context.Background(), opts)

	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if rec.Conclusion != ConclusionSuccess || calls != 2 {
		t.Errorf("conclusion=%q calls=%d, want success after 2 fetches", rec.Conclusion, calls)
	}
}

func escalationStatus() RunStatus {
	return RunStatus{Status: StatusCompleted, Conclusion: "failure", RunURL: "https://ci/run/7"}
}

func soleEscalation(t *testing.T, inbox string) map[string]any {
	t.Helper()
	names := inboxFiles(t, inbox)
	if len(names) != 1 {
		t.Fatalf("inbox files = %v, want exactly one", names)
	}
	body, err := os.ReadFile(filepath.Join(inbox, names[0]))
	if err != nil {
		t.Fatalf("read escalation: %v", err)
	}
	var item map[string]any
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatalf("escalation is not JSON: %v", err)
	}
	return item
}

func TestFileEscalation_UnnamedFailingTestPointsAtTheRunURL(t *testing.T) {
	for name, failing := range map[string]string{"empty": "", "whitespace": " \t\n"} {
		t.Run(name, func(t *testing.T) {
			opts, inbox, _ := watchOpts(t, nil)
			st := escalationStatus()
			st.FailingTest = failing

			if err := fileEscalation(opts, st, opts.Now()); err != nil {
				t.Fatalf("fileEscalation: %v", err)
			}

			item := soleEscalation(t, inbox)
			want := "CI failure on pushed commit deadbeefcafe — (failing job/test not identified — open the run URL)"
			if item["title"] != want {
				t.Errorf("title = %q, want %q", item["title"], want)
			}
		})
	}
}

func TestFileEscalation_ShortSHAIsKeptWhole(t *testing.T) {
	for _, sha := range []string{"abc", "123456789012"} {
		t.Run(sha, func(t *testing.T) {
			opts, inbox, _ := watchOpts(t, nil)
			opts.SHA = sha

			if err := fileEscalation(opts, escalationStatus(), opts.Now()); err != nil {
				t.Fatalf("fileEscalation: %v", err)
			}

			if item := soleEscalation(t, inbox); item["id"] != "ci-red-"+sha {
				t.Errorf("id = %q, want ci-red-%s", item["id"], sha)
			}
		})
	}
}

func TestFileEscalation_ThirteenCharSHAIsCutToTwelve(t *testing.T) {
	opts, inbox, _ := watchOpts(t, nil)
	opts.SHA = "0123456789abc"

	if err := fileEscalation(opts, escalationStatus(), opts.Now()); err != nil {
		t.Fatalf("fileEscalation: %v", err)
	}

	item := soleEscalation(t, inbox)
	if item["id"] != "ci-red-0123456789ab" {
		t.Errorf("id = %q, want ci-red-0123456789ab", item["id"])
	}
	if !strings.Contains(item["summary"].(string), "0123456789abc") {
		t.Errorf("summary must carry the full SHA: %q", item["summary"])
	}
}

func TestFileEscalation_ExcerptAtTheBoundIsNotTruncated(t *testing.T) {
	for name, tc := range map[string]struct {
		size      int
		truncated bool
	}{
		"at bound":   {maxLogExcerpt, false},
		"one over":   {maxLogExcerpt + 1, true},
		"one under":  {maxLogExcerpt - 1, false},
		"empty log":  {0, false},
		"far beyond": {maxLogExcerpt * 5, true},
	} {
		t.Run(name, func(t *testing.T) {
			opts, inbox, _ := watchOpts(t, nil)
			st := escalationStatus()
			st.LogExcerpt = strings.Repeat("é", tc.size/2) + strings.Repeat("x", tc.size%2)

			if err := fileEscalation(opts, st, opts.Now()); err != nil {
				t.Fatalf("fileEscalation: %v", err)
			}

			summary := soleEscalation(t, inbox)["summary"].(string)
			if got := strings.Contains(summary, "[... excerpt truncated by ciwatch ...]"); got != tc.truncated {
				t.Errorf("truncated = %v, want %v", got, tc.truncated)
			}
		})
	}
}

func TestFileEscalation_FailingDependenciesReturnWrappedErrors(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		arrange func(t *testing.T, opts *Options)
		want    string
	}{
		{"marshal", func(t *testing.T, _ *Options) {
			orig := marshalIndent
			marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("encode boom") }
			t.Cleanup(func() { marshalIndent = orig })
		}, "ciwatch: marshal escalation: encode boom"},
		{"inbox dir is a file", func(t *testing.T, opts *Options) {
			file := filepath.Join(t.TempDir(), "inbox")
			if err := os.WriteFile(file, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			opts.InboxDir = file
		}, "ciwatch: inbox dir:"},
		{"inbox dir read-only", func(t *testing.T, opts *Options) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			opts.InboxDir = dir
		}, "ciwatch: write escalation:"},
		{"target is a non-empty directory", func(t *testing.T, opts *Options) {
			target := filepath.Join(opts.InboxDir, now.Format(inboxbatch.FilenameStampLayout)+"-ci-red-deadbeefcafe.json")
			if err := os.MkdirAll(filepath.Join(target, "occupied"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "ciwatch: rename escalation:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, _, _ := watchOpts(t, nil)
			tc.arrange(t, &opts)

			err := fileEscalation(opts, escalationStatus(), now)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
