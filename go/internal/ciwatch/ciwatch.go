// Package ciwatch watches the CI run of a pushed commit and files a fix-forward item when it is red.
// See docs/architecture/packages/internal-ciwatch.md.
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
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const StatusCompleted = "completed"

const ConclusionSuccess = "success"

const maxLogExcerpt = 4000

var marshalIndent = json.MarshalIndent

type RunStatus struct {
	Status       string
	Conclusion   string
	RunURL       string
	FailingTest  string
	LogExcerpt   string
	FailingJobs  []string
	RunID        int64
	HeadSHA      string
	Attempt      int
	CreatedAt    time.Time
	FailingTests []FailingTest
}

type Fetcher func(ctx context.Context, sha string) (RunStatus, error)

type Options struct {
	SHA          string
	Cycle        int
	InboxDir     string
	NoEscalation bool
	WorkspaceDir string
	Fetch        Fetcher
	Timeout      time.Duration
	Poll         time.Duration
	Now          func() time.Time
	Sleep        func(time.Duration)
}

var ErrWatchTimeout = errors.New("ciwatch: timed out waiting for CI run to complete")

func Watch(ctx context.Context, opts Options) (dossier.CIWatchRecord, error) {
	var rec dossier.CIWatchRecord
	if strings.TrimSpace(opts.SHA) == "" {
		return rec, errors.New("ciwatch: SHA required")
	}
	if opts.Fetch == nil {
		return rec, errors.New("ciwatch: Fetch seam required")
	}
	if !opts.NoEscalation && strings.TrimSpace(opts.InboxDir) == "" {
		return rec, errors.New("ciwatch: InboxDir required")
	}
	opts = resolveWatchDefaults(opts)
	st, err := pollUntilComplete(ctx, opts)
	if err != nil {
		return rec, err
	}

	rec = dossier.CIWatchRecord{
		SHA:         opts.SHA,
		Conclusion:  st.Conclusion,
		RunURL:      st.RunURL,
		FailingTest: st.FailingTest,
		CheckedAt:   opts.Now().UTC().Format(time.RFC3339),
	}
	if opts.WorkspaceDir != "" {
		if err := writeVerdict(opts.WorkspaceDir, rec); err != nil {
			return rec, err
		}
	}
	if st.Conclusion != ConclusionSuccess && !opts.NoEscalation {
		if err := fileEscalation(opts, st, opts.Now().UTC()); err != nil {
			return rec, err
		}
	}
	return rec, nil
}

func resolveWatchDefaults(opts Options) Options {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Sleep == nil {
		opts.Sleep = time.Sleep
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 900 * time.Second
	}
	if opts.Poll <= 0 {
		opts.Poll = 30 * time.Second
	}
	return opts
}

func pollUntilComplete(ctx context.Context, opts Options) (RunStatus, error) {
	deadline := opts.Now().Add(opts.Timeout)
	lastStatus := "queued"
	for {
		remaining := deadline.Sub(opts.Now())
		if remaining <= 0 {
			return RunStatus{}, fmt.Errorf("%w: sha=%s last status=%q", ErrWatchTimeout, opts.SHA, lastStatus)
		}
		fetchCtx, cancel := context.WithTimeout(ctx, remaining)
		st, err := opts.Fetch(fetchCtx, opts.SHA)
		timedOut := errors.Is(fetchCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded)
		cancel()
		if err != nil {
			if timedOut {
				return RunStatus{}, fmt.Errorf("%w: sha=%s last status=%q", ErrWatchTimeout, opts.SHA, lastStatus)
			}
			return RunStatus{}, fmt.Errorf("ciwatch: fetch run status for %s: %w", opts.SHA, err)
		}
		lastStatus = st.Status
		if st.Status == StatusCompleted {
			return st, nil
		}
		if opts.Now().Add(opts.Poll).After(deadline) {
			return RunStatus{}, fmt.Errorf("%w: sha=%s last status=%q", ErrWatchTimeout, opts.SHA, lastStatus)
		}
		opts.Sleep(opts.Poll)
	}
}

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

type escalationItem struct {
	ID            string  `json:"id"`
	CreatedAt     string  `json:"created_at"`
	Weight        float64 `json:"weight"`
	Kind          string  `json:"kind"`
	Priority      string  `json:"priority"`
	PriorityClass string  `json:"priority_class"`
	Title         string  `json:"title"`
	Summary       string  `json:"summary"`
	Evidence      string  `json:"evidence,omitempty"`
	Source        string  `json:"source"`
}

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
		ID:            "ci-red-" + short,
		CreatedAt:     now.Format(time.RFC3339),
		Weight:        0.95,
		Kind:          "fix",
		Priority:      "critical",
		PriorityClass: inboxbatch.ClassCorrectness,
		Title:         fmt.Sprintf("CI %s on pushed commit %s — %s", st.Conclusion, short, failing),
		Summary: fmt.Sprintf(
			"Post-push CI watch (cycle %d): the GitHub run for %s completed with conclusion=%q. Failing: %s. Fix forward before shipping on top.\n\nLog excerpt:\n%s",
			opts.Cycle, opts.SHA, st.Conclusion, failing, excerpt),
		Evidence: st.RunURL,
		Source:   "ciwatch",
	}
	body, err := marshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Errorf("ciwatch: marshal escalation: %w", err)
	}
	if err := os.MkdirAll(opts.InboxDir, 0o755); err != nil {
		return fmt.Errorf("ciwatch: inbox dir: %w", err)
	}
	name := fmt.Sprintf("%s-ci-red-%s.json", now.Format(inboxbatch.FilenameStampLayout), short)
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
