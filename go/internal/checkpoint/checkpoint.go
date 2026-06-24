// Package checkpoint ports the pre-emptive cycle-checkpoint logic from
// scripts/lifecycle/cycle-state.sh:cycle_state_checkpoint.
//
// The on-disk shape is an additive "checkpoint" block inside
// .evolve/cycle-state.json (not a separate file, despite older docs).
// The block schema mirrors bash exactly so `evolve cycle resume` and
// `bash scripts/dispatch/resume-cycle.sh` consume the same data.
//
// Checkpoints are written for the escalation reasons (quota-likely,
// batch-cap-near, operator-requested, stall-inactivity) plus the
// lowest-priority phase-complete boundary. The former cost-percentage
// trigger (EVOLVE_CHECKPOINT_*_PCT) was removed with the token-budget
// cost gates; cost is no longer a checkpoint signal.
package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Reason mirrors the four bash-canonical checkpoint reasons.
type Reason string

const (
	ReasonQuotaLikely     Reason = "quota-likely"
	ReasonBatchCapNear    Reason = "batch-cap-near"
	ReasonOperatorRequest Reason = "operator-requested"
	ReasonStallInactivity Reason = "stall-inactivity"
	ReasonPhaseComplete   Reason = "phase-complete"
)

// IsValid reports whether r is one of the five canonical reasons.
func (r Reason) IsValid() bool {
	switch r {
	case ReasonQuotaLikely, ReasonBatchCapNear, ReasonOperatorRequest, ReasonStallInactivity, ReasonPhaseComplete:
		return true
	}
	return false
}

// Checkpoint mirrors the bash jq spec at cycle-state.sh:511-524.
type Checkpoint struct {
	Enabled               bool     `json:"enabled"`
	Reason                Reason   `json:"reason"`
	SavedAt               string   `json:"savedAt"`
	ResumeFromPhase       string   `json:"resumeFromPhase"`
	WorktreePath          string   `json:"worktreePath"`
	CompletedPhases       []string `json:"completedPhases"`
	GitHead               string   `json:"gitHead"`
	CostAtCheckpoint      float64  `json:"costAtCheckpoint"`
	QuotaResetAt          string   `json:"quotaResetAt"`
	QuotaResetSource      string   `json:"quotaResetSource"`
	AutoResumeAttempts    int      `json:"autoResumeAttempts"`
	AutoResumeMaxAttempts int      `json:"autoResumeMaxAttempts"`
}

// DefaultAutoResumeAttempts is the default cap on automatic resume attempts
// after a checkpoint (used by Compose).
const DefaultAutoResumeAttempts = 3

// Compose builds a Checkpoint block from a CycleState + the
// orchestrator-supplied reason/cost/gitHead/now. Pure; no I/O.
// Does NOT validate the reason — use ComposeChecked when the reason
// origin is untrusted.
func Compose(cs core.CycleState, reason Reason, cost float64, gitHead string, now time.Time) Checkpoint {
	completed := cs.CompletedPhases
	if completed == nil {
		completed = []string{}
	}
	return Checkpoint{
		Enabled:               true,
		Reason:                reason,
		SavedAt:               now.UTC().Format(time.RFC3339),
		ResumeFromPhase:       cs.Phase,
		WorktreePath:          cs.ActiveWorktree,
		CompletedPhases:       completed,
		GitHead:               gitHead,
		CostAtCheckpoint:      cost,
		AutoResumeAttempts:    0,
		AutoResumeMaxAttempts: DefaultAutoResumeAttempts,
	}
}

// ComposeChecked is Compose with reason validation. Returns an error
// if reason is not one of the four bash-canonical values.
func ComposeChecked(cs core.CycleState, reason Reason, cost float64, gitHead string, now time.Time) (Checkpoint, error) {
	if !reason.IsValid() {
		return Checkpoint{}, fmt.Errorf("checkpoint: invalid reason %q (want quota-likely | batch-cap-near | operator-requested | stall-inactivity | phase-complete)", reason)
	}
	return Compose(cs, reason, cost, gitHead, now), nil
}

// hooks bundles every I/O + serialization primitive ApplyToStateFile
// touches so tests can drive each error branch independently. The
// production zero value (defaultHooks) wires to the stdlib.
type hooks struct {
	readFile      func(string) ([]byte, error)
	writeFile     func(string, []byte, os.FileMode) error
	rename        func(string, string) error
	remove        func(string) error
	jsonMarshal   func(any) ([]byte, error)
	jsonUnmarshal func([]byte, any) error
}

func defaultHooks() hooks {
	return hooks{
		readFile:      os.ReadFile,
		writeFile:     os.WriteFile,
		rename:        os.Rename,
		remove:        os.Remove,
		jsonMarshal:   json.Marshal,
		jsonUnmarshal: json.Unmarshal,
	}
}

