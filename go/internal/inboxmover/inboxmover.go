// Package inboxmover moves inbox items between lifecycle states and applies each
// cycle's outcome to the inbox; the movers themselves live in the lifecycle leaf.
// See docs/architecture/packages/internal-inboxmover.md.
package inboxmover

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Sentinel errors are the leaf's own pointers, so errors.Is holds across the seam.
var (
	ErrNotFound = lifecycle.ErrNotFound
	ErrMvFailed = lifecycle.ErrMvFailed
	ErrBadArgs  = lifecycle.ErrBadArgs
	ErrBadState = lifecycle.ErrBadState
	// ErrConsoleRouted refuses the lane handoff of an operator-owned item.
	// See ADR-0074.
	ErrConsoleRouted = lifecycle.ErrConsoleRouted
)

type (
	// LedgerAppender is the chained-append seam, satisfied by *ledger.FileLedger.
	LedgerAppender = lifecycle.LedgerAppender
	// ClaimResult describes what a claim did.
	ClaimResult = lifecycle.ClaimResult
	// PromoteOpts gathers the optional flag-bearing promote arguments.
	PromoteOpts = lifecycle.PromoteOpts
	// PromoteResult describes what a promote did.
	PromoteResult = lifecycle.PromoteResult
	// RecoverResult counts how many files were moved back to inbox/.
	RecoverResult = lifecycle.RecoverResult
)

// Options configures every mover call; each zero field takes a default, most derived from ProjectRoot.
type Options struct {
	ProjectRoot string
	InboxDir    string // defaults to <ProjectRoot>/.evolve/inbox
	// LedgerPath defaults to <ProjectRoot>/.evolve/ledger.jsonl; only its
	// directory is used, because FileLedger owns its file names.
	LedgerPath string
	// Ledger defaults to the chained FileLedger in LedgerPath's directory.
	Ledger LedgerAppender
	Stderr io.Writer
	Now    func() time.Time

	// ActiveCycleFn defaults to reading cycle_id from .evolve/cycle-state.json.
	ActiveCycleFn func() (string, error)

	// IsLandedFn gates a processed promotion that carries a CommitSHA. It fails
	// open (landed) on a probe fault and returns the error, so a non-git root never blocks.
	IsLandedFn func(sha string) (bool, error)

	// IsProtectedPath drives the files-derived half of the console-routing claim floor;
	// nil disables only that half, and route:"console-*" always refuses.
	IsProtectedPath func(path string) bool

	// Signals receives the mover's inbox.warning events; nil prints the legacy
	// [inbox-mover] line instead, so nothing goes silent on a Center-less root.
	Signals *signalcenter.Center
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
}

// shaLandedOnMain answers exit 0 as landed and exit 1 as unlanded. Any other
// answer fails open (landed) with the error, so missing git never blocks a promotion.
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

// mover is the one place a lifecycle Mover is built. The hooks close over the
// resolved copy, because releaseContinuationOnRetire reads its defaults.
func (o Options) mover() *lifecycle.Mover {
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
		lifecycle.WithSignals(func() *signalcenter.Center { return o.Signals }))
}

// Claim moves taskID's item from inbox/ to processing/cycle-N/, or returns ErrNotFound.
func Claim(opts Options, taskID, cycle string) (ClaimResult, error) {
	return opts.mover().Claim(taskID, cycle)
}

// Promote moves taskID's item to newState's dir; a missing source is a NoOp, never an error.
func Promote(opts Options, taskID, newState string, p PromoteOpts) (PromoteResult, error) {
	return opts.mover().Promote(taskID, newState, p)
}

// ShouldQuarantine is the pure retry-ceiling decision; a system-level failure never quarantines.
// See ADR-0072.
func ShouldQuarantine(failureCount, ceiling int, systemLevelFailure bool) bool {
	return lifecycle.ShouldQuarantine(failureCount, ceiling, systemLevelFailure)
}

// ReleaseFromQuarantine moves an item from quarantine/ to the inbox root and resets its failure_count.
func ReleaseFromQuarantine(opts Options, taskID string) (PromoteResult, error) {
	return opts.mover().ReleaseFromQuarantine(taskID)
}

// RecoverOrphans idempotently returns items claimed by inactive cycles to the inbox root.
func RecoverOrphans(opts Options) (RecoverResult, error) {
	return opts.mover().RecoverOrphans()
}

// ReleaseCycleProcessing drains one cycle's claims to the inbox root, never clobbering an existing root copy.
func ReleaseCycleProcessing(opts Options, cycle int) (RecoverResult, error) {
	return ReleaseCycleProcessingWithReason(opts, cycle, "")
}

// ReleaseCycleProcessingWithReason is ReleaseCycleProcessing with a ledger reason ("" means "cycle-release").
func ReleaseCycleProcessingWithReason(opts Options, cycle int, reason string) (RecoverResult, error) {
	return releaseCycleProcessing(opts, cycle, reason, nil)
}

type quarantinePolicy = lifecycle.Policy

// releaseCycleProcessing stays unexported so ApplyCycleOutcome remains the one
// door into the failure drain; a second entry point would let PASS and FAIL drift.
func releaseCycleProcessing(opts Options, cycle int, reason string, quar *quarantinePolicy) (RecoverResult, error) {
	return opts.mover().Release(cycle, reason, quar)
}

// ReadFailureCount returns taskID's durable failure_count and whether the item was found.
func ReadFailureCount(opts Options, taskID string) (int, bool) {
	return opts.mover().ReadFailureCount(taskID)
}

// FindFileByTaskID resolves a task id to its file in one inbox directory; ids live inside the JSON.
func FindFileByTaskID(dir, taskID string) (string, error) {
	return lifecycle.FindFileByTaskID(dir, taskID)
}

func bumpFailureCount(path, reason string) (int, error) {
	return lifecycle.BumpFailureCount(path, reason)
}

// updateItemJSON rewrites an inbox item atomically.
func updateItemJSON(path string, mutate func(m map[string]json.RawMessage)) error {
	return lifecycle.UpdateItemJSON(path, mutate)
}

// SupersededInboxIDs returns the deduped "superseded" ids of a triage decision, or nil on bad JSON.
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

// ReconcileSuperseded promotes each listed id to newState by id alone and returns the ids it retired.
func ReconcileSuperseded(opts Options, supersededIDs []string, newState string, p PromoteOpts) ([]string, error) {
	var retired []string
	m := opts.mover()
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

// RouteResult is the leaf's route receipt (the rewritten item's path).
type RouteResult = lifecycle.RouteResult

// RouteConsole rewrites the item in place as route:console-manual, so the claim floor refuses every later lane.
func RouteConsole(opts Options, taskID, reason string, cycle int) (RouteResult, error) {
	return opts.mover().RouteConsole(taskID, reason, cycle)
}
