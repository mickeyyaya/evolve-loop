// Package inboxmover moves inbox items between lifecycle states and applies each
// cycle's outcome to the inbox; the movers themselves live in the lifecycle leaf.
// See docs/architecture/packages/internal-inboxmover.md.
package inboxmover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var (
	ErrNotFound      = lifecycle.ErrNotFound
	ErrMvFailed      = lifecycle.ErrMvFailed
	ErrBadArgs       = lifecycle.ErrBadArgs
	ErrBadState      = lifecycle.ErrBadState
	ErrConsoleRouted = lifecycle.ErrConsoleRouted
	ErrInvalidItem   = lifecycle.ErrInvalidItem
)

type (
	LedgerAppender = lifecycle.LedgerAppender
	ClaimResult    = lifecycle.ClaimResult
	PromoteOpts    = lifecycle.PromoteOpts
	PromoteResult  = lifecycle.PromoteResult
	RecoverResult  = lifecycle.RecoverResult
)

type Options struct {
	ProjectRoot string
	InboxDir    string
	LedgerPath  string
	Ledger      LedgerAppender
	Stderr      io.Writer
	Now         func() time.Time

	ActiveCycleFn func() (string, error)

	IsLandedFn func(sha string) (bool, error)

	MainHeadFn func() (string, error)

	IsProtectedPath func(path string) bool

	Signals *signalcenter.Center

	PriorityClasses []string

	CurrentGoal string
}

func (o *Options) resolveOpts() {
	if o.InboxDir == "" {
		o.InboxDir = filepath.Join(o.ProjectRoot, ".evolve", "inbox")
	}
	if o.LedgerPath == "" {
		o.LedgerPath = filepath.Join(o.ProjectRoot, ".evolve", "ledger.jsonl")
	}
	if o.Ledger == nil {
		o.Ledger = ledger.New(filepath.Dir(o.LedgerPath))
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Stderr == nil {
		o.Stderr = io.Discard
	}
	if o.ActiveCycleFn == nil {
		o.ActiveCycleFn = func() (string, error) {
			return readActiveCycle(filepath.Join(o.ProjectRoot, ".evolve", "cycle-state.json"))
		}
	}
	if o.IsLandedFn == nil {
		root := o.ProjectRoot
		o.IsLandedFn = func(sha string) (bool, error) {
			return shaLandedOnMain(root, sha)
		}
	}
	if o.MainHeadFn == nil {
		root := o.ProjectRoot
		o.MainHeadFn = func() (string, error) { return originMainHead(root) }
	}
}

func shaLandedOnMain(root, sha string) (bool, error) {
	_, stderr, code, err := gitexec.Default(root).Capture(context.Background(), "merge-base", "--is-ancestor", sha, "main")
	switch {
	case err != nil:
		return true, fmt.Errorf("git merge-base --is-ancestor %s main: %w", sha, err)
	case code == 0:
		return true, nil
	case code == 1:
		return false, nil
	}
	return true, fmt.Errorf("git merge-base --is-ancestor %s main exit=%d: %s", sha, code, strings.TrimSpace(stderr))
}

func readActiveCycle(cycleStatePath string) (string, error) {
	body, err := os.ReadFile(cycleStatePath)
	if err != nil {
		return "", err
	}
	var st struct {
		CycleID json.Number `json:"cycle_id"`
	}
	if err := json.Unmarshal(body, &st); err != nil {
		return "", err
	}
	return string(st.CycleID), nil
}

func (o *Options) logf(prefix, format string, args ...any) {
	fmt.Fprintf(o.Stderr, lifecycle.LegacyPrefix+prefix+format+"\n", args...)
}

var errCycleWorktreeInbox = errors.New("inboxmover: refusing to move items in the tracked inbox copy of a cycle worktree")

func cycleWorktreeRefusal(inboxDir string) error {
	inboxDir = filepath.Clean(inboxDir)
	target, err := os.Readlink(filepath.Join(filepath.Dir(inboxDir), "state.json"))
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.EINVAL):
		return nil
	case err != nil:
		return fmt.Errorf("%w %s: cannot read its state.json link: %w", errCycleWorktreeInbox, inboxDir, err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(inboxDir), target)
	}
	plane := filepath.Dir(filepath.Dir(target))
	return fmt.Errorf("%w %s; the plane root is %s: run the command with EVOLVE_PROJECT_ROOT=%s", errCycleWorktreeInbox, inboxDir, plane, plane)
}

func (o Options) mover() (*lifecycle.Mover, error) {
	o.resolveOpts()
	if err := cycleWorktreeRefusal(o.InboxDir); err != nil {
		return nil, err
	}
	return o.reader(), nil
}

func withMover[T any](opts Options, call func(*lifecycle.Mover) (T, error)) (T, error) {
	m, err := opts.mover()
	if err != nil {
		var zero T
		return zero, err
	}
	return call(m)
}

