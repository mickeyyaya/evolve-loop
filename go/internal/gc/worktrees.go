package gc

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
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type WorktreeAction string

const (
	WorktreeActionRemove        WorktreeAction = "remove"
	WorktreeActionDeleteBranch  WorktreeAction = "delete-branch"
	WorktreeActionFlagDirty     WorktreeAction = "flag-dirty"
	WorktreeActionFlagUnmerged  WorktreeAction = "flag-unmerged"
	WorktreeActionSalvageRemove WorktreeAction = "salvage-remove"
)

type WorktreeItem struct {
	Path   string         `json:"path,omitempty"`
	Branch string         `json:"branch,omitempty"`
	Action WorktreeAction `json:"action"`
	Reason string         `json:"reason,omitempty"`
}

type WorktreeManifest struct {
	Items []WorktreeItem `json:"items"`
}

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
	branch string
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

func LeafCycleNumber(leaf string) (int, bool) {
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

func resolvePath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

func underBase(base, path string) bool {
	return resolvePath(filepath.Dir(filepath.Clean(path))) == resolvePath(base)
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

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

func (o WorktreeOptions) runClosedOut(runDir string) bool {
	n, ok := LeafCycleNumber(runDir)
	return ok && strings.HasPrefix(runDir, "cycle-") && dossier.ClosedOut(o.ProjectRoot, n)
}

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
			if activeWorktreeMatches(filepath.Join(runsDir, e.Name(), "run.json"), path) && !o.runClosedOut(e.Name()) {
				return true
			}
		}
	}
	if n, ok := LeafCycleNumber(filepath.Base(path)); ok {
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

func PlanWorktrees(o WorktreeOptions) (WorktreeManifest, error) {
	if o.Exec == nil {
		return WorktreeManifest{}, errors.New("gc: PlanWorktrees requires Exec")
	}
	porcelain, err := o.git(o.ProjectRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return WorktreeManifest{}, err
	}
	merged, err := o.mergedBranches()
	if err != nil {
		return WorktreeManifest{}, err
	}
	items, pool, seenBranch, err := o.scanWorktrees(porcelain, merged)
	if err != nil {
		return WorktreeManifest{}, err
	}

	items = append(items, removalItems(pool, o.Policy.KeepRecent)...)

	orphans, err := o.orphanBranchItems(merged, seenBranch)
	if err != nil {
		return WorktreeManifest{}, err
	}
	items = append(items, orphans...)

	sortWorktreeItems(items)
	return WorktreeManifest{Items: items}, nil
}

func (o WorktreeOptions) mergedBranches() (map[string]bool, error) {
	mergedOut, err := o.git(o.ProjectRoot, "branch", "--merged", "HEAD")
	if err != nil {
		return nil, err
	}
	merged := map[string]bool{}
	for _, b := range parseBranchList(mergedOut) {
		merged[b] = true
	}
	return merged, nil
}

type eligible struct {
	path, branch string
	mtime        time.Time
}

func (o WorktreeOptions) scanWorktrees(porcelain string, merged map[string]bool) ([]WorktreeItem, []eligible, map[string]bool, error) {
	minAge := time.Duration(o.Policy.MinAgeMinutes) * time.Minute
	now := o.now()

	var items []WorktreeItem
	seenBranch := map[string]bool{}
	var pool []eligible

	for _, e := range parseWorktreePorcelain(porcelain) {
		if e.branch == "" || !underBase(o.WorktreeBase, e.path) {
			continue
		}
		leaf := filepath.Base(e.path)
		if !strings.HasPrefix(leaf, "cycle-") {
			continue
		}
		path := filepath.Join(o.WorktreeBase, leaf)
		seenBranch[e.branch] = true

		if o.isLive(path) {
			continue
		}
		dirty, err := o.isDirty(path)
		if err != nil {
			return nil, nil, nil, err
		}
		if dirty || !merged[e.branch] {
			items = append(items, o.keptOrSalvaged(path, e.branch, dirty))
			continue
		}
		if c, ok := eligibleAfterGrace(path, e.branch, now, minAge); ok {
			pool = append(pool, c)
		}
	}
	return items, pool, seenBranch, nil
}

func (o WorktreeOptions) keptOrSalvaged(path, branch string, dirty bool) WorktreeItem {
	if age, ok := o.finishedFor(path); ok && o.Policy.SalvageAfterHours > 0 && age >= time.Duration(o.Policy.SalvageAfterHours)*time.Hour {
		return WorktreeItem{Path: path, Branch: branch, Action: WorktreeActionSalvageRemove,
			Reason: fmt.Sprintf("cycle closed out %s ago — uncommitted state salvaged to operator-salvage, branch kept", age.Round(time.Hour))}
	}
	if dirty {
		return WorktreeItem{Path: path, Branch: branch, Action: WorktreeActionFlagDirty, Reason: "dirty worktree — preserved for manual review"}
	}
	return WorktreeItem{Path: path, Branch: branch, Action: WorktreeActionFlagUnmerged, Reason: "branch not merged into HEAD"}
}

func (o WorktreeOptions) finishedFor(path string) (time.Duration, bool) {
	n, ok := LeafCycleNumber(filepath.Base(path))
	if !ok {
		return 0, false
	}
	at, ok := dossier.ClosedOutAt(o.ProjectRoot, n)
	if !ok {
		return 0, false
	}
	return o.now().Sub(at), true
}

func eligibleAfterGrace(path, branch string, now time.Time, minAge time.Duration) (eligible, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return eligible{}, false
	}
	if minAge > 0 && now.Sub(info.ModTime()) < minAge {
		return eligible{}, false
	}
	return eligible{path: path, branch: branch, mtime: info.ModTime()}, true
}

