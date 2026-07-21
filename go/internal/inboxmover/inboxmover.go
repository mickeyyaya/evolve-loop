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
package inboxmover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// Sentinel errors.
var (
	ErrNotFound = errors.New("inboxmover: task not found")
	ErrMvFailed = errors.New("inboxmover: mv failed")
	ErrBadArgs  = errors.New("inboxmover: bad arguments")
	ErrBadState = errors.New("inboxmover: invalid new_state")
)

// validStates is the set of allowed promote targets. "quarantine" is the
// ADR-0072 S5 terminal state: a task that has failed task_retry_ceiling times
// routes here (a sibling dir the triage scanner never walks) instead of being
// released back to the inbox root every cycle, so a poison todo stops being
// re-picked forever.
var validStates = map[string]bool{
	"processed":  true,
	"rejected":   true,
	"retry":      true,
	"quarantine": true,
}

// Options shared by all subcommands.
type Options struct {
	ProjectRoot string
	InboxDir    string // defaulted to <ProjectRoot>/.evolve/inbox
	LedgerPath  string // defaulted to <ProjectRoot>/.evolve/ledger.jsonl
	Stderr      io.Writer
	Now         func() time.Time

	// Test seam for cycle-state.json resolution (recover-orphans).
	ActiveCycleFn func() (string, error)

	// IsLandedFn is the delivery-evidence seam for processed-promotion. When
	// nil it defaults to a real `git merge-base --is-ancestor <sha> main`
	// check rooted at ProjectRoot; it is fail-open (treats the SHA as landed)
	// on any exec/seam error or non-git ProjectRoot, so a non-repo dir never
	// regresses existing Promote behavior. Consulted ONLY when a
	// processed-promotion carries a non-empty CommitSHA.
	IsLandedFn func(sha string) (bool, error)
}

// LedgerEntry is the NDJSON line written for each lifecycle transition.
type LedgerEntry struct {
	TS     string  `json:"ts"`
	Class  string  `json:"class"`
	Action string  `json:"action"`
	TaskID string  `json:"task_id"`
	From   string  `json:"from"`
	To     string  `json:"to"`
	Cycle  *int    `json:"cycle"`   // null when empty
	GitSHA *string `json:"git_sha"` // null when empty
	Reason string  `json:"reason"`
}

// resolveOpts populates defaults derived from ProjectRoot.
func (o *Options) resolveOpts() {
	if o.InboxDir == "" {
		o.InboxDir = filepath.Join(o.ProjectRoot, ".evolve", "inbox")
	}
	if o.LedgerPath == "" {
		o.LedgerPath = filepath.Join(o.ProjectRoot, ".evolve", "ledger.jsonl")
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
// dir / unknown rev) or seam error is fail-open (treated as landed) so a
// non-repo ProjectRoot never blocks a promotion — delivery evidence gates,
// it never manufactures a false negative from missing git.
func shaLandedOnMain(root, sha string) (bool, error) {
	_, _, code, err := gitexec.Default(root).Capture(context.Background(), "merge-base", "--is-ancestor", sha, "main")
	if err != nil {
		return true, nil
	}
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return true, nil
	}
}

// logf emits a "[inbox-mover] ..." line to the configured stderr.
func (o *Options) logf(prefix, format string, args ...any) {
	fmt.Fprintf(o.Stderr, "[inbox-mover] "+prefix+format+"\n", args...)
}

// --- Subcommand: claim -----------------------------------------------------

// ClaimResult describes what happened.
type ClaimResult struct {
	SrcPath  string
	DestPath string
}

// Claim moves a file from inbox/ to processing/cycle-N/ atomically.
// Returns ErrNotFound if no inbox/*.json has matching task_id.
func Claim(opts Options, taskID, cycle string) (ClaimResult, error) {
	opts.resolveOpts()
	res := ClaimResult{}
	if taskID == "" || cycle == "" {
		opts.logf("ERROR: ", "usage: claim <task_id> <cycle>")
		return res, fmt.Errorf("%w: claim requires task_id and cycle", ErrBadArgs)
	}
	src, err := findFileByTaskID(opts.InboxDir, taskID)
	if err != nil {
		opts.logf("WARN: ", "claim: task '%s' not found in %s", taskID, opts.InboxDir)
		return res, fmt.Errorf("%w: %s", ErrNotFound, taskID)
	}
	base := filepath.Base(src)
	destDir := filepath.Join(opts.InboxDir, "processing", "cycle-"+cycle)
	dest := filepath.Join(destDir, base)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		opts.logf("ERROR: ", "claim: mkdir -p '%s' failed: %v", destDir, err)
		return res, fmt.Errorf("%w: mkdir: %v", ErrMvFailed, err)
	}
	if err := os.Rename(src, dest); err != nil {
		opts.logf("WARN: ", "claim: mv failed for '%s' (may already be claimed): %v", taskID, err)
		return res, fmt.Errorf("%w: %v", ErrMvFailed, err)
	}
	res.SrcPath = src
	res.DestPath = dest
	opts.logf("", "claimed: %s → processing/cycle-%s/", base, cycle)
	writeLedger(opts, LedgerEntry{
		Action: "claim",
		TaskID: taskID,
		From:   ".evolve/inbox/" + base,
		To:     ".evolve/inbox/processing/cycle-" + cycle + "/" + base,
		Cycle:  intPtr(cycle),
		Reason: "triage-claim",
	})
	return res, nil
}

