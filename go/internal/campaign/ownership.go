package campaign

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

type Owner struct {
	PID       int    `json:"pid"`
	GoalHash  string `json:"goal_hash"`
	Worktree  string `json:"worktree"`
	Host      string `json:"host"`
	StartedAt string `json:"started_at"`
}

type HeldError struct{ Owner Owner }

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

type OwnershipLease struct{ release func() }

func (l *OwnershipLease) Release() {
	if l != nil && l.release != nil {
		l.release()
	}
}

func ownershipPath(leaseDir, goalHash string) string {
	return filepath.Join(leaseDir, "campaign-lease-"+goalHash+".json")
}

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
