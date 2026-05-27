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
)

// reset.go — the complement of resume.go. Where resume CONTINUES a
// checkpointed cycle, SealCycle ABANDONS a stuck/unfinished cycle while
// preserving its full history for later analysis:
//
//   - the workspace is moved into a sibling archive <workspace>.reset-<UTCnano>/
//     (never deleted), alongside a verbatim cycle-state snapshot + a
//     reset-manifest.json describing why/when/what was sealed;
//   - lastCycleNumber is advanced to the sealed cycle's number so the number
//     is never reused (the next cycle is N+1);
//   - an auditable, hash-chained ledger entry is appended (cycle:0 +
//     cycle_label); and
//   - cycle-state.json is removed — the abandon commit point that disarms the
//     phase-gate precondition and lets a fresh cycle start clean.
//
// state.json is mutated through a full-fidelity map (not the typed core.State,
// which would drop unmodelled fields like expected_ship_sha on round-trip —
// the same trap ship/statefile.go documents).

// ErrNothingToReset is returned when there is no in-progress cycle to seal
// (no cycle-state.json, or cycle_id == 0).
var ErrNothingToReset = errors.New("reset: no in-progress cycle to seal")

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
}

// SealResult reports what was (or, in dry-run, would be) sealed.
type SealResult struct {
	SealedCycleID int
	SealedPhase   string
	Workspace     string
	ArchiveDir    string
	NextCycle     int
	DryRun        bool
}

// SealCycle seals the in-progress cycle described by
// <EvolveDir>/cycle-state.json. See the package comment for the contract.
func SealCycle(ctx context.Context, ledger ledgerAppender, opts SealOptions) (SealResult, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	gitHead := opts.GitHead
	if gitHead == nil {
		gitHead = defaultCurrentHead
	}

	csPath := filepath.Join(opts.EvolveDir, "cycle-state.json")
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

	// 3. Auditable ledger entry (append-only, hash-chained).
	if ledger != nil {
		entry := LedgerEntry{
			TS:           rfc,
			Cycle:        0,
			CycleLabel:   fmt.Sprintf("reset-seal-cycle-%d", cycleID),
			Role:         "operator",
			Kind:         "reset",
			GitHEAD:      head,
			ArtifactPath: archiveDir,
		}
		if err := ledger.Append(ctx, entry); err != nil {
			return SealResult{}, fmt.Errorf("reset: append ledger: %w", err)
		}
	}

	// 4. Advance lastCycleNumber (number never reused) + zero the batch accrual,
	//    via a full-fidelity map so unmodelled fields survive.
	statePath := filepath.Join(opts.EvolveDir, "state.json")
	sm, err := readJSONMapFile(statePath)
	if err != nil {
		return SealResult{}, fmt.Errorf("reset: read state.json: %w", err)
	}
	sm["lastCycleNumber"] = cycleID
	cb, _ := sm["currentBatch"].(map[string]any)
	if cb == nil {
		cb = map[string]any{}
	}
	cb["cycleAccruedCostUSD"] = 0
	sm["currentBatch"] = cb
	sm["lastUpdated"] = rfc
	if err := writeJSONMapFileAtomic(statePath, sm); err != nil {
		return SealResult{}, fmt.Errorf("reset: write state.json: %w", err)
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

// readJSONMapFile parses path as a JSON object. Missing/empty → empty map.
// Full-fidelity counterpart to the typed storage adapter (preserves unmodelled
// fields like expected_ship_sha); mirrors ship/statefile.go:readStateMap.
func readJSONMapFile(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// writeJSONMapFileAtomic writes m as indented JSON to path via tmp + rename.
func writeJSONMapFileAtomic(path string, m map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(buf); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
