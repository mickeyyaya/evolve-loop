package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// ErrNothingToReset is returned when there is no in-progress cycle to seal
// (no cycle-state.json, or cycle_id == 0).
var ErrNothingToReset = errors.New("reset: no in-progress cycle to seal")

// ErrCycleOwnedLive is returned when the cycle's per-run lease is still FRESH
// (its owner is alive, heartbeating) and SealOptions.Force was not set. A
// stale/missing lease (dead owner) seals freely. SealResult carries
// LeaseOwnerPID + LeaseHeartbeatAge so the caller can name the live owner.
var ErrCycleOwnedLive = errors.New("reset: cycle is owned by a live run (fresh lease) — refusing to seal without --force")

// ledgerAppender is the narrow slice of core.Ledger that SealCycle needs.
// Accepting it (rather than the full Ledger) keeps the dependency minimal and
// the test fake trivial; *ledger.FileLedger satisfies it.
type ledgerAppender interface {
	Append(ctx context.Context, entry LedgerEntry) error
}

// SealOptions configures a SealCycle. Now/GitHead are test seams.
type SealOptions struct {
	EvolveDir   string                                   // .evolve/ directory holding cycle-state.json + state.json + runs/
	ProjectRoot string                                   // writable repo root (for git HEAD capture)
	Reason      string                                   // operator-supplied reason, recorded in the manifest + ledger
	DryRun      bool                                     // compute the plan, mutate nothing
	Now         func() time.Time                         // defaults to time.Now
	GitHead     func(projectRoot string) (string, error) // defaults to defaultCurrentHead
	// Force seals even when the cycle's run lease is FRESH (a live owner). The
	// default refuses with ErrCycleOwnedLive. A stale/missing lease never needs it.
	Force bool
	// LeaseTTL overrides the liveness-fence freshness window; 0 = runlease.DefaultTTL.
	LeaseTTL time.Duration
	// PidAlive probes whether the lease's owner process is still running. When
	// set, the liveness fence uses runlease.OwnerLive (fresh heartbeat AND
	// alive pid) instead of freshness alone: a crashed owner whose heartbeat is
	// still fresh (the 2-6min post-crash window) now seals WITHOUT --force. nil
	// preserves the old freshness-only fence for un-migrated callers
	// (back-compat).
	PidAlive func(pid int) bool
	// AutomatedRecovery marks a seal driven by unattended boot self-heal (a
	// dead owner PID) rather than an explicit human `evolve cycle reset`. The
	// ledger verify epoch-anchor resolver trusts a ledger line only when its
	// Role is exactly "operator" — a real human sign-off — so an automated
	// seal carries a distinct role that the resolver does not recognize: the
	// recovery still runs (clearing the role-gate block), it just never gains
	// operator trust for chain verification.
	AutomatedRecovery bool
}

// autosealRole is the Role an unattended boot self-heal seal carries — never
// "operator", so it cannot move the ledger verify epoch anchor (see
// SealOptions.AutomatedRecovery).
const autosealRole = "operator-autoseal"

// SealResult reports what was (or, in dry-run, would be) sealed.
type SealResult struct {
	SealedCycleID int
	SealedPhase   string
	Workspace     string
	ArchiveDir    string
	NextCycle     int
	DryRun        bool
	// ForcedOverLiveOwner is true when Force sealed a cycle whose lease was still
	// fresh (a live owner was overridden) — recorded for the audit trail.
	ForcedOverLiveOwner bool
	// LeaseOwnerPID / LeaseHeartbeatAge surface the live owner on an
	// ErrCycleOwnedLive refusal (and in dry-run) so the caller can name it.
	LeaseOwnerPID     int
	LeaseHeartbeatAge time.Duration
}

