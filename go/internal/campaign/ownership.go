package campaign

// ownership.go — cross-session campaign ownership lease (ADR-0059). A campaign
// is keyed by goal hash, but its progress checkpoint lives under the running
// worktree's .evolve dir — so two autonomous sessions running the SAME plan
// from DIFFERENT worktrees never see each other and silently clobber, and a
// relaunch SIGTERM-reaps the incumbent. This lease moves the OWNERSHIP signal
// into a namespace every worktree shares (the git common dir), keyed by goal
// hash, guarded by a non-blocking flock: a second `campaign run` on the same
// goal hash learns who owns it and refuses (attach or stop-then-start) instead
// of clobbering. The flock auto-releases on holder death (even SIGKILL), so a
// dead owner's lease is immediately re-acquirable — no stale-PID heuristics.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// Owner identifies the process that holds a campaign's ownership lease. PID,
// Host, Worktree, and StartedAt are informational (for the refuse message and
// `campaign status`); the authoritative liveness signal is the flock itself.
type Owner struct {
	PID       int    `json:"pid"`
	GoalHash  string `json:"goal_hash"`
	Worktree  string `json:"worktree"`
	Host      string `json:"host"`
	StartedAt string `json:"started_at"`
}

// HeldError reports that another live process already owns the campaign's lease.
// Callers refuse-or-attach on it rather than clobbering the incumbent run.
type HeldError struct{ Owner Owner }

// Error renders a refuse message naming the live owner so an operator can attach
// (`evolve campaign status`) or stop it before relaunching. In the narrow window
// where the incumbent holds the flock but has not yet written its owner record,
// the owner reads back zero-valued; the message degrades to an honest "owner
// record not yet written" rather than printing "PID 0".
func (e *HeldError) Error() string {
	hash := e.Owner.GoalHash
	if len(hash) > 8 {
		hash = hash[:8]
	}
	owner := fmt.Sprintf("PID %d on %q since %s", e.Owner.PID, e.Owner.Worktree, e.Owner.StartedAt)
	if e.Owner.PID == 0 && e.Owner.Worktree == "" {
		owner = "another process (owner record not yet written)"
	}
	return fmt.Sprintf(
		"campaign %s already owned by %s — attach with `evolve campaign status` or stop it first",
		hash, owner)
}

// OwnershipLease is a held cross-session campaign ownership lease. Release frees
// it (the OS also frees it automatically when the holder process dies).
type OwnershipLease struct{ release func() }

// Release frees the ownership lease. Safe to call exactly once — defer it.
func (l *OwnershipLease) Release() {
	if l != nil && l.release != nil {
		l.release()
	}
}

// ownershipPath is the canonical lease data-file path for a goal hash under
// leaseDir (the git common dir shared by all worktrees of a repo). Unexported:
// the sidecar `.json`/`.json.lock` layout is an internal convention with no
// external consumer, so it stays free to change.
func ownershipPath(leaseDir, goalHash string) string {
	return filepath.Join(leaseDir, "campaign-lease-"+goalHash+".json")
}

// AcquireOwnership takes the exclusive cross-session lease for goalHash under
// leaseDir. It returns a *HeldError naming the live owner when another process
// holds it. On success it records self (stamping GoalHash) and returns a lease
// the caller must Release on exit.
func AcquireOwnership(leaseDir, goalHash string, self Owner) (*OwnershipLease, error) {
	path := ownershipPath(leaseDir, goalHash)
	release, held, err := flock.TryLock(path + ".lock")
	if err != nil {
		return nil, fmt.Errorf("campaign ownership: %w", err)
	}
	if held {
		owner, _ := ReadOwner(leaseDir, goalHash)
		return nil, &HeldError{Owner: owner}
	}
	self.GoalHash = goalHash
	if err := atomicwrite.JSON(path, self); err != nil {
		release()
		return nil, fmt.Errorf("campaign ownership: record owner: %w", err)
	}
	return &OwnershipLease{release: release}, nil
}

// ReadOwner returns the recorded owner for goalHash under leaseDir. It is
// best-effort display only — absent or unparsable yields ok=false, never an
// error — because the flock, not this file, is the authoritative liveness
// signal (a stale record after a clean release is only ever read while no live
// holder exists, where it is correctly ignored).
func ReadOwner(leaseDir, goalHash string) (Owner, bool) {
	raw, err := os.ReadFile(ownershipPath(leaseDir, goalHash))
	if err != nil {
		return Owner{}, false
	}
	var o Owner
	if json.Unmarshal(raw, &o) != nil {
		return Owner{}, false
	}
	return o, true
}
