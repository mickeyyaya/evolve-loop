// Package checkpoint writes the cycle checkpoint that lets an interrupted cycle resume.
// The checkpoint is an additive block inside .evolve/cycle-state.json.
// See docs/architecture/packages/internal-checkpoint.md.
package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseblock"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
)

type Reason string

const (
	ReasonQuotaLikely     Reason = "quota-likely"
	ReasonBatchCapNear    Reason = "batch-cap-near"
	ReasonOperatorRequest Reason = "operator-requested"
	ReasonStallInactivity Reason = "stall-inactivity"
	ReasonPhaseComplete   Reason = "phase-complete"
)

func (r Reason) IsValid() bool {
	switch r {
	case ReasonQuotaLikely, ReasonBatchCapNear, ReasonOperatorRequest, ReasonStallInactivity, ReasonPhaseComplete:
		return true
	}
	return false
}

type Checkpoint struct {
	Enabled               bool                `json:"enabled"`
	Reason                Reason              `json:"reason"`
	SavedAt               string              `json:"savedAt"`
	ResumeFromPhase       string              `json:"resumeFromPhase"`
	WorktreePath          string              `json:"worktreePath"`
	CompletedPhases       []string            `json:"completedPhases"`
	GitHead               string              `json:"gitHead"`
	CostAtCheckpoint      float64             `json:"costAtCheckpoint"`
	QuotaResetAt          string              `json:"quotaResetAt"`
	QuotaResetSource      string              `json:"quotaResetSource"`
	AutoResumeAttempts    int                 `json:"autoResumeAttempts"`
	AutoResumeMaxAttempts int                 `json:"autoResumeMaxAttempts"`
	OperatorAction        string              `json:"operatorAction,omitempty"`
	PhaseIntegrity        []phaseblock.Digest `json:"phaseIntegrity,omitempty"`
}

const DefaultAutoResumeAttempts = 3

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

func ComposeWithIntegrity(cs core.CycleState, reason Reason, cost float64, gitHead string, now time.Time, integrity []phaseblock.Digest) Checkpoint {
	cp := Compose(cs, reason, cost, gitHead, now)
	cp.PhaseIntegrity = integrity
	return cp
}

func ComposeChecked(cs core.CycleState, reason Reason, cost float64, gitHead string, now time.Time) (Checkpoint, error) {
	if !reason.IsValid() {
		return Checkpoint{}, fmt.Errorf("checkpoint: invalid reason %q (want quota-likely | batch-cap-near | operator-requested | stall-inactivity | phase-complete)", reason)
	}
	return Compose(cs, reason, cost, gitHead, now), nil
}

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
	core.ResumeBoundaryCheckpointer = func(cs core.CycleState, projectRoot string, now time.Time) error {
		path := core.ResolveCycleStatePath(filepath.Join(projectRoot, ".evolve"))
		return flock.WithPathLock(path, func() error {
			attempts, err := carriedAttempts(path)
			if err != nil {
				return err
			}
			cp := ComposeWithIntegrity(cs, ReasonOperatorRequest, 0, cycleBase(cs), now, readExistingIntegrity(path))
			cp.AutoResumeAttempts = attempts
			return applyWithHooks(defaultHooks(), path, cp)
		})
	}
	core.QuotaBoundaryCheckpointer = func(cs core.CycleState, projectRoot string, now time.Time) error {
		cycleStatePath := core.ResolveCycleStatePath(filepath.Join(projectRoot, ".evolve"))
		return flock.WithPathLock(cycleStatePath, func() error {
			existing := readExistingIntegrity(cycleStatePath)
			cp := withQuotaReset(ComposeWithIntegrity(cs, ReasonQuotaLikely, 0, cycleBase(cs), now, existing), cs, projectRoot, now)
			return applyQuotaPause(cycleStatePath, cp)
		})
	}
	core.PhaseBoundaryCheckpointer = func(cs core.CycleState, projectRoot string, now time.Time) error {
		cycleStatePath := core.ResolveCycleStatePath(filepath.Join(projectRoot, ".evolve"))
		return flock.WithPathLock(cycleStatePath, func() error {
			yield, err := hasEscalationCheckpoint(cycleStatePath)
			if err != nil {
				return err
			}
			if yield {
				return nil
			}
			existing := readExistingIntegrity(cycleStatePath)
			return applyWithHooks(defaultHooks(), cycleStatePath, ComposeWithIntegrity(cs, ReasonPhaseComplete, 0, cycleBase(cs), now, existing))
		})
	}
}

func cycleBase(cs core.CycleState) string {
	if cs.WorktreeBaseSHA != "" {
		return cs.WorktreeBaseSHA
	}
	return cs.PreCycleHEAD
}

var computeQuotaReset = quotareset.Compute

var UsageReset func(projectRoot string, families []string, now time.Time) (time.Time, bool)

func withQuotaReset(cp Checkpoint, cs core.CycleState, projectRoot string, now time.Time) Checkpoint {
	qr := policy.Policy{}.QuotaResetConfig()
	if pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json")); err == nil {
		qr = pol.QuotaResetConfig()
	}
	families := walkedFamilies(cs.QuotaWalkCLIs)
	res, err := computeQuotaReset(cs.WorkspacePath, quotareset.Options{
		Now:          func() time.Time { return now },
		ResetAt:      qr.ResetAt,
		DefaultHours: qr.DefaultHours,
		BenchedUntil: earliestQuotaReset(projectRoot, now, families),
		UsageReset:   usageResetOf(projectRoot, families, now),
	})
	switch {
	case err != nil:
		cp.QuotaResetSource = "unavailable"
	case res.Source == unknownResetSource:
		cp.QuotaResetSource = res.Source
		cp.AutoResumeMaxAttempts = 0
		cp.OperatorAction = unknownResetAction
	default:
		cp.QuotaResetAt = res.ISO
		cp.QuotaResetSource = res.Source
	}
	return cp
}

const (
	unknownResetSource = "unknown"
	unknownResetAction = "no quota reset time is known: check the usage of the walled CLIs, then run `evolve loop --resume` by hand"
)

func walkedFamilies(clis []string) []string {
	families := make([]string, 0, len(clis))
	for _, cli := range clis {
		if family := llmroute.Family(cli); !slices.Contains(families, family) {
			families = append(families, family)
		}
	}
	return families
}

func earliestQuotaReset(projectRoot string, now time.Time, families []string) time.Time {
	var earliest time.Time
	for _, entry := range clihealth.NewStore(projectRoot, func() time.Time { return now }).Active() {
		if !clihealth.QuotaPattern(entry.Reason) || !slices.Contains(families, entry.Family) {
			continue
		}
		if earliest.IsZero() || entry.BenchedUntil.Before(earliest) {
			earliest = entry.BenchedUntil
		}
	}
	return earliest
}

func usageResetOf(projectRoot string, families []string, now time.Time) func() (time.Time, bool) {
	if UsageReset == nil || len(families) == 0 {
		return nil
	}
	return func() (time.Time, bool) { return UsageReset(projectRoot, families, now) }
}

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