// --- Subcommand: promote ---------------------------------------------------

// PromoteOpts gathers the optional flag-bearing arguments.
type PromoteOpts struct {
	Cycle     string // empty → "0"
	CommitSHA string // empty → no SHA prefix
}

// PromoteResult describes what happened.
type PromoteResult struct {
	SrcPath  string
	DestPath string
	NoOp     bool // true if source was not found (ship.sh compat → exit 0)
}

// Promote moves a file from processing/ (or inbox/ fallback) to
// processed|rejected|retry/. Exits 0-equivalent even when source not found
// — ship.sh must never block on this.
func Promote(opts Options, taskID, newState string, p PromoteOpts) (PromoteResult, error) {
	opts.resolveOpts()
	res := PromoteResult{}
	if taskID == "" || newState == "" {
		opts.logf("ERROR: ", "usage: promote <task_id> <new_state> [<cycle>] [--commit-sha <sha>]")
		return res, fmt.Errorf("%w: promote requires task_id and new_state", ErrBadArgs)
	}
	if !validStates[newState] {
		opts.logf("ERROR: ", "promote: invalid state '%s'; must be processed|rejected|retry", newState)
		return res, fmt.Errorf("%w: %s", ErrBadState, newState)
	}

	// Search processing/cycle-*/ first, then inbox/ fallback.
	src, srcRel := "", ""
	procDir := filepath.Join(opts.InboxDir, "processing")
	if entries, err := os.ReadDir(procDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), "cycle-") {
				continue
			}
			d := filepath.Join(procDir, e.Name())
			if found, err := findFileByTaskID(d, taskID); err == nil {
				src, srcRel = found, "processing"
				break
			}
		}
	}
	if src == "" {
		if found, err := findFileByTaskID(opts.InboxDir, taskID); err == nil {
			src, srcRel = found, "inbox"
		}
	}
	if src == "" {
		opts.logf("WARN: ", "promote: task '%s' not found in processing/ or inbox/ — already moved?", taskID)
		res.NoOp = true
		return res, nil // ship.sh compat: NoOp success
	}

	// Delivery-evidence gate (inbox-promotion-requires-landed-ship): a
	// processed-promotion carrying a ship SHA must be backed by that commit
	// actually landing on main. Historically Promote keyed on the caller's
	// PASS verdict alone, so a push-rejected ship whose recovery still
	// reported PASS could bury a directive in processed/ under a SHA
	// git log --all never contained. An unlanded SHA reroutes to retry/
	// instead. Empty SHA (legacy/ship.sh-compat) and non-processed states
	// skip the check entirely.
	reroutedUnlanded := false
	if newState == "processed" && p.CommitSHA != "" {
		landed, err := opts.IsLandedFn(p.CommitSHA)
		if err != nil {
			landed = true // fail-open: never block a promotion on a gate error
		}
		if !landed {
			newState = "retry"
			reroutedUnlanded = true
			opts.logf("WARN: ", "promote: ship SHA %s for '%s' not landed on main — rerouting to retry/ instead of processed/", p.CommitSHA, taskID)
		}
	}

	base := filepath.Base(src)
	destDir, dest := promoteDestPath(opts.InboxDir, base, newState, p)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		opts.logf("WARN: ", "promote: mkdir -p '%s' failed — leaving file in %s/", destDir, srcRel)
		writeLedger(opts, LedgerEntry{
			Action: "promote-warn",
			TaskID: taskID,
			From:   ".evolve/inbox/" + srcRel + "/" + base,
			To:     dest,
			Cycle:  intPtr(p.Cycle),
			GitSHA: strPtr(p.CommitSHA),
			Reason: "mkdir-failed",
		})
		res.NoOp = true
		return res, nil
	}
	if err := os.Rename(src, dest); err != nil {
		opts.logf("WARN: ", "promote: mv failed for '%s' → %s (leaving in %s/): %v",
			taskID, newState, srcRel, err)
		writeLedger(opts, LedgerEntry{
			Action: "promote-warn",
			TaskID: taskID,
			From:   ".evolve/inbox/" + srcRel + "/" + base,
			To:     dest,
			Cycle:  intPtr(p.Cycle),
			GitSHA: strPtr(p.CommitSHA),
			Reason: "mv-failed",
		})
		res.NoOp = true
		return res, nil
	}
	res.SrcPath = src
	res.DestPath = dest
	opts.logf("", "promoted: %s → %s/", base, newState)
	reason := "ship-promote-" + newState
	if reroutedUnlanded {
		reason = "ship-promote-retry-unlanded-sha"
	}
	writeLedger(opts, LedgerEntry{
		Action: "promote",
		TaskID: taskID,
		From:   ".evolve/inbox/" + srcRel + "/" + base,
		To:     dest,
		Cycle:  intPtr(p.Cycle),
		GitSHA: strPtr(p.CommitSHA),
		Reason: reason,
	})
	return res, nil
}

