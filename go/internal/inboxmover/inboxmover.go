// Package inboxmover ports legacy/scripts/utility/inbox-mover.sh.
//
// Atomic inbox lifecycle transitions (v9.6.0+). Three subcommands:
//
//	claim <task_id> <cycle>                     inbox/ → processing/cycle-N/
//	promote <task_id> <new_state> [<cycle>]     processing/ → processed|rejected|retry/
//	  [--commit-sha <sha>]
//	recover-orphans                             processing/cycle-X/ → inbox/ (dead cycles)
//
// All state transitions use a single atomic os.Rename (same-FS). Ledger
// writes are best-effort — failure to write the ledger never blocks a
// lifecycle operation.
//
// Exit codes (cmd layer maps from sentinel errors):
//
//	0 — success (or promote no-op for ship.sh compat)
//	1 — not-found / bad args (claim)
//	2 — mv failed (claim only)
//
// Since ADR-0103 unit 06 this file is the SEAM: the movers, the processed-
// record primitives and the ledger line live in the lifecycle leaf
// (internal/inboxmover/lifecycle); this file owns Options and its resolved
// defaults (the fallback file ledger, the git landing probe, the cycle-state
// reader), the ONE construction of a Mover per call (mover) and the Strangler
// facades every production root, the ship phase and the ACS predicates keep.
// Design: docs/architecture/decomposition/06-inboxmover.md.
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

// Sentinel errors — the leaf's own pointers, re-exported so errors.Is at the
// cmd layer's exit map, the ship consume and the declared-effects gate keep
// working unchanged.
var (
	ErrNotFound = lifecycle.ErrNotFound
	ErrMvFailed = lifecycle.ErrMvFailed
	ErrBadArgs  = lifecycle.ErrBadArgs
	ErrBadState = lifecycle.ErrBadState
	// ErrConsoleRouted refuses the lane handoff of an operator-owned item
	// (ADR-0074 I1): route:"console-*" or a protected fix surface.
	ErrConsoleRouted = lifecycle.ErrConsoleRouted
)