func (o Options) reader() *lifecycle.Mover {
	o.resolveOpts()
	return lifecycle.New(o.InboxDir, o.Ledger,
		lifecycle.WithStderr(o.Stderr),
		lifecycle.WithNow(o.Now),
		lifecycle.WithActiveCycle(o.ActiveCycleFn),
		lifecycle.WithLanded(o.IsLandedFn),
		lifecycle.WithProtectedPath(o.IsProtectedPath),
		lifecycle.WithRetire(func(itemPath, taskID, reason string) { releaseContinuationOnRetire(o, itemPath, taskID, reason) }),
		lifecycle.WithRunWorkspace(func(cycle int) string {
			return filepath.Join(o.ProjectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
		}),
		lifecycle.WithSignals(func() *signalcenter.Center { return o.Signals }),
		lifecycle.WithMainHead(o.MainHeadFn),
		lifecycle.WithPriorityClasses(o.PriorityClasses),
		lifecycle.WithBinding(func(taskID string) (bool, error) {
			_, bound, err := continuation.ReadRegistryEntry(o.ProjectRoot, taskID)
			return bound, err
		}))
}

func Claim(opts Options, taskID, cycle string) (ClaimResult, error) {
	return withMover(opts, func(m *lifecycle.Mover) (ClaimResult, error) { return m.Claim(taskID, cycle) })
}

func Promote(opts Options, taskID, newState string, p PromoteOpts) (PromoteResult, error) {
	return withMover(opts, func(m *lifecycle.Mover) (PromoteResult, error) { return m.Promote(taskID, newState, p) })
}

func ShouldQuarantine(failureCount, ceiling int, systemLevelFailure bool) bool {
	return lifecycle.ShouldQuarantine(failureCount, ceiling, systemLevelFailure)
}

func ReleaseFromQuarantine(opts Options, taskID string) (PromoteResult, error) {
	return withMover(opts, func(m *lifecycle.Mover) (PromoteResult, error) { return m.ReleaseFromQuarantine(taskID) })
}

func RecoverOrphans(opts Options) (RecoverResult, error) {
	return withMover(opts, func(m *lifecycle.Mover) (RecoverResult, error) { return m.RecoverOrphans() })
}

func ReleaseCycleProcessing(opts Options, cycle int) (RecoverResult, error) {
	return ReleaseCycleProcessingWithReason(opts, cycle, "")
}

func ReleaseCycleProcessingWithReason(opts Options, cycle int, reason string) (RecoverResult, error) {
	return releaseCycleProcessing(opts, cycle, reason, nil)
}

type quarantinePolicy = lifecycle.Policy

func releaseCycleProcessing(opts Options, cycle int, reason string, quar *quarantinePolicy) (RecoverResult, error) {
	return withMover(opts, func(m *lifecycle.Mover) (RecoverResult, error) { return m.Release(cycle, reason, quar) })
}

func ReadFailureCount(opts Options, taskID string) (int, bool) {
	return opts.reader().ReadFailureCount(taskID)
}

func FindFileByTaskID(dir, taskID string) (string, error) {
	return lifecycle.FindFileByTaskID(dir, taskID)
}

func bumpFailureCount(path, reason string) (int, error) {
	return lifecycle.BumpFailureCount(path, reason)
}

func updateItemJSON(path string, mutate func(m map[string]json.RawMessage)) error {
	return lifecycle.UpdateItemJSON(path, mutate)
}

func UpdateItemJSON(path string, mutate func(m map[string]json.RawMessage)) error {
	return lifecycle.UpdateItemJSON(path, mutate)
}

func IsMoverWritten(key string) bool {
	return lifecycle.IsMoverWritten(key)
}

func SupersededInboxIDs(triageDecisionJSON []byte) []string {
	var doc struct {
		Superseded []string `json:"superseded"`
	}
	if err := json.Unmarshal(triageDecisionJSON, &doc); err != nil {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, id := range doc.Superseded {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func ReconcileSuperseded(opts Options, supersededIDs []string, newState string, p PromoteOpts) ([]string, error) {
	m, err := opts.mover()
	if err != nil {
		return nil, err
	}
	var retired []string
	for _, id := range supersededIDs {
		if id == "" {
			continue
		}
		res, err := m.Promote(id, newState, p)
		if err != nil {
			return retired, fmt.Errorf("reconcile-superseded: promote %q → %s: %w", id, newState, err)
		}
		if !res.NoOp {
			retired = append(retired, id)
		}
	}
	return retired, nil
}

type RouteResult = lifecycle.RouteResult

func RouteLane(opts Options, taskID, reason string) (RouteResult, error) {
	return withMover(opts, func(m *lifecycle.Mover) (RouteResult, error) { return m.RouteLane(taskID, reason) })
}

type FileResult = lifecycle.FileResult

func File(opts Options, raw []byte) (FileResult, error) {
	return withMover(opts, func(m *lifecycle.Mover) (FileResult, error) { return m.File(raw) })
}

func RouteConsole(opts Options, taskID, reason string, cycle int) (RouteResult, error) {
	return withMover(opts, func(m *lifecycle.Mover) (RouteResult, error) { return m.RouteConsole(taskID, reason, cycle) })
}