func removalItems(pool []eligible, keepRecent int) []WorktreeItem {
	sort.Slice(pool, func(i, j int) bool { return pool[i].mtime.After(pool[j].mtime) })
	var items []WorktreeItem
	for i, c := range pool {
		if i < keepRecent {
			continue
		}
		items = append(items,
			WorktreeItem{Path: c.path, Branch: c.branch, Action: WorktreeActionRemove, Reason: "merged, clean, dead"},
			WorktreeItem{Path: c.path, Branch: c.branch, Action: WorktreeActionDeleteBranch, Reason: "merged, clean, dead"},
		)
	}
	return items
}

func (o WorktreeOptions) orphanBranchItems(merged, seenBranch map[string]bool) ([]WorktreeItem, error) {
	backlogOut, err := o.git(o.ProjectRoot, "branch", "--list", "cycle-*")
	if err != nil {
		return nil, err
	}
	var items []WorktreeItem
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
	return items, nil
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

func ApplyWorktrees(o WorktreeOptions, m WorktreeManifest) error {
	if o.Exec == nil {
		return errors.New("gc: ApplyWorktrees requires Exec")
	}
	release, err := flock.Lock(flock.ShipLockPath(o.ProjectRoot))
	if err != nil {
		return fmt.Errorf("gc: acquire ship.lock: %w", err)
	}
	defer release()

	refused, errs := o.refuseChangedTargets(m.Items)

	didRemove, removeErrs := o.removeWorktrees(m.Items, refused)
	errs = append(errs, removeErrs...)
	didSalvage, salvageErrs := o.salvageRemoveWorktrees(m.Items, refused)
	errs = append(errs, salvageErrs...)
	didRemove = didRemove || didSalvage
	errs = append(errs, o.deleteBranches(m.Items, refused)...)
	if didRemove {
		if _, err := o.git(o.ProjectRoot, "worktree", "prune"); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (o WorktreeOptions) refuseChangedTargets(items []WorktreeItem) (map[string]bool, []error) {
	var errs []error
	refused := map[string]bool{}
	for _, it := range items {
		if it.Path == "" || refused[it.Path] {
			continue
		}
		if it.Action != WorktreeActionRemove && it.Action != WorktreeActionDeleteBranch && it.Action != WorktreeActionSalvageRemove {
			continue
		}
		if o.isLive(it.Path) {
			refused[it.Path] = true
			errs = append(errs, fmt.Errorf("gc: refuse %s: became live between plan and apply", it.Path))
			continue
		}
		if it.Action == WorktreeActionSalvageRemove {
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
	return refused, errs
}

func (o WorktreeOptions) removeWorktrees(items []WorktreeItem, refused map[string]bool) (bool, []error) {
	var errs []error
	didRemove := false
	for _, it := range items {
		if it.Action != WorktreeActionRemove || refused[it.Path] {
			continue
		}
		if _, err := o.git(o.ProjectRoot, "worktree", "remove", it.Path); err != nil {
			errs = append(errs, err)
			continue
		}
		didRemove = true
	}
	return didRemove, errs
}

func (o WorktreeOptions) deleteBranches(items []WorktreeItem, refused map[string]bool) []error {
	var errs []error
	for _, it := range items {
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
	return errs
}
