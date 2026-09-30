package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	hostStateDir                  = ".evolve"
	porcelainStatusPrefix         = len("XY ")
	worktreeEvidenceReportedPaths = 8
)

type worktreeStamp struct {
	present bool
	size    int64
	mtimeNS int64
}

type worktreeSnapshot struct {
	captured bool
	entries  map[string]worktreeStamp
}

func captureWorktreeEvidenceBaseline(ctx context.Context, cfg *Config, deps Deps) worktreeSnapshot {
	if cfg.Completion != completionWorktreeEvidence || cfg.Worktree == "" {
		return worktreeSnapshot{}
	}
	return captureWorktreeSnapshot(ctx, cfg, deps)
}

func captureWorktreeSnapshot(ctx context.Context, cfg *Config, deps Deps) worktreeSnapshot {
	paths, err := worktreeStatusPaths(ctx, cfg.Worktree, deps)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN: worktree snapshot failed in %s: %v — this dispatch completes only on a rewritten deliverable\n", cfg.Worktree, err)
		return worktreeSnapshot{}
	}
	entries := make(map[string]worktreeStamp, len(paths))
	for _, rel := range paths {
		if !hostOwned(cfg, rel) {
			entries[rel] = stampOf(filepath.Join(cfg.Worktree, rel))
		}
	}
	return worktreeSnapshot{captured: true, entries: entries}
}

func worktreeStatusPaths(ctx context.Context, worktree string, deps Deps) ([]string, error) {
	var out, errOut strings.Builder
	args := []string{"--no-optional-locks", "-C", worktree, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames"}
	code, err := deps.Runner(ctx, "git", "", args, driverEnv(deps, nil), nil, &out, &errOut)
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	if code != 0 {
		return nil, fmt.Errorf("git status exit %d: %s", code, strings.TrimSpace(errOut.String()))
	}
	var paths []string
	for _, record := range strings.Split(out.String(), "\x00") {
		if len(record) > porcelainStatusPrefix {
			paths = append(paths, record[porcelainStatusPrefix:])
		}
	}
	return paths, nil
}

func hostOwned(cfg *Config, rel string) bool {
	if rel == hostStateDir || strings.HasPrefix(rel, hostStateDir+"/") {
		return true
	}
	abs := filepath.Join(cfg.Worktree, rel)
	return withinDir(cfg.Workspace, abs) || slices.Contains(artifactCandidatePaths(cfg), abs)
}

func withinDir(dir, path string) bool {
	if dir == "" {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func stampOf(path string) worktreeStamp {
	fi, err := os.Lstat(path)
	if err != nil {
		return worktreeStamp{}
	}
	return worktreeStamp{present: true, size: fi.Size(), mtimeNS: fi.ModTime().UnixNano()}
}

func (s worktreeSnapshot) changedSince(before worktreeSnapshot) []string {
	var changed []string
	for rel, stamp := range s.entries {
		if prior, seen := before.entries[rel]; !seen || prior != stamp {
			changed = append(changed, rel)
		}
	}
	for rel := range before.entries {
		if _, still := s.entries[rel]; !still {
			changed = append(changed, rel)
		}
	}
	sort.Strings(changed)
	return changed
}

type idleCompleter interface {
	completeOnIdle(ctx context.Context) (bool, completionEvidence, string, error)
}

type worktreeEvidenceDetector struct {
	*artifactDetector
	deps Deps
}

func (d *worktreeEvidenceDetector) completeOnIdle(ctx context.Context) (bool, completionEvidence, string, error) {
	if !d.deliverableUnrewritten() || !d.baseline.worktree.captured || d.missingSecondary() != "" {
		return false, completionEvidence{}, "", nil
	}
	now := captureWorktreeSnapshot(ctx, d.cfg, d.deps)
	changed := now.changedSince(d.baseline.worktree)
	if !now.captured || len(changed) == 0 {
		return false, completionEvidence{}, "", nil
	}
	ready, relocatedFrom, err := artifactCanonicalize(d.cfg, relocateFile)
	if err != nil || !ready {
		return false, completionEvidence{}, "", err
	}
	evidence := completionEvidence{CarriedDeliverable: d.cfg.Artifact, WorktreeChanges: changed}
	return true, evidence, worktreeEvidenceNote(evidence, relocatedFrom), nil
}

func (d *worktreeEvidenceDetector) deliverableUnrewritten() bool {
	path, found := artifactLocate(d.cfg)
	if !found {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && d.baseline.matches(path, fi)
}

func worktreeEvidenceNote(evidence completionEvidence, relocatedFrom string) string {
	note := fmt.Sprintf("worktree-evidence: %s was not rewritten during this re-dispatch, and the idle agent changed %d worktree path(s) since dispatch (%s) — completing on the carried deliverable for the host's phase verify",
		evidence.CarriedDeliverable, len(evidence.WorktreeChanges), reportedPaths(evidence.WorktreeChanges))
	if relocatedFrom != "" {
		note += "; relocated from non-canonical " + relocatedFrom
	}
	return note
}

func reportedPaths(paths []string) string {
	if len(paths) <= worktreeEvidenceReportedPaths {
		return strings.Join(paths, ",")
	}
	more := len(paths) - worktreeEvidenceReportedPaths
	return strings.Join(paths[:worktreeEvidenceReportedPaths], ",") + ",+" + strconv.Itoa(more) + " more"
}

func worktreeEvidenceEvent(cfg *Config, evidence completionEvidence, note string) signalcenter.Event {
	id := configIdentity(cfg)
	return signalcenter.Event{
		Cycle: id.cycle, RunID: id.runID, Phase: id.phase, Module: signalcenter.ModuleBridge, Origin: "replWaiter.completeOnIdle",
		Kind: signalcenter.KindBridgeWarning, Severity: signalcenter.SeverityWarn, Code: CodeCompletedOnWorktreeEvidence,
		Reason: note,
		Fields: map[string]string{
			"deliverable":   evidence.CarriedDeliverable,
			"changed_paths": strconv.Itoa(len(evidence.WorktreeChanges)),
			"paths":         reportedPaths(evidence.WorktreeChanges),
			"cli":           cfg.CLI,
		},
	}
}
