package gc

// worktrees.go — Slice S4 of docs/plans/workspace-hygiene-2026-07.md: a
// gc-sibling Plan/Apply planner that drains the ACCUMULATED worktree+branch
// backlog (65 worktrees / 106 never-deleted cycle-* branches at audit time)
// that S1 (pid-aware lease staleness) and S3 (in-cycle branch deletion) do not
// retroactively clean up. It mirrors gc.go's Plan/Apply shape: PlanWorktrees is
// the pure dry-run (never mutates), ApplyWorktrees executes under
// .evolve/ship.lock with a per-item TOCTOU re-check.
//
// Evidence pipeline (evidence-based, never name-parsed):
//   - `git worktree list --porcelain` (in ProjectRoot) is the source of truth
//     for registered worktrees and their branch (the "branch refs/heads/<name>"
//     line, NOT the directory leaf).
//   - a candidate is a worktree whose directory is a direct child of
//     WorktreeBase AND whose leaf carries the "cycle-" prefix.
//   - merged  = the branch is listed by `git branch --merged HEAD`.
//   - dirty   = `git status --porcelain` (run IN the worktree dir) is non-empty.
//   - live    = ANY of: a fresh runlease.OwnerLive lease at
//     <EvolveDir>/runs/cycle-<N>/.lease (N = the leaf's trailing numeric
//     segment after stripping a "-integration" / "-w<digits>" swarm suffix);
//     <EvolveDir>/cycle-state.json's active_worktree equals the path; any
//     <EvolveDir>/runs/*/run.json's active_worktree equals the path.
//   - a cycle-* branch with NO worktree entry (the literal worktree/branch skew)
//     is swept purely on the merged/unmerged split — no liveness check, nothing
//     to remove, delete-branch (merged) or flag-unmerged (unmerged) only.
//
// Safety invariants (also pinned by worktrees_fuzz_test.go against random
// populations): a live, dirty, or unmerged candidate is NEVER removed and its
// branch is NEVER deleted.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// WorktreesPolicy is the retention grace on top of the merged/clean/dead gate,
// embedded in gc.Policy (S5 wiring reads it from policy.json:gc.worktrees).
type WorktreesPolicy struct {
	// KeepRecent: among fully-eligible (merged, clean, dead) candidates, the
	// newest N by mtime are always kept, mirroring RunsPolicy.KeepFull.
	KeepRecent int `json:"keep_recent,omitempty"`
	// MinAgeMinutes: a candidate younger than this is never touched — the grace
	// window that covers the create -> lease-write race.
	MinAgeMinutes int `json:"min_age_minutes,omitempty"`
}

// WorktreeAction is the disposition PlanWorktrees assigns each candidate.
type WorktreeAction string

const (
	// WorktreeActionRemove removes the (merged, clean, dead) worktree dir via a
	// non-force `git worktree remove`.
	WorktreeActionRemove WorktreeAction = "remove"
	// WorktreeActionDeleteBranch deletes the merged branch via `git branch -d`
	// (never -D).
	WorktreeActionDeleteBranch WorktreeAction = "delete-branch"
	// WorktreeActionFlagDirty flags a dirty worktree for manual review; it is
	// never removed (preserves ship-fail evidence).
	WorktreeActionFlagDirty WorktreeAction = "flag-dirty"
	// WorktreeActionFlagUnmerged flags an unmerged branch; it is never deleted.
	WorktreeActionFlagUnmerged WorktreeAction = "flag-unmerged"
)

// WorktreeItem is one planned action. Path is empty for a branch-only backlog
// entry (no worktree dir to touch).
type WorktreeItem struct {
	Path   string         `json:"path,omitempty"`
	Branch string         `json:"branch,omitempty"`
	Action WorktreeAction `json:"action"`
	Reason string         `json:"reason,omitempty"`
}

// WorktreeManifest is the full plan, sorted deterministically so shadow-soak
// diffs are stable.
type WorktreeManifest struct {
	Items []WorktreeItem `json:"items"`
}