// promoteDestPath computes (destDir, dest) for a given new state.
// Mirrors bash:
//
//	processed: <inbox>/processed/cycle-<cycle|0>/[<sha8>-]<base>
//	rejected:  <inbox>/rejected/cycle-<cycle|0>/<base>
//	retry:     <inbox>/retry/<base>
func promoteDestPath(inboxDir, base, newState string, p PromoteOpts) (string, string) {
	switch newState {
	case "processed":
		effCycle := p.Cycle
		if effCycle == "" {
			effCycle = "0"
		}
		destDir := filepath.Join(inboxDir, "processed", "cycle-"+effCycle)
		if p.CommitSHA != "" {
			sha8 := p.CommitSHA
			if len(sha8) > 8 {
				sha8 = sha8[:8]
			}
			return destDir, filepath.Join(destDir, sha8+"-"+base)
		}
		return destDir, filepath.Join(destDir, base)
	case "rejected":
		effCycle := p.Cycle
		if effCycle == "" {
			effCycle = "0"
		}
		destDir := filepath.Join(inboxDir, "rejected", "cycle-"+effCycle)
		return destDir, filepath.Join(destDir, base)
	case "retry":
		destDir := filepath.Join(inboxDir, "retry")
		return destDir, filepath.Join(destDir, base)
	case "quarantine":
		// Flat sibling dir (no cycle-N subdir): quarantine is terminal, not
		// per-cycle, and LoadDir skips subdirs so the item vanishes from triage.
		destDir := filepath.Join(inboxDir, "quarantine")
		return destDir, filepath.Join(destDir, base)
	}
	return "", ""
}

// ShouldQuarantine is the pure ADR-0072 S5 decision: quarantine a task once its
// task-level failure count reaches the configured ceiling. A zero (or negative)
// ceiling disables quarantine entirely, and a system-level failure NEVER
// quarantines — the S3 floor halt takes precedence (AC4). The caller passes the
// ceiling (FailureThresholds.TaskRetryCeiling, default 2) and the system-level
// flag; inboxmover deliberately does not import internal/policy so the package
// layering stays intact.
func ShouldQuarantine(failureCount, ceiling int, systemLevelFailure bool) bool {
	return ceiling > 0 && !systemLevelFailure && failureCount >= ceiling
}