// ApplyToStateFile reads a cycle-state.json, splices the checkpoint
// block into it (preserving every existing field), and atomically
// writes back. Mirrors bash cycle_state_checkpoint at cycle-state.sh:479.
//
// ADR-0049 G7: the read-modify-write holds the SAME cycle-state.json sidecar
// lock storage.WriteCycleState holds, so a concurrent fleet cycle's phase write
// (which owns "phase" and preserves "checkpoint") and this checkpoint write
// (which owns "checkpoint" and preserves "phase") serialize instead of clobbering
// each other's key. flock.WithPathLock is the single home for the "<file>.lock"
// convention shared across packages.
func ApplyToStateFile(path string, cp Checkpoint) error {
	return flock.WithPathLock(path, func() error {
		return applyWithHooks(defaultHooks(), path, cp)
	})
}

func applyWithHooks(h hooks, path string, cp Checkpoint) error {
	b, err := h.readFile(path)
	if err != nil {
		return fmt.Errorf("checkpoint: read state: %w", err)
	}
	var state map[string]any
	if err := h.jsonUnmarshal(b, &state); err != nil {
		return fmt.Errorf("checkpoint: parse state: %w", err)
	}
	cpBytes, err := h.jsonMarshal(cp)
	if err != nil {
		return fmt.Errorf("checkpoint: marshal block: %w", err)
	}
	var cpMap map[string]any
	if err := h.jsonUnmarshal(cpBytes, &cpMap); err != nil {
		return fmt.Errorf("checkpoint: re-parse block: %w", err)
	}
	state["checkpoint"] = cpMap
	out, err := h.jsonMarshal(state)
	if err != nil {
		return fmt.Errorf("checkpoint: marshal state: %w", err)
	}
	tmp := path + ".tmp"
	if err := h.writeFile(tmp, out, 0o644); err != nil {
		return fmt.Errorf("checkpoint: write tmp: %w", err)
	}
	if err := h.rename(tmp, path); err != nil {
		_ = h.remove(tmp)
		return fmt.Errorf("checkpoint: rename: %w", err)
	}
	return nil
}

func init() {
	core.PhaseBoundaryCheckpointer = func(cs core.CycleState, projectRoot string, now time.Time) error {
		cycleStatePath := filepath.Join(projectRoot, ".evolve", "cycle-state.json")
		// ADR-0049 N17: check-and-act under ONE lock. The escalation guard read
		// used to be UNLOCKED and the phase-complete write took the cycle-state
		// lock SEPARATELY — a TOCTOU. Under fleet mode concurrent cycles share the
		// host-global cycle-state.json, so a peer could write an escalation
		// checkpoint in the window between our read and our write, which this
		// lowest-priority phase-complete write then clobbered (lost escalation).
		// Folding the read + write into a single flock.WithPathLock critical
		// section closes the window: both serialized orders converge on the
		// escalation. applyWithHooks (the lock-free inner of ApplyToStateFile) is
		// called DIRECTLY — flock is non-reentrant, so re-calling ApplyToStateFile
		// here would self-deadlock on the same sidecar lock.
		return flock.WithPathLock(cycleStatePath, func() error {
			yield, err := hasEscalationCheckpoint(cycleStatePath)
			if err != nil {
				return err
			}
			if yield {
				// phase-complete is the lowest-priority reason: never clobber an
				// escalation checkpoint (quota-likely, batch-cap-near,
				// operator-requested, stall-inactivity) — those must survive until
				// their consumer (e.g. detectQuotaPause after RunCycle) reads them.
				return nil
			}
			return applyWithHooks(defaultHooks(), cycleStatePath, Compose(cs, ReasonPhaseComplete, 0, "", now))
		})
	}
}

// hasEscalationCheckpoint reports whether path already holds a checkpoint
// block with a canonical reason other than phase-complete. It reads WITHOUT
// taking the cycle-state lock, so the caller MUST already hold
// flock.WithPathLock(path) (ADR-0049 N17) — otherwise the check and the
// caller's subsequent write straddle the lock and a peer escalation is lost.
func hasEscalationCheckpoint(path string) (bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("checkpoint: read state: %w", err)
	}
	var state struct {
		Checkpoint *struct {
			Reason Reason `json:"reason"`
		} `json:"checkpoint"`
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return false, fmt.Errorf("checkpoint: parse state: %w", err)
	}
	if state.Checkpoint == nil {
		return false, nil
	}
	r := state.Checkpoint.Reason
	return r.IsValid() && r != ReasonPhaseComplete, nil
}
