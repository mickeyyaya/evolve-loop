// Package ciwatch closes the feedback edge from GitHub Actions back into the
// loop (cycle-748, push-ci-watch-remote-parity). The go.yml matrix runs
// ubuntu+macos while every local gate runs macOS-only, so ubuntu-only
// failures are structurally invisible to local gates: main stayed red from
// 2026-07-06 11:50 through 20:16+ across 8 pushes with the loop shipping on
// top. Watch polls the CI run for a pushed SHA until it completes (or times
// out), records the verdict as a workspace artifact the dossier ingests
// (dossier.CIWatchVerdictFile), and on a red conclusion auto-files a critical
// fix-forward inbox item naming the failing job/test with a bounded log
// excerpt. A green run files nothing.
//
// All knobs (enabled, timeout, poll interval) come from the policy.json
// ci_watch block (policy.CIWatchConfig) — zero env flags. Tests inject a fake
// Fetcher; production uses NewGHFetcher (gh CLI, existing gh auth).
package ciwatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// StatusCompleted is the terminal RunStatus.Status value; every other status
// means the run is still queued or in progress and Watch keeps polling.
const StatusCompleted = "completed"

// ConclusionSuccess is the one green RunStatus.Conclusion value; anything
// else on a completed run files the critical fix-forward inbox item.
const ConclusionSuccess = "success"

// maxLogExcerpt bounds the failing-log excerpt carried into the inbox item so
// a huge CI log can never bloat the inbox (the item must stay a pointer plus
// evidence, not a log archive).
const maxLogExcerpt = 4000

// RunStatus is one observation of the CI run for a pushed SHA.
type RunStatus struct {
	// Status is the run lifecycle state ("queued", "in_progress", "completed").
	Status string
	// Conclusion is set once Status is completed ("success", "failure", ...).
	Conclusion string
	// RunURL links the observed workflow run.
	RunURL string
	// FailingTest is the best-effort failing job/test name on a red run.
	FailingTest string
	// LogExcerpt is a bounded excerpt of the failing job's log.
	LogExcerpt string
}

// Fetcher returns the current CI run status for a pushed SHA. Tests inject
// fakes; production uses NewGHFetcher.
type Fetcher func(ctx context.Context, sha string) (RunStatus, error)

// Options drives one Watch invocation.
type Options struct {
	// SHA is the pushed commit to watch. Required.
	SHA string
	// Cycle is the cycle number recorded in the escalation item (0 = unknown).
	Cycle int
	// InboxDir is where a red conclusion files the critical fix-forward inbox
	// item. Required.
	InboxDir string
	// WorkspaceDir, when non-empty, receives the verdict artifact
	// (dossier.CIWatchVerdictFile) for dossier ingestion.
	WorkspaceDir string
	// Fetch observes the run state. Required.
	Fetch Fetcher
	// Timeout and Poll come from the policy.json ci_watch block
	// (policy.CIWatchConfig); zero values fall back to the same compiled
	// defaults (900s / 30s).
	Timeout time.Duration
	Poll    time.Duration
	// Now and Sleep are test seams; nil = real clock.
	Now   func() time.Time
	Sleep func(time.Duration)
}

// ErrWatchTimeout reports that the CI run did not complete within Timeout.
var ErrWatchTimeout = errors.New("ciwatch: timed out waiting for CI run to complete")

