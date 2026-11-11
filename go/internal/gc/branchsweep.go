package gc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BranchScope int

const (
	BranchScopeCycle BranchScope = iota
	BranchScopeAll
)

const salvageBundleFile = "refs.bundle"

var protectedBranches = map[string]bool{"main": true, "master": true}

func (o WorktreeOptions) otherBranchItems(merged *mergedSet, porcelain string) ([]WorktreeItem, error) {
	if o.BranchScope != BranchScopeAll {
		return nil, nil
	}
	listed, err := o.git(o.ProjectRoot, "branch", "--list")
	if err != nil {
		return nil, err
	}
	held := map[string]bool{}
	for _, e := range parseWorktreePorcelain(porcelain) {
		held[e.branch] = true
	}
	var items []WorktreeItem
	for _, b := range parseBranchList(listed) {
		if strings.HasPrefix(b, "cycle-") || held[b] || protectedBranches[b] {
			continue
		}
		it, sweep, err := o.otherBranchItem(merged, b)
		if err != nil {
			return nil, err
		}
		if sweep {
			items = append(items, it)
		}
	}
	return items, nil
}

func (o WorktreeOptions) otherBranchItem(merged *mergedSet, branch string) (WorktreeItem, bool, error) {
	l, err := merged.landing(branch)
	if err != nil {
		return WorktreeItem{}, false, err
	}
	head, err := o.git(o.ProjectRoot, "rev-parse", "refs/heads/"+branch)
	if err != nil {
		return WorktreeItem{}, false, err
	}
	it := WorktreeItem{Branch: branch, Head: strings.TrimSpace(head), Action: WorktreeActionDeleteBranch, Bundle: true}
	if l != landingNone {
		it.Reason = "landed branch (" + removalReason(l) + "), saved to a verified bundle first"
		return it, true, nil
	}
	idle, err := o.branchIdle(branch)
	if err != nil {
		return WorktreeItem{}, false, err
	}
	if o.EndedAfter <= 0 || idle < o.EndedAfter {
		return WorktreeItem{}, false, nil
	}
	it.Reason = fmt.Sprintf("ended branch: no commit for %d days (limit %d), saved to a verified bundle first", int(idle.Hours()/24), int(o.EndedAfter.Hours()/24))
	return it, true, nil
}

func (o WorktreeOptions) branchIdle(branch string) (time.Duration, error) {
	out, err := o.git(o.ProjectRoot, "log", "-1", "--format=%ct", "refs/heads/"+branch)
	if err != nil {
		return 0, err
	}
	last, err := parseUnixSeconds(out)
	if err != nil {
		return 0, err
	}
	return o.now().Sub(last), nil
}

func (o WorktreeOptions) deleteOne(it WorktreeItem) error {
	if it.Bundle {
		return o.deleteBundled(it)
	}
	return o.deleteBranch(it)
}

func (o WorktreeOptions) deleteBundled(it WorktreeItem) error {
	if got := o.branchHead(it.Branch); got != it.Head {
		return fmt.Errorf("gc: refuse to delete %s: tip is %q, the plan saw %q", it.Branch, got, it.Head)
	}
	if err := o.refuseCheckedOut(it.Branch); err != nil {
		return err
	}
	if err := o.bundleBranch(it); err != nil {
		return fmt.Errorf("gc: refuse to delete %s: bundle failed, branch kept: %w", it.Branch, err)
	}
	return o.dropRef(it)
}

func (o WorktreeOptions) dropRef(it WorktreeItem) error {
	if _, err := o.git(o.ProjectRoot, "update-ref", "-d", "refs/heads/"+it.Branch, it.Head); err != nil {
		return err
	}
	o.logDeleted(it.Branch, it.Head)
	return nil
}

func (o WorktreeOptions) bundleBranch(it WorktreeItem) error {
	leaf := fmt.Sprintf("branch-%s-%.12s", strings.ReplaceAll(it.Branch, "/", "-"), it.Head)
	dir := filepath.Join(OperatorSalvageDir(o.EvolveDir), leaf)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create bundle dir: %w", err)
	}
	file := filepath.Join(dir, salvageBundleFile)
	if _, err := o.git(o.ProjectRoot, "bundle", "create", file, "refs/heads/"+it.Branch); err != nil {
		return err
	}
	if _, err := o.git(o.ProjectRoot, "bundle", "verify", file); err != nil {
		return err
	}
	heads, err := o.git(o.ProjectRoot, "bundle", "list-heads", file)
	if err != nil {
		return err
	}
	if !strings.Contains(heads, it.Head+" refs/heads/"+it.Branch) {
		return fmt.Errorf("bundle %s does not hold %s@%s", file, it.Branch, it.Head)
	}
	return os.WriteFile(filepath.Join(dir, salvageHeadFile), []byte(it.Head+" "+it.Branch+"\n"), 0o644)
}