// WorktreeOptions drives PlanWorktrees/ApplyWorktrees. Exec is the injected git
// runner (production: sysexec.DefaultRunner); PidAlive/Now/LeaseTTL are the
// liveness seams shared with runlease.
type WorktreeOptions struct {
	ProjectRoot  string
	WorktreeBase string
	EvolveDir    string
	Policy       WorktreesPolicy
	Now          func() time.Time
	Exec         sysexec.RunFunc
	PidAlive     func(int) bool
	LeaseTTL     time.Duration
}

var swarmSuffixRe = regexp.MustCompile(`-(integration|w\d+)$`)

func (o WorktreeOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// git runs a git subcommand in dir and returns captured stdout. A non-zero exit
// or a runner error is surfaced (git's own dirty/merged safety checks live in
// those exit codes).
func (o WorktreeOptions) git(dir string, args ...string) (string, error) {
	var out strings.Builder
	code, err := o.Exec(context.Background(), "git", dir, args, nil, nil, &out, nil)
	if err != nil {
		return out.String(), fmt.Errorf("gc: git %s: %w", strings.Join(args, " "), err)
	}
	if code != 0 {
		return out.String(), fmt.Errorf("gc: git %s: exit %d", strings.Join(args, " "), code)
	}
	return out.String(), nil
}

type worktreeEntry struct {
	path   string
	branch string // "" => detached (never a candidate)
}

func parseWorktreePorcelain(s string) []worktreeEntry {
	var out []worktreeEntry
	var cur worktreeEntry
	flush := func() {
		if cur.path != "" {
			out = append(out, cur)
		}
		cur = worktreeEntry{}
	}
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur.path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.HasPrefix(line, "branch refs/heads/"):
			cur.branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "detached":
			cur.branch = ""
		}
	}
	flush()
	return out
}

// parseBranchList normalizes `git branch` output: strips the "* " current and
// "+ " worktree-checkout markers and skips "(HEAD detached...)" pseudo-entries.
func parseBranchList(s string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "+ ")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "(") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// leafCycleNumber extracts the trailing cycle number from a worktree leaf after
// stripping a swarm suffix. cycle-aaa1111-570 -> 570;
// cycle-legacyB-8-integration -> 8; cycle-legacyC-9-w0 -> 9.
func leafCycleNumber(leaf string) (int, bool) {
	base := swarmSuffixRe.ReplaceAllString(leaf, "")
	idx := strings.LastIndex(base, "-")
	if idx < 0 || idx == len(base)-1 {
		return 0, false
	}
	n, err := strconv.Atoi(base[idx+1:])
	if err != nil {
		return 0, false
	}
	return n, true
}