// ReleaseFromQuarantine is the operator escape hatch for ADR-0072 S5: it moves
// an item out of .evolve/inbox/quarantine/ back to the inbox root and resets its
// failure_count to 0, so the next cycle's triage can re-pick it. Returns
// ErrNotFound when no quarantined item carries taskID. Idempotent-safe: a
// basename already present at the inbox root is left untouched (never clobbered)
// and reported as ErrMvFailed. The counter reset keeps the item JSON the single
// source of truth for task-level failure memory (no stale count strands it).
func ReleaseFromQuarantine(opts Options, taskID string) (PromoteResult, error) {
	opts.resolveOpts()
	res := PromoteResult{}
	if taskID == "" {
		return res, fmt.Errorf("%w: release-from-quarantine requires task_id", ErrBadArgs)
	}
	qDir := filepath.Join(opts.InboxDir, "quarantine")
	src, err := findFileByTaskID(qDir, taskID)
	if err != nil {
		return res, fmt.Errorf("%w: %s (not in quarantine)", ErrNotFound, taskID)
	}
	base := filepath.Base(src)
	dest := filepath.Join(opts.InboxDir, base)
	if _, statErr := os.Stat(dest); statErr == nil {
		return res, fmt.Errorf("%w: %s already at inbox root", ErrMvFailed, base)
	}
	// Reset the failure counter before re-entry so a released item gets a fresh
	// retry budget (best-effort — a rewrite failure must not block the release).
	_ = updateItemJSON(src, func(m map[string]json.RawMessage) {
		zero, _ := json.Marshal(0)
		m["failure_count"] = zero
		delete(m, "last_failure_reason")
	})
	if mvErr := os.Rename(src, dest); mvErr != nil {
		return res, fmt.Errorf("%w: %v", ErrMvFailed, mvErr)
	}
	res.SrcPath = src
	res.DestPath = dest
	opts.logf("", "released from quarantine: %s → inbox/", base)
	writeLedger(opts, LedgerEntry{
		Action: "quarantine-release",
		TaskID: taskID,
		From:   ".evolve/inbox/quarantine/" + base,
		To:     ".evolve/inbox/" + base,
		Reason: "operator-quarantine-release",
	})
	return res, nil
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
	for _, id := range supersededIDs {
		if id == "" {
			continue
		}
		res, err := Promote(opts, id, newState, p)
		if err != nil {
			return retired, fmt.Errorf("reconcile-superseded: promote %q → %s: %w", id, newState, err)
		}
		if !res.NoOp {
			retired = append(retired, id)
		}
	}
	return retired, nil
}

// --- Subcommand: recover-orphans ------------------------------------------

// RecoverResult counts how many files were moved back to inbox/.
type RecoverResult struct {
	Recovered int
	Paths     []string
}

// RecoverOrphans moves files from processing/cycle-X/ back to inbox/ for
// any cycle X that is no longer active. Idempotent.
func RecoverOrphans(opts Options) (RecoverResult, error) {
	opts.resolveOpts()
	res := RecoverResult{Paths: []string{}}

	procDir := filepath.Join(opts.InboxDir, "processing")
	if info, err := os.Stat(procDir); err != nil || !info.IsDir() {
		opts.logf("", "recover-orphans: no processing/ dir — nothing to do")
		return res, nil
	}

	activeCycle, _ := opts.ActiveCycleFn()
	if activeCycle == "" {
		activeCycle = "-1"
	}

	entries, _ := os.ReadDir(procDir)
	// Sort for deterministic test output.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "cycle-") {
			continue
		}
		cycleNum := strings.TrimPrefix(e.Name(), "cycle-")
		if cycleNum == activeCycle {
			opts.logf("", "recover-orphans: cycle-%s/ is active — skipping", cycleNum)
			continue
		}
		dir := filepath.Join(procDir, e.Name())
		files, _ := os.ReadDir(dir)
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			base := f.Name()
			src := filepath.Join(dir, base)
			dest := filepath.Join(opts.InboxDir, base)
			taskID := readTaskIDOrUnknown(src)
			if err := os.Rename(src, dest); err != nil {
				opts.logf("WARN: ", "recover-orphans: mv failed for %s (leaving in processing/): %v", base, err)
				continue
			}
			opts.logf("", "recovered: %s ← processing/cycle-%s/", base, cycleNum)
			writeLedger(opts, LedgerEntry{
				Action: "recover",
				TaskID: taskID,
				From:   ".evolve/inbox/processing/cycle-" + cycleNum + "/" + base,
				To:     ".evolve/inbox/" + base,
				Cycle:  intPtr(cycleNum),
				Reason: "orphan-recovery-cycle-not-active",
			})
			res.Recovered++
			res.Paths = append(res.Paths, dest)
		}
	}
	opts.logf("", "recover-orphans: %d file(s) recovered", res.Recovered)
	return res, nil
}