// The leaf's value objects under their historical spellings.
type (
	// LedgerAppender is the chained-append seam (interface at point of use);
	// satisfied by *ledger.FileLedger.
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

// Options shared by all subcommands.
type Options struct {
	ProjectRoot string
	InboxDir    string // defaulted to <ProjectRoot>/.evolve/inbox
	// LedgerPath defaults to <ProjectRoot>/.evolve/ledger.jsonl. Writes go
	// through the chained FileLedger constructed on this path's DIRECTORY —
	// the basename is not honored (FileLedger owns its file names).
	LedgerPath string
	// Ledger is the CHAINED append seam for inbox-lifecycle records —
	// defaulted from LedgerPath's directory. Lifecycle lines are chain
	// participants (prev_hash/entry_seq/tip like every other entry); the old
	// raw O_APPEND path was the fleet-concurrency chain-break generator.
	Ledger LedgerAppender
	Stderr io.Writer
	Now    func() time.Time

	// Test seam for cycle-state.json resolution (recover-orphans).
	ActiveCycleFn func() (string, error)

	// IsLandedFn is the delivery-evidence seam for processed-promotion. When
	// nil it defaults to a real `git merge-base --is-ancestor <sha> main`
	// check rooted at ProjectRoot; it is fail-open (treats the SHA as landed)
	// on any exec/seam error or non-git ProjectRoot, so a non-repo dir never
	// regresses existing Promote behavior — and the error travels with the
	// true, so the leaf reports INBOX_LANDED_CHECK_FAILED. Consulted ONLY when
	// a processed-promotion carries a non-empty CommitSHA.
	IsLandedFn func(sha string) (bool, error)

	// IsProtectedPath is the control-plane membership predicate for the
	// ADR-0074 claim floor (guards.IsProtectedSurface at composition roots).
	// nil disables only the files-derived rule; an explicit route:"console-*"
	// field always refuses the claim.
	IsProtectedPath func(path string) bool

	// Signals is the root's Signal Center the mover's inbox.warning events go
	// to (ADR-0103 unit 06). nil — every literal but the FAIL closeout's today
	// — is unwired: the leaf prints the legacy [inbox-mover] line instead (the
	// two-link producer), so nothing goes silent on a Center-less root.
	Signals *signalcenter.Center
}

// resolveOpts populates defaults derived from ProjectRoot.
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

// shaLandedOnMain reports whether sha is an ancestor of main via
// `git merge-base --is-ancestor <sha> main`. Exit 0 = ancestor (landed),
// exit 1 = cleanly-not-an-ancestor (unlanded). Any other exit (128 = non-git
// dir / unknown rev / no local main) or seam error is fail-open (treated as
// landed) so a non-repo ProjectRoot never blocks a promotion — delivery
// evidence gates, it never manufactures a false negative from missing git —
// and, since unit 06's review fold, RETURNED as the error so the leaf's
// landing gate reports INBOX_LANDED_CHECK_FAILED instead of promoting in
// silence (the hard-coded main is 06-F10's).
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

// readActiveCycle reads .evolve/cycle-state.json and returns the cycle_id
// field, or empty string + error if unavailable.
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

// logf emits a `[inbox-mover] …` line (the leaf's one LegacyPrefix) to the
// configured stderr — the voice of the sibling files (outcome, root_failure,
// the continuation trio) that stay in the host until unit 06b.
func (o *Options) logf(prefix, format string, args ...any) {
	fmt.Fprintf(o.Stderr, lifecycle.LegacyPrefix+prefix+format+"\n", args...)
}

// mover is the ONE lifecycle.New site (TestOptionsMover_OneConstructionSite)
// and the ONE projection of the resolved Options onto the leaf. Options is a
// value copied at every entry point, so there is no host object to cache a
// Mover on: the accessor pair of the other units collapses to this one
// function, built per call (allocation only — the resolved defaults were
// re-derived per call before the unit too). The retire hook and the
// run-workspace spelling close over the RESOLVED copy (releaseContinuationOnRetire
// reads ProjectRoot, Now and Stderr from it).
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

// --- Strangler facades — every production and ACS spelling unchanged --------

// Claim moves a file from inbox/ to processing/cycle-N/ atomically.
// Returns ErrNotFound if no inbox/*.json has matching task_id.
func Claim(opts Options, taskID, cycle string) (ClaimResult, error) {
	return opts.mover().Claim(taskID, cycle)
}

// Promote moves a file from processing/ (or inbox/ fallback) to
// processed|rejected|retry/. Exits 0-equivalent even when source not found
// — ship.sh must never block on this.
func Promote(opts Options, taskID, newState string, p PromoteOpts) (PromoteResult, error) {
	return opts.mover().Promote(taskID, newState, p)
}

// ShouldQuarantine is the pure ADR-0072 S5 decision (see lifecycle.ShouldQuarantine).
func ShouldQuarantine(failureCount, ceiling int, systemLevelFailure bool) bool {
	return lifecycle.ShouldQuarantine(failureCount, ceiling, systemLevelFailure)
}

// ReleaseFromQuarantine is the operator escape hatch for ADR-0072 S5: it moves
// an item out of .evolve/inbox/quarantine/ back to the inbox root and resets
// its failure_count to 0, so the next cycle's triage can re-pick it.
func ReleaseFromQuarantine(opts Options, taskID string) (PromoteResult, error) {
	return opts.mover().ReleaseFromQuarantine(taskID)
}

// RecoverOrphans moves files from processing/cycle-X/ back to inbox/ for
// any cycle X that is no longer active. Idempotent.
func RecoverOrphans(opts Options) (RecoverResult, error) {
	return opts.mover().RecoverOrphans()
}

// ReleaseCycleProcessing moves all *.json files from processing/cycle-<cycle>/
// back to the inbox root. It is scoped to the single named cycle dir and is
// idempotent: a missing or already-drained dir is a clean no-op. A file whose
// basename already exists at the inbox root (double-move race) is warned and
// skipped — the existing inbox-root copy is never clobbered.
func ReleaseCycleProcessing(opts Options, cycle int) (RecoverResult, error) {
	return ReleaseCycleProcessingWithReason(opts, cycle, "")
}

// ReleaseCycleProcessingWithReason is ReleaseCycleProcessing with an explicit
// ledger reason for each released item. An empty reason keeps the generic
// "cycle-release". Callers that drain because delivery failed (e.g. an
// unlanded ship commit, cycle-598 shape) pass a reason carrying "unlanded" so
// the ledger durably distinguishes a delivery-failure retry from an ordinary
// residual drain (inbox-promotion-requires-landed-ship).
func ReleaseCycleProcessingWithReason(opts Options, cycle int, reason string) (RecoverResult, error) {
	return releaseCycleProcessing(opts, cycle, reason, nil)
}

// quarantinePolicy is the drain's historical spelling of the leaf's Policy —
// the ADR-0072 S5 decision inputs (ceiling, system-level, the committed set
// with its nil-means-whole-drain contract) are documented ONCE, on
// lifecycle.Policy; an alias keeps outcome.go's literal and this file's
// signature on that one struct instead of a hand-projected mirror.
type quarantinePolicy = lifecycle.Policy

// releaseCycleProcessing is the shared drain core: the plain release-to-root
// when quar is nil, the ADR-0072 S5 failure drain (bump, quarantine at the
// ceiling, fail-open) when it is not. It stays UNEXPORTED on purpose (audit
// D3): ApplyCycleOutcome is the one public door into the cycle-outcome
// lifecycle, so the PASS-promote and FAIL-bump halves cannot drift apart
// behind a second entry point (never_duplicate_centralize; the leaf's
// Mover.Release is reachable only through this file — TestLifecycle_OnlyHostImportsTheLeaf).
func releaseCycleProcessing(opts Options, cycle int, reason string, quar *quarantinePolicy) (RecoverResult, error) {
	return opts.mover().Release(cycle, reason, quar)
}

// ReadFailureCount resolves taskID across the inbox root and processing/
// cycle-* dirs and returns its durable failure_count. (0,false) = item not
// found; (0,true) = item present, never failed.
func ReadFailureCount(opts Options, taskID string) (int, bool) {
	return opts.mover().ReadFailureCount(taskID)
}

// FindFileByTaskID resolves a task id to its file within one inbox directory
// (ids live INSIDE the JSON; filenames carry timestamps). Exported for the
// ship-side transactional consumption (consumption-rides-landing-ship): the
// ship needs the same id→file resolution against the WORKTREE's tracked
// inbox copy that this package uses against the runtime root.
func FindFileByTaskID(dir, taskID string) (string, error) {
	return lifecycle.FindFileByTaskID(dir, taskID)
}

// bumpFailureCount increments the durable "failure_count" on an inbox item
// (the root-resident twin RecordRootTaskFailure keeps this spelling).
func bumpFailureCount(path, reason string) (int, error) {
	return lifecycle.BumpFailureCount(path, reason)
}

// updateItemJSON rewrites an inbox item atomically (the continuation retire
// keeps this spelling).
func updateItemJSON(path string, mutate func(m map[string]json.RawMessage)) error {
	return lifecycle.UpdateItemJSON(path, mutate)
}

// --- Reconciliation: retire-by-id (superseded) ----------------------------

// SupersededInboxIDs extracts the top-level "superseded" string array from a
// triage-decision.json body: deduped, order-preserving. Returns nil on an
// absent field or invalid JSON — never panics.
//
// This is the data-driven declaration that feeds ReconcileSuperseded at ship,
// replacing the prose-only "verify vs HEAD, move to consumed" carryover
// instruction that silently lapsed for cycles 544..548. It names inbox items
// whose underlying work already shipped under a DIFFERENT id (e.g. cycle 544
// shipped the fleet-starvation observer as "recover-ship-fleet-starvation-
// observer", stranding its originating request "loop-self-prioritize-unmet-
// fleet-concurrency" in the inbox root).
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

// ReconcileSuperseded retires (promotes → newState) each live inbox item whose
// .id appears in supersededIDs, keyed by id ALONE — independent of the shipping
// cycle's committed top_n/skip_shipped set. This closes the inbox-lifecycle gap
// where an item shipped under a synthesized id strands its originating request
// in the inbox root, so scout/triage keep re-selecting already-completed work.
//
// It is selective (only declared ids move — Promote matches a single id and
// leaves every other item in place) and idempotent (an id not present in the
// inbox is a clean no-op via Promote's ship.sh-compat NoOp, never an error).
// Returns the ids actually retired, in declared order. Best-effort like the
// rest of the lifecycle: never blocks ship.
func ReconcileSuperseded(opts Options, supersededIDs []string, newState string, p PromoteOpts) ([]string, error) {
	var retired []string
	m := opts.mover() // one Mover for the loop — one resolve, one fallback ledger
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