// SealCycle abandons a stuck/unfinished cycle while preserving its full
// history: the workspace is archived to a sibling
// <workspace>.reset-<UTCnano>/ directory (never deleted) alongside a
// cycle-state snapshot and a reset-manifest.json, lastCycleNumber advances
// past the sealed cycle so the number is never reused, an auditable ledger
// entry is appended, and cycle-state.json is removed to disarm the
// phase-gate precondition for a fresh cycle.
func SealCycle(ctx context.Context, ledger ledgerAppender, opts SealOptions) (SealResult, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	gitHead := opts.GitHead
	if gitHead == nil {
		gitHead = defaultCurrentHead
	}

	csPath := ResolveCycleStatePath(opts.EvolveDir) // fleet per-run override when set
	raw, err := os.ReadFile(csPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SealResult{}, ErrNothingToReset
		}
		return SealResult{}, fmt.Errorf("reset: read cycle-state: %w", err)
	}
	var cs map[string]any
	if err := json.Unmarshal(raw, &cs); err != nil {
		return SealResult{}, fmt.Errorf("reset: parse cycle-state: %w", err)
	}
	cycleID := intFromAny(cs["cycle_id"])
	if cycleID == 0 {
		return SealResult{}, ErrNothingToReset
	}
	phase := strFromAny(cs["phase"])
	workspace := strFromAny(cs["workspace_path"])
	if workspace == "" {
		workspace = filepath.Join(opts.EvolveDir, "runs", fmt.Sprintf("cycle-%d", cycleID))
	}
	// One timestamp for the whole seal so the archive suffix, manifest
	// sealed_at, ledger TS, and state lastUpdated all agree.
	t := now()
	stamp := t.UTC().Format("20060102T150405.000000000")
	rfc := t.UTC().Format(time.RFC3339)
	archiveDir := workspace + ".reset-" + stamp

	res := SealResult{
		SealedCycleID: cycleID,
		SealedPhase:   phase,
		Workspace:     workspace,
		ArchiveDir:    archiveDir,
		NextCycle:     cycleID + 1,
		DryRun:        opts.DryRun,
	}

	// Liveness fence: refuse to seal a cycle whose run owner is still alive.
	// runlease is the SSOT for liveness: a fresh heartbeat AND (when PidAlive is
	// wired) a running owner pid. A stale heartbeat is never live regardless of
	// pid state (guards pid reuse); a dead owner with a still-fresh heartbeat
	// now seals freely. A stale/missing/unparsable lease ⇒ the owner is gone ⇒
	// safe to seal. Dry-run is a read-only preview and never blocks; Force
	// overrides a live owner (loud, operator-attested) and is recorded in
	// ForcedOverLiveOwner.
	if lease, ok, _ := runlease.Read(workspace); ok && runlease.OwnerLive(lease, t, opts.LeaseTTL, opts.PidAlive) {
		res.LeaseOwnerPID = lease.OwnerPID
		if hb, perr := time.Parse(time.RFC3339Nano, lease.HeartbeatAt); perr == nil {
			res.LeaseHeartbeatAge = t.Sub(hb)
		}
		switch {
		case opts.DryRun:
			// preview surfaces the live owner but never blocks
		case opts.Force:
			res.ForcedOverLiveOwner = true
		default:
			return res, ErrCycleOwnedLive
		}
	}

	if opts.DryRun {
		return res, nil
	}

	// Defensive: workspace_path is orchestrator-written and should always be
	// under the evolve/project root, but this is a destructive rename — refuse
	// a path that escapes both roots rather than move an arbitrary directory.
	if !pathWithin(workspace, opts.EvolveDir) && !pathWithin(workspace, opts.ProjectRoot) {
		return SealResult{}, fmt.Errorf("reset: workspace_path %q is outside evolveDir/projectRoot — refusing to rename", workspace)
	}

	head, _ := gitHead(opts.ProjectRoot)
	manifest := map[string]any{
		"sealed_cycle":       cycleID,
		"sealed_phase":       phase,
		"reason":             opts.Reason,
		"sealed_at":          rfc,
		"git_head":           head,
		"original_workspace": workspace,
		"next_cycle":         cycleID + 1,
	}
	manRaw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return SealResult{}, fmt.Errorf("reset: marshal manifest: %w", err)
	}
	manRaw = append(manRaw, '\n')

	// An operator reset is an abnormal termination and must LEARN, not just
	// archive. The deterministic retrospective is written INTO the workspace
	// alongside the snapshot/manifest (before the rename — same
	// complete-at-appearance invariant), the lesson goes to
	// instincts/lessons/.
	ev := faillearn.FailureEvent{
		Cycle:          cycleID,
		FailedPhase:    phase,
		Scope:          faillearn.ScopeReset,
		Classification: string(failurelog.OperatorReset),
		Verdict:        "RESET",
		Summary:        fmt.Sprintf("operator reset of cycle %d at phase %s: %s", cycleID, phase, opts.Reason),
		// archiveDir is the DURABLE location: the workspace path stops
		// existing at the rename below, so the lesson points at where the
		// evidence will live, not where it briefly was.
		EvidencePaths: []string{archiveDir},
		GitHead:       head,
		Now:           t,
	}
	lessonsDir := filepath.Join(opts.EvolveDir, "instincts", "lessons")

	// 1+2. Seal the workspace into the archive. The snapshot + manifest are
	// written INTO the workspace BEFORE the rename, so the archive is complete
	// the instant it appears — a partial failure can never split the workspace
	// files and the metadata across two timestamped archive dirs. When the
	// workspace was never created, mkdir the archive and write metadata there.
	writeMeta := func(dir string) error {
		if err := os.WriteFile(filepath.Join(dir, "cycle-state.snapshot.json"), raw, 0o644); err != nil {
			return fmt.Errorf("reset: write snapshot: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "reset-manifest.json"), manRaw, 0o644); err != nil {
			return fmt.Errorf("reset: write manifest: %w", err)
		}
		// Hard-fail (unlike the WARN-only orchestrator/loop floor sites):
		// this runs BEFORE the destructive rename, so aborting here leaves
		// the workspace intact for a clean retry — strictly safer than
		// sealing without learning.
		if err := faillearn.WriteArtifacts(ev, dir, lessonsDir); err != nil {
			return fmt.Errorf("reset: write learning artifacts: %w", err)
		}
		return nil
	}
	if info, statErr := os.Stat(workspace); statErr == nil && info.IsDir() {
		if err := writeMeta(workspace); err != nil {
			return SealResult{}, err
		}
		if err := os.Rename(workspace, archiveDir); err != nil {
			return SealResult{}, fmt.Errorf("reset: archive workspace %s: %w", workspace, err)
		}
	} else {
		if err := os.MkdirAll(archiveDir, 0o755); err != nil {
			return SealResult{}, fmt.Errorf("reset: create archive %s: %w", archiveDir, err)
		}
		if err := writeMeta(archiveDir); err != nil {
			return SealResult{}, err
		}
	}

	// 3. Auditable ledger entry (append-only, hash-chained). Role is
	// "operator" only for an explicit human `evolve cycle reset`; an
	// AutomatedRecovery seal writes autosealRole instead (see
	// SealOptions.AutomatedRecovery).
	if ledger != nil {
		role := "operator"
		if opts.AutomatedRecovery {
			role = autosealRole
		}
		entry := LedgerEntry{
			TS:           rfc,
			Cycle:        0,
			CycleLabel:   fmt.Sprintf("reset-seal-cycle-%d", cycleID),
			Role:         role,
			Kind:         "reset",
			GitHEAD:      head,
			ArtifactPath: archiveDir,
		}
		if err := ledger.Append(ctx, entry); err != nil {
			return SealResult{}, fmt.Errorf("reset: append ledger: %w", err)
		}
	}

	// 3b+4. Mutate state.json under the SAME "<state.json>.lock" sidecar that
	// storage.UpdateState holds for its whole RMW (flock.PathLock — the single
	// documented single-writer contract). Both the failedApproaches record and
	// the lastCycleNumber/batch RMW below were previously unlocked, so in fleet
	// mode (2+ concurrent lanes are the live operating mode) a concurrent locked
	// UpdateState could read stale state and clobber this seal's writes.
	// flock is blocking and per-open-file-description;
	// neither failurelog.Record nor readJSONMapFile/writeJSONMapFileAtomic locks
	// internally, so wrapping them here is not re-entrant. ORDERING IS
	// LOAD-BEARING: Record runs BEFORE the seal's own read-modify-write, so the
	// seal's write (which re-reads the file, picking up the new entry) stays the
	// final authority on lastCycleNumber / currentBatch / lastUpdated. A missing
	// state.json is soft-skipped (preflight owns creating it; the seal's own
	// write below creates it fresh).
	// The lock sits on the RESOLVED path: a worktree's state.json is a link to
	// canonical, and a sidecar beside the link is one no canonical-path writer
	// ever contends on (cross-tree lost update).
	statePath := filepath.Join(opts.EvolveDir, "state.json")
	if err := flock.WithPathLock(statemap.ResolveWriteTarget(statePath), func() error {
		if _, recErr := failurelog.Record(statePath, "", failurelog.RecordRequest{
			Cycle:          cycleID,
			Classification: string(failurelog.OperatorReset),
			Summary:        ev.Summary,
			Now:            t,
		}); recErr != nil && !errors.Is(recErr, failurelog.ErrStateMissing) {
			return fmt.Errorf("reset: record failed approach: %w", recErr)
		}

		// Advance lastCycleNumber (number never reused) + zero the batch accrual,
		// via a full-fidelity map so unmodelled fields survive. statemap owns the
		// single-source read/write primitives; we call the UNLOCKED read+write
		// here because the enclosing WithPathLock already holds the sidecar lock
		// across Record + this RMW (calling statemap.UpdateStateMap would re-lock
		// and deadlock).
		sm, err := statemap.ReadStateMap(statePath)
		if err != nil {
			return fmt.Errorf("reset: read state.json: %w", err)
		}
		sm["lastCycleNumber"] = cycleID
		cb, _ := sm["currentBatch"].(map[string]any)
		if cb == nil {
			cb = map[string]any{}
		}
		cb["cycleAccruedCostUSD"] = 0
		sm["currentBatch"] = cb
		sm["lastUpdated"] = rfc
		if err := statemap.WriteStateMap(statePath, sm); err != nil {
			return fmt.Errorf("reset: write state.json: %w", err)
		}
		return nil
	}); err != nil {
		return SealResult{}, err
	}

	// 5. Remove cycle-state.json — the abandon commit point.
	if err := os.Remove(csPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return SealResult{}, fmt.Errorf("reset: clear cycle-state: %w", err)
	}

	return res, nil
}

// pathWithin reports whether target is root or nested under it. Used to keep
// the destructive workspace rename inside the evolve/project root.
func pathWithin(target, root string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
