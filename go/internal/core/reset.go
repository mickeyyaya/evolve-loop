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

var ErrNothingToReset = errors.New("reset: no in-progress cycle to seal")

var ErrCycleOwnedLive = errors.New("reset: cycle is owned by a live run (fresh lease) — refusing to seal without --force")

type ledgerAppender interface {
	Append(ctx context.Context, entry LedgerEntry) error
}

type SealOptions struct {
	EvolveDir         string
	ProjectRoot       string
	Reason            string
	DryRun            bool
	Now               func() time.Time
	GitHead           func(projectRoot string) (string, error)
	Force             bool
	LeaseTTL          time.Duration
	PidAlive          func(pid int) bool
	AutomatedRecovery bool
	seams             sealSeams
}

type sealSeams struct {
	marshalManifest func(v any, prefix, indent string) ([]byte, error)
	readState       func(path string) (map[string]any, error)
	writeState      func(path string, m map[string]any) error
}

func (s sealSeams) withDefaults() sealSeams {
	if s.marshalManifest == nil {
		s.marshalManifest = json.MarshalIndent
	}
	if s.readState == nil {
		s.readState = statemap.ReadStateMap
	}
	if s.writeState == nil {
		s.writeState = statemap.WriteStateMap
	}
	return s
}

const autosealRole = "operator-autoseal"

type SealResult struct {
	SealedCycleID       int
	SealedPhase         string
	Workspace           string
	ArchiveDir          string
	NextCycle           int
	DryRun              bool
	ForcedOverLiveOwner bool
	LeaseOwnerPID       int
	LeaseHeartbeatAge   time.Duration
}

func SealCycle(ctx context.Context, ledger ledgerAppender, opts SealOptions) (SealResult, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	seams := opts.seams.withDefaults()
	gitHead := opts.GitHead
	if gitHead == nil {
		gitHead = defaultCurrentHead
	}

	csPath := ResolveCycleStatePath(opts.EvolveDir)
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

	if lease, ok, _ := runlease.Read(workspace); ok && runlease.OwnerLive(lease, t, opts.LeaseTTL, opts.PidAlive) {
		res.LeaseOwnerPID = lease.OwnerPID
		if hb, perr := time.Parse(time.RFC3339Nano, lease.HeartbeatAt); perr == nil {
			res.LeaseHeartbeatAge = t.Sub(hb)
		}
		switch {
		case opts.DryRun:
		case opts.Force:
			res.ForcedOverLiveOwner = true
		default:
			return res, ErrCycleOwnedLive
		}
	}

	if opts.DryRun {
		return res, nil
	}

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
	manRaw, err := seams.marshalManifest(manifest, "", "  ")
	if err != nil {
		return SealResult{}, fmt.Errorf("reset: marshal manifest: %w", err)
	}
	manRaw = append(manRaw, '\n')

	ev := faillearn.FailureEvent{
		Cycle:          cycleID,
		FailedPhase:    phase,
		Scope:          faillearn.ScopeReset,
		Classification: string(failurelog.OperatorReset),
		Verdict:        "RESET",
		Summary:        fmt.Sprintf("operator reset of cycle %d at phase %s: %s", cycleID, phase, opts.Reason),
		EvidencePaths:  []string{archiveDir},
		GitHead:        head,
		Now:            t,
	}
	lessonsDir := filepath.Join(opts.EvolveDir, "instincts", "lessons")

	writeMeta := func(dir string) error {
		if err := os.WriteFile(filepath.Join(dir, "cycle-state.snapshot.json"), raw, 0o644); err != nil {
			return fmt.Errorf("reset: write snapshot: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "reset-manifest.json"), manRaw, 0o644); err != nil {
			return fmt.Errorf("reset: write manifest: %w", err)
		}
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

		sm, err := seams.readState(statePath)
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
		if err := seams.writeState(statePath, sm); err != nil {
			return fmt.Errorf("reset: write state.json: %w", err)
		}
		return nil
	}); err != nil {
		return SealResult{}, err
	}

	if err := os.Remove(csPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return SealResult{}, fmt.Errorf("reset: clear cycle-state: %w", err)
	}

	return res, nil
}

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
