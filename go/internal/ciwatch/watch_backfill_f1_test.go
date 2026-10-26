package ciwatch

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func greenFetchF1(context.Context, string) (RunStatus, error) {
	return RunStatus{Status: StatusCompleted, Conclusion: ConclusionSuccess, RunURL: "https://ci/green"}, nil
}

func redFetchF1(context.Context, string) (RunStatus, error) {
	return RunStatus{Status: StatusCompleted, Conclusion: "failure", RunURL: "https://ci/red", FailingTest: "TestX"}, nil
}

func TestWatchBackfillF1_RefusesIncompleteOptionsBeforeFetching(t *testing.T) {
	fetched := 0
	counting := func(ctx context.Context, sha string) (RunStatus, error) {
		fetched++
		return greenFetchF1(ctx, sha)
	}
	cases := []struct {
		name string
		mut  func(o *Options)
		want string
	}{
		{"empty SHA", func(o *Options) { o.SHA = "" }, "SHA required"},
		{"whitespace SHA", func(o *Options) { o.SHA = " \t\n" }, "SHA required"},
		{"no fetcher", func(o *Options) { o.Fetch = nil }, "Fetch seam required"},
		{"no inbox dir while escalation is on", func(o *Options) { o.InboxDir = "  " }, "InboxDir required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, _, _ := watchOpts(t, counting)
			tc.mut(&opts)
			rec, err := Watch(context.Background(), opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if rec.SHA != "" || rec.Conclusion != "" {
				t.Fatalf("a refused watch must return a zero record, got %+v", rec)
			}
		})
	}
	if fetched != 0 {
		t.Fatalf("the fetcher ran %d times for refused options", fetched)
	}
}

func TestWatchBackfillF1_AMissingInboxDirIsFineWhenEscalationIsOff(t *testing.T) {
	opts, _, _ := watchOpts(t, redFetchF1)
	opts.InboxDir = ""
	opts.NoEscalation = true
	rec, err := Watch(context.Background(), opts)
	if err != nil || rec.Conclusion != "failure" || rec.FailingTest != "TestX" {
		t.Fatalf("Watch = %+v, %v; want the red verdict without an inbox", rec, err)
	}
}

func TestWatchBackfillF1_AVerdictThatCannotBeRecordedStopsBeforeEscalating(t *testing.T) {
	opts, inbox, _ := watchOpts(t, redFetchF1)
	opts.WorkspaceDir = filepath.Join(t.TempDir(), "no", "such", "workspace")
	rec, err := Watch(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "ciwatch: write verdict:") {
		t.Fatalf("err = %v, want the verdict write failure", err)
	}
	if rec.Conclusion != "failure" {
		t.Fatalf("the observed verdict is still returned with the error: %+v", rec)
	}
	if got := inboxFiles(t, inbox); len(got) != 0 {
		t.Fatalf("no escalation may be filed when the verdict write failed, got %v", got)
	}
}

func TestWatchBackfillF1_AnEscalationThatCannotBeFiledReturnsTheRecordAndTheError(t *testing.T) {
	opts, _, _ := watchOpts(t, redFetchF1)
	blocker := filepath.Join(t.TempDir(), "inbox-is-a-file")
	writeFileF1(t, blocker)
	opts.InboxDir = blocker
	rec, err := Watch(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "ciwatch: file escalation:") {
		t.Fatalf("err = %v, want the inbox failure", err)
	}
	if rec.Conclusion != "failure" || rec.RunURL != "https://ci/red" {
		t.Fatalf("record = %+v, want the observed red verdict", rec)
	}
}

func TestResolveWatchDefaultsBackfillF1(t *testing.T) {
	pinned := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	slept := time.Duration(0)
	cases := []struct {
		name        string
		in          Options
		wantTimeout time.Duration
		wantPoll    time.Duration
	}{
		{"zero options take every default", Options{}, 900 * time.Second, 30 * time.Second},
		{"negative durations are not honoured", Options{Timeout: -time.Second, Poll: -time.Minute}, 900 * time.Second, 30 * time.Second},
		{"explicit values survive", Options{Timeout: 5 * time.Minute, Poll: 7 * time.Second}, 5 * time.Minute, 7 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveWatchDefaults(tc.in)
			if got.Timeout != tc.wantTimeout || got.Poll != tc.wantPoll {
				t.Fatalf("timeout/poll = %v/%v, want %v/%v", got.Timeout, got.Poll, tc.wantTimeout, tc.wantPoll)
			}
			if got.Now == nil || got.Sleep == nil {
				t.Fatalf("clock seams must always be filled: %+v", got)
			}
		})
	}
	t.Run("default clock is the wall clock", func(t *testing.T) {
		before := time.Now()
		got := resolveWatchDefaults(Options{}).Now()
		if got.Before(before) || got.After(time.Now()) {
			t.Fatalf("default Now = %v, want the wall clock", got)
		}
		resolveWatchDefaults(Options{}).Sleep(0)
	})
	t.Run("injected clock seams are kept", func(t *testing.T) {
		got := resolveWatchDefaults(Options{Now: func() time.Time { return pinned }, Sleep: func(d time.Duration) { slept = d }})
		got.Sleep(3 * time.Second)
		if !got.Now().Equal(pinned) || slept != 3*time.Second {
			t.Fatalf("injected seams were replaced: now=%v slept=%v", got.Now(), slept)
		}
	})
}