// resolvePath resolves symlinks so a comparison survives macOS's
// /var -> /private/var divergence (real `git worktree list` reports the
// resolved path; the caller passes the unresolved base). Falls back to a
// lexical clean when the path does not exist on disk.
func resolvePath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// underBase reports whether path is a direct child of base (the worktree leaf
// layout gitWorktree.Create always produces), comparing resolved parents.
func underBase(base, path string) bool {
	return resolvePath(filepath.Dir(filepath.Clean(path))) == resolvePath(base)
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

// activeWorktreeMatches reports whether the JSON file at jsonPath carries an
// "active_worktree" equal to wtPath (fleet cycle-state / per-run mirrors).
func activeWorktreeMatches(jsonPath, wtPath string) bool {
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return false
	}
	var m struct {
		ActiveWorktree string `json:"active_worktree"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	return m.ActiveWorktree != "" && samePath(m.ActiveWorktree, wtPath)
}

// isLive proves a worktree is genuinely in-flight via any of the three
// evidence sources; used both by Plan and by Apply's TOCTOU re-check.
func (o WorktreeOptions) isLive(path string) bool {
	if activeWorktreeMatches(filepath.Join(o.EvolveDir, "cycle-state.json"), path) {
		return true
	}
	runsDir := filepath.Join(o.EvolveDir, "runs")
	if entries, err := os.ReadDir(runsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if activeWorktreeMatches(filepath.Join(runsDir, e.Name(), "run.json"), path) {
				return true
			}
		}
	}
	if n, ok := leafCycleNumber(filepath.Base(path)); ok {
		runDir := filepath.Join(o.EvolveDir, "runs", fmt.Sprintf("cycle-%d", n))
		if l, present, err := runlease.Read(runDir); err == nil && present {
			if runlease.OwnerLive(l, o.now(), o.LeaseTTL, o.PidAlive) {
				return true
			}
		}
	}
	return false
}

func (o WorktreeOptions) isDirty(path string) (bool, error) {
	out, err := o.git(path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// PlanWorktrees evaluates the worktree+branch backlog and returns the action
// manifest. It never mutates the tree — it IS the dry-run.
func PlanWorktrees(o WorktreeOptions) (WorktreeManifest, error) {
	if o.Exec == nil {
		return WorktreeManifest{}, errors.New("gc: PlanWorktrees requires Exec")
	}
	porcelain, err := o.git(o.ProjectRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return WorktreeManifest{}, err
	}
	mergedOut, err := o.git(o.ProjectRoot, "branch", "--merged", "HEAD")
	if err != nil {
		return WorktreeManifest{}, err
	}
	merged := map[string]bool{}
	for _, b := range parseBranchList(mergedOut) {
		merged[b] = true
	}

	minAge := time.Duration(o.Policy.MinAgeMinutes) * time.Minute
	now := o.now()

	var items []WorktreeItem
	seenBranch := map[string]bool{}

	type eligible struct {
		path, branch string
		mtime        time.Time
	}
	var pool []eligible

	for _, e := range parseWorktreePorcelain(porcelain) {
		if e.branch == "" || !underBase(o.WorktreeBase, e.path) {
			continue
		}
		leaf := filepath.Base(e.path)
		if !strings.HasPrefix(leaf, "cycle-") {
			continue
		}
		// Emit the caller-relative path (WorktreeBase/leaf), not git's
		// symlink-resolved porcelain path, so callers match items against the
		// paths they know. git resolves it again for status/remove.
		path := filepath.Join(o.WorktreeBase, leaf)
		seenBranch[e.branch] = true

		if o.isLive(path) {
			continue // a live lease excludes the worktree entirely
		}
		dirty, err := o.isDirty(path)
		if err != nil {
			return WorktreeManifest{}, err
		}
		if dirty {
			items = append(items, WorktreeItem{Path: path, Branch: e.branch, Action: WorktreeActionFlagDirty, Reason: "dirty worktree — preserved for manual review"})
			continue
		}
		if !merged[e.branch] {
			items = append(items, WorktreeItem{Path: path, Branch: e.branch, Action: WorktreeActionFlagUnmerged, Reason: "branch not merged into HEAD"})
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if minAge > 0 && now.Sub(info.ModTime()) < minAge {
			continue // MinAgeMinutes grace
		}
		pool = append(pool, eligible{path: path, branch: e.branch, mtime: info.ModTime()})
	}

	// KeepRecent: retain the newest N eligible candidates by mtime.
	sort.Slice(pool, func(i, j int) bool { return pool[i].mtime.After(pool[j].mtime) })
	for i, c := range pool {
		if i < o.Policy.KeepRecent {
			continue
		}
		items = append(items,
			WorktreeItem{Path: c.path, Branch: c.branch, Action: WorktreeActionRemove, Reason: "merged, clean, dead"},
			WorktreeItem{Path: c.path, Branch: c.branch, Action: WorktreeActionDeleteBranch, Reason: "merged, clean, dead"},
		)
	}

	// Branch backlog: cycle-* branches with no worktree entry at all.
	backlogOut, err := o.git(o.ProjectRoot, "branch", "--list", "cycle-*")
	if err != nil {
		return WorktreeManifest{}, err
	}
	for _, b := range parseBranchList(backlogOut) {
		if !strings.HasPrefix(b, "cycle-") || seenBranch[b] {
			continue
		}
		if merged[b] {
			items = append(items, WorktreeItem{Branch: b, Action: WorktreeActionDeleteBranch, Reason: "merged orphan branch (no worktree)"})
		} else {
			items = append(items, WorktreeItem{Branch: b, Action: WorktreeActionFlagUnmerged, Reason: "unmerged orphan branch (no worktree)"})
		}
	}

	sortWorktreeItems(items)
	return WorktreeManifest{Items: items}, nil
}

func sortWorktreeItems(items []WorktreeItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		if items[i].Branch != items[j].Branch {
			return items[i].Branch < items[j].Branch
		}
		return items[i].Action < items[j].Action
	})
}

// ApplyWorktrees executes a manifest under .evolve/ship.lock. Every
// worktree-backed target is re-checked (TOCTOU) immediately before mutation: a
// target that became live or dirty since Plan is refused and reported, never
// removed. Removal is non-force `git worktree remove`; branch deletion is
// `git branch -d` (never -D); a single trailing `git worktree prune` reconciles
// the whole batch.
func ApplyWorktrees(o WorktreeOptions, m WorktreeManifest) error {
	if o.Exec == nil {
		return errors.New("gc: ApplyWorktrees requires Exec")
	}
	// Whole-apply critical section on the SHARED integrator lock (flock.ShipLockPath,
	// the SAME file internal/phases/ship acquireShipLock and the cycle-dossier commit
	// take) so a gc worktree apply never races a lane's ship/dossier index mutation.
	release, err := flock.Lock(flock.ShipLockPath(o.ProjectRoot))
	if err != nil {
		return fmt.Errorf("gc: acquire ship.lock: %w", err)
	}
	defer release()

	var errs []error
	refused := map[string]bool{}

	// Pass 1: TOCTOU re-check every worktree-backed target before any mutation.
	for _, it := range m.Items {
		if it.Path == "" || refused[it.Path] {
			continue
		}
		if it.Action != WorktreeActionRemove && it.Action != WorktreeActionDeleteBranch {
			continue
		}
		if o.isLive(it.Path) {
			refused[it.Path] = true
			errs = append(errs, fmt.Errorf("gc: refuse %s: became live between plan and apply", it.Path))
			continue
		}
		dirty, derr := o.isDirty(it.Path)
		if derr != nil {
			refused[it.Path] = true
			errs = append(errs, fmt.Errorf("gc: refuse %s: status re-check failed: %w", it.Path, derr))
			continue
		}
		if dirty {
			refused[it.Path] = true
			errs = append(errs, fmt.Errorf("gc: refuse %s: became dirty between plan and apply", it.Path))
		}
	}

	// Pass 2a: remove worktrees FIRST. git refuses `branch -d` on a branch
	// still checked out in a linked worktree, so the dir must go before its
	// branch.
	didRemove := false
	for _, it := range m.Items {
		if it.Action != WorktreeActionRemove || refused[it.Path] {
			continue
		}
		if _, err := o.git(o.ProjectRoot, "worktree", "remove", it.Path); err != nil {
			errs = append(errs, err)
			continue
		}
		didRemove = true
	}
	// Pass 2b: delete branches (their worktrees, if any, are now gone).
	for _, it := range m.Items {
		if it.Action != WorktreeActionDeleteBranch {
			continue
		}
		if it.Path != "" && refused[it.Path] {
			continue
		}
		if _, err := o.git(o.ProjectRoot, "branch", "-d", it.Branch); err != nil {
			errs = append(errs, err)
		}
	}
	if didRemove {
		if _, err := o.git(o.ProjectRoot, "worktree", "prune"); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