// --- Subcommand: release-cycle-processing ---------------------------------

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

// quarantinePolicy carries the ADR-0072 S5 decision inputs for a failure drain:
// the task-level retry ceiling and whether this cycle's failure was
// system-level (an S3 floor halt), which suppresses quarantine (AC4).
type quarantinePolicy struct {
	ceiling     int
	systemLevel bool
}

// ReleaseCycleProcessingWithQuarantine is the ADR-0072 S5 failure-drain: it
// releases processing/cycle-<cycle>/ like ReleaseCycleProcessingWithReason but
// first increments each item's durable task-level failure_count — the single
// source of truth that replaces the dead cyclestate.CyclesUnpicked counter —
// and, once that count reaches `ceiling` on a task-level failure (systemLevel
// false, honoring S3 precedence), routes the item to .evolve/inbox/quarantine/
// instead of back to the inbox root, so a poison todo stops being re-picked
// every cycle. Fail-open end to end: any per-item read/write error falls back
// to a normal release so a bookkeeping fault never strands nor wrongly
// quarantines an item. A ceiling <= 0 is exactly ReleaseCycleProcessingWithReason.
func ReleaseCycleProcessingWithQuarantine(opts Options, cycle int, reason string, ceiling int, systemLevel bool) (RecoverResult, error) {
	return releaseCycleProcessing(opts, cycle, reason, &quarantinePolicy{ceiling: ceiling, systemLevel: systemLevel})
}

// releaseCycleProcessing is the shared drain core. quar==nil is the plain
// release-to-root behavior; a non-nil quar applies the S5 quarantine decision
// per item before falling back to the release.
func releaseCycleProcessing(opts Options, cycle int, reason string, quar *quarantinePolicy) (RecoverResult, error) {
	if reason == "" {
		reason = "cycle-release"
	}
	opts.resolveOpts()
	res := RecoverResult{Paths: []string{}}

	cycleDir := filepath.Join(opts.InboxDir, "processing", fmt.Sprintf("cycle-%d", cycle))
	info, err := os.Stat(cycleDir)
	if err != nil {
		if os.IsNotExist(err) {
			opts.logf("", "release-cycle: processing/cycle-%d/ absent — nothing to release", cycle)
			return res, nil
		}
		return res, fmt.Errorf("release-cycle: stat processing/cycle-%d: %w", cycle, err)
	}
	if !info.IsDir() {
		return res, nil
	}

	files, _ := os.ReadDir(cycleDir)
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		base := f.Name()
		src := filepath.Join(cycleDir, base)
		dest := filepath.Join(opts.InboxDir, base)
		taskID := readTaskIDOrUnknown(src)

		// ADR-0072 S5: bump the durable failure_count and quarantine at the
		// ceiling instead of releasing back to root. Fail-open — a read/write
		// error skips quarantine and falls through to the normal release.
		if quar != nil {
			if count, bumpErr := bumpFailureCount(src, reason); bumpErr == nil &&
				ShouldQuarantine(count, quar.ceiling, quar.systemLevel) {
				if pr, pErr := Promote(opts, taskID, "quarantine", PromoteOpts{Cycle: fmt.Sprintf("%d", cycle)}); pErr == nil && !pr.NoOp {
					opts.logf("", "quarantined: %s (task-level failure #%d >= ceiling %d) ← processing/cycle-%d/", base, count, quar.ceiling, cycle)
					res.Recovered++
					res.Paths = append(res.Paths, pr.DestPath)
					continue
				}
			}
		}

		// Double-move race: a concurrent release already landed this file.
		if _, statErr := os.Stat(dest); statErr == nil {
			opts.logf("WARN: ", "release-cycle: %s already at inbox root (double-move for %s) — skipping", base, taskID)
			continue
		}

		if mvErr := os.Rename(src, dest); mvErr != nil {
			opts.logf("WARN: ", "release-cycle: mv failed for %s (leaving in processing/cycle-%d/): %v", base, cycle, mvErr)
			continue
		}
		opts.logf("", "released: %s ← processing/cycle-%d/", base, cycle)
		writeLedger(opts, LedgerEntry{
			Action: "recover",
			TaskID: taskID,
			From:   fmt.Sprintf(".evolve/inbox/processing/cycle-%d/%s", cycle, base),
			To:     ".evolve/inbox/" + base,
			Cycle:  intPtr(fmt.Sprintf("%d", cycle)),
			Reason: reason,
		})
		res.Recovered++
		res.Paths = append(res.Paths, dest)
	}
	opts.logf("", "release-cycle: %d file(s) released from cycle-%d", res.Recovered, cycle)
	return res, nil
}