// Watch polls opts.Fetch until the run for opts.SHA completes or Timeout
// elapses, then records the verdict. On a non-success conclusion it files a
// critical fix-forward inbox item naming the failing test; a green run files
// nothing. The returned record is what the dossier ingests.
func Watch(ctx context.Context, opts Options) (dossier.CIWatchRecord, error) {
	var rec dossier.CIWatchRecord
	if strings.TrimSpace(opts.SHA) == "" {
		return rec, errors.New("ciwatch: SHA required")
	}
	if opts.Fetch == nil {
		return rec, errors.New("ciwatch: Fetch seam required")
	}
	if strings.TrimSpace(opts.InboxDir) == "" {
		return rec, errors.New("ciwatch: InboxDir required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	sleep := opts.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 900 * time.Second
	}
	poll := opts.Poll
	if poll <= 0 {
		poll = 30 * time.Second
	}

	deadline := now().Add(timeout)
	var st RunStatus
	for {
		var err error
		st, err = opts.Fetch(ctx, opts.SHA)
		if err != nil {
			return rec, fmt.Errorf("ciwatch: fetch run status for %s: %w", opts.SHA, err)
		}
		if st.Status == StatusCompleted {
			break
		}
		if now().Add(poll).After(deadline) {
			return rec, fmt.Errorf("%w: sha=%s last status=%q", ErrWatchTimeout, opts.SHA, st.Status)
		}
		sleep(poll)
	}

	rec = dossier.CIWatchRecord{
		SHA:         opts.SHA,
		Conclusion:  st.Conclusion,
		RunURL:      st.RunURL,
		FailingTest: st.FailingTest,
		CheckedAt:   now().UTC().Format(time.RFC3339),
	}
	if opts.WorkspaceDir != "" {
		if err := writeVerdict(opts.WorkspaceDir, rec); err != nil {
			return rec, err
		}
	}
	if st.Conclusion != ConclusionSuccess {
		if err := fileEscalation(opts, st, now().UTC()); err != nil {
			return rec, err
		}
	}
	return rec, nil
}

// writeVerdict records the CI verdict artifact atomically (tmp + rename) so a
// concurrent dossier build never reads a torn file.
func writeVerdict(dir string, rec dossier.CIWatchRecord) error {
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("ciwatch: marshal verdict: %w", err)
	}
	path := filepath.Join(dir, dossier.CIWatchVerdictFile)
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("ciwatch: write verdict: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("ciwatch: rename verdict: %w", err)
	}
	return nil
}

// escalationItem is the critical fix-forward inbox item filed on a red run.
// The shape mirrors the hand-filed .evolve/inbox items triage already reads.
type escalationItem struct {
	ID        string  `json:"id"`
	CreatedAt string  `json:"created_at"`
	Weight    float64 `json:"weight"`
	Kind      string  `json:"kind"`
	Priority  string  `json:"priority"`
	Title     string  `json:"title"`
	Summary   string  `json:"summary"`
	Evidence  string  `json:"evidence,omitempty"`
	Source    string  `json:"source"`
}

// fileEscalation writes the critical fix-forward inbox item for a red run,
// naming the failing job/test and carrying a bounded log excerpt.
func fileEscalation(opts Options, st RunStatus, now time.Time) error {
	short := opts.SHA
	if len(short) > 12 {
		short = short[:12]
	}
	failing := st.FailingTest
	if strings.TrimSpace(failing) == "" {
		failing = "(failing job/test not identified — open the run URL)"
	}
	excerpt := st.LogExcerpt
	if len(excerpt) > maxLogExcerpt {
		excerpt = excerpt[:maxLogExcerpt] + "\n[... excerpt truncated by ciwatch ...]"
	}
	item := escalationItem{
		ID:        "ci-red-" + short,
		CreatedAt: now.Format(time.RFC3339),
		Weight:    0.95,
		Kind:      "fix",
		Priority:  "critical",
		Title:     fmt.Sprintf("CI %s on pushed commit %s — %s", st.Conclusion, short, failing),
		Summary: fmt.Sprintf(
			"Post-push CI watch (cycle %d): the GitHub run for %s completed with conclusion=%q. Failing: %s. Fix forward before shipping on top.\n\nLog excerpt:\n%s",
			opts.Cycle, opts.SHA, st.Conclusion, failing, excerpt),
		Evidence: st.RunURL,
		Source:   "ciwatch",
	}
	body, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Errorf("ciwatch: marshal escalation: %w", err)
	}
	if err := os.MkdirAll(opts.InboxDir, 0o755); err != nil {
		return fmt.Errorf("ciwatch: inbox dir: %w", err)
	}
	name := fmt.Sprintf("%s-ci-red-%s.json", now.Format("2006-01-02T15-04-05Z"), short)
	path := filepath.Join(opts.InboxDir, name)
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("ciwatch: write escalation: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("ciwatch: rename escalation: %w", err)
	}
	return nil
}