// bumpFailureCount increments the durable "failure_count" on an inbox item JSON
// (the single source of truth for ADR-0072 S5 task-level failure memory) and
// stamps the latest failure reason, preserving every other field. Returns the
// new count. Atomic (write-tmp + rename) so a crash never leaves a half-written
// item. Any parse/IO error is returned so the caller can fail open.
func bumpFailureCount(path, reason string) (int, error) {
	count := 0
	err := updateItemJSON(path, func(m map[string]json.RawMessage) {
		if raw, ok := m["failure_count"]; ok {
			_ = json.Unmarshal(raw, &count)
		}
		count++
		cb, _ := json.Marshal(count)
		m["failure_count"] = cb
		if reason != "" {
			rb, _ := json.Marshal(reason)
			m["last_failure_reason"] = rb
		}
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// updateItemJSON reads an inbox item, applies mutate to its top-level field map
// (preserving every field the loop does not touch), and writes it back
// atomically (write-tmp + rename). Any parse/IO error is returned so callers can
// fail open. mutate must not retain the map after returning.
func updateItemJSON(path string, mutate func(m map[string]json.RawMessage)) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return err
	}
	mutate(m)
	out, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// --- Helpers ---------------------------------------------------------------

// findFileByTaskID scans <dir>/*.json and returns the path of the first
// file whose JSON .id equals taskID.
func findFileByTaskID(dir, taskID string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			continue
		}
		if doc.ID == taskID {
			return path, nil
		}
	}
	return "", ErrNotFound
}

// readTaskIDOrUnknown returns the JSON .id of a file, or "unknown" on failure.
func readTaskIDOrUnknown(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	var doc struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "unknown"
	}
	if doc.ID == "" {
		return "unknown"
	}
	return doc.ID
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

// writeLedger appends one NDJSON line. Best-effort: if the ledger or its
// parent dir is unwritable, drop silently (matches bash semantics).
func writeLedger(opts Options, entry LedgerEntry) {
	entry.TS = opts.Now().UTC().Format(time.RFC3339)
	entry.Class = "inbox-lifecycle"
	body, err := json.Marshal(entry)
	if err != nil {
		return
	}
	dir := filepath.Dir(opts.LedgerPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(opts.LedgerPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(body)
	_, _ = f.Write([]byte("\n"))
}

// intPtr returns a *int from a numeric string, or nil if empty/unparseable.
// Mirrors bash semantics: empty cycle → null; numeric → numeric.
func intPtr(s string) *int {
	if s == "" {
		return nil
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return nil
	}
	return &v
}

// strPtr returns a *string, or nil if empty.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
