package gc

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var (
	ErrBranchKept         = errors.New("branch kept")
	errArchiveTipMismatch = errors.New("archive ref tip mismatch")
)

type BranchScope int

const (
	BranchScopeCycle BranchScope = iota
	BranchScopeAll
)

const salvageBundleFile = "refs.bundle"

var protectedBranches = map[string]bool{"main": true, "master": true}

type BranchSide string

const (
	BranchSideLocal  BranchSide = "local"
	BranchSideRemote BranchSide = "remote"
)

type BranchAction string

const (
	BranchActionDelete BranchAction = "delete"
)

type BranchSweepItem struct {
	Side   BranchSide   `json:"side"`
	Branch string       `json:"branch"`
	Tip    string       `json:"tip"`
	PR     int          `json:"pr,omitempty"`
	Class  string       `json:"class"`
	Action BranchAction `json:"action"`
	Reason string       `json:"reason"`
}

type BranchSweepManifest struct {
	Items      []BranchSweepItem `json:"items"`
	RemoteKept int               `json:"remote_kept"`
}

func (m BranchSweepManifest) RemoteCounts() (planned, kept int) {
	for _, it := range m.Items {
		if it.Side == BranchSideRemote && it.Action == BranchActionDelete {
			planned++
		}
	}
	return planned, m.RemoteKept
}

type BranchSweepOptions struct {
	ProjectRoot  string
	EvolveDir    string
	Remote       string
	Policy       Policy
	Stage        RemoteStage
	Exec         sysexec.RunFunc
	API          RemoteAPI
	Now          func() time.Time
	Log          io.Writer
	Stamp        string
	AllowOffline bool
}

func (o BranchSweepOptions) remoteName() string {
	return cmp.Or(o.Remote, "origin")
}

func (o BranchSweepOptions) nowTime() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

func (o BranchSweepOptions) git(ctx context.Context, args ...string) (string, error) {
	return sysexec.Output(ctx, o.Exec, o.ProjectRoot, "git", args...)
}

func (o BranchSweepOptions) defaultBranch(ctx context.Context) string {
	remote := o.remoteName()
	out, err := o.git(ctx, "symbolic-ref", "--quiet", "refs/remotes/"+remote+"/HEAD")
	if err == nil {
		if name, ok := strings.CutPrefix(strings.TrimSpace(out), "refs/remotes/"+remote+"/"); ok && name != "" {
			return name
		}
	}
	return "main"
}

func (o BranchSweepOptions) checkedOutBranches(ctx context.Context) (map[string]bool, error) {
	porcelain, err := o.git(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	held := map[string]bool{}
	for _, e := range parseWorktreePorcelain(porcelain) {
		if e.branch != "" {
			held[e.branch] = true
		}
	}
	return held, nil
}

func (o BranchSweepOptions) localTips(ctx context.Context) (map[string]string, error) {
	out, err := o.git(ctx, "for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	tips := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 {
			tips[f[0]] = f[1]
		}
	}
	return tips, nil
}

func (o BranchSweepOptions) remoteTips(ctx context.Context) (map[string]string, error) {
	out, err := o.git(ctx, "ls-remote", "--heads", o.remoteName())
	if err != nil {
		return nil, err
	}
	return parseRemoteHeads(out), nil
}

func PlanBranchSweep(ctx context.Context, o BranchSweepOptions) (BranchSweepManifest, error) {
	if o.Exec == nil {
		return BranchSweepManifest{}, errors.New("gc: PlanBranchSweep requires Exec")
	}
	held, err := o.checkedOutBranches(ctx)
	if err != nil {
		return BranchSweepManifest{}, err
	}
	localTips, err := o.localTips(ctx)
	if err != nil {
		return BranchSweepManifest{}, err
	}
	remoteTips, err := o.remoteTips(ctx)
	if err != nil {
		if !o.AllowOffline {
			return BranchSweepManifest{}, err
		}
		remoteTips = map[string]string{}
	}
	defaultBranch := o.defaultBranch(ctx)
	candidates := branchCandidates(localTips, remoteTips, defaultBranch, o.Policy.Branches)
	if len(candidates) == 0 {
		return BranchSweepManifest{RemoteKept: len(remoteTips)}, nil
	}
	idx, err := loadPRIndex(ctx, o.API, candidates)
	if err != nil {
		if !o.AllowOffline {
			return BranchSweepManifest{}, fmt.Errorf("gc: read pull requests: %w", err)
		}
		return BranchSweepManifest{RemoteKept: len(remoteTips)}, nil
	}
	return o.classifyCandidates(ctx, candidates, localTips, remoteTips, held, defaultBranch, idx), nil
}

func branchCandidates(localTips, remoteTips map[string]string, defaultBranch string, pol BranchesPolicy) []string {
	seen := map[string]bool{}
	for b := range localTips {
		seen[b] = true
	}
	for b := range remoteTips {
		seen[b] = true
	}
	var out []string
	for _, b := range slices.Sorted(maps.Keys(seen)) {
		if strings.HasPrefix(b, "cycle-") || protectedBranches[b] || b == defaultBranch || !pol.IsOwned(b) {
			continue
		}
		out = append(out, b)
	}
	return out
}

func (o BranchSweepOptions) classifyCandidates(ctx context.Context, candidates []string, localTips, remoteTips map[string]string, held map[string]bool, defaultBranch string, idx *prIndex) BranchSweepManifest {
	now := o.nowTime()
	grace := o.Policy.Branches.ClosedGrace()
	targetRef := "refs/remotes/" + o.remoteName() + "/" + defaultBranch
	hasTarget := o.refExists(ctx, targetRef)
	remoteDeletes := 0
	var items []BranchSweepItem
	rulesetAllowed := map[string]bool{}
	for _, b := range candidates {
		if held[b] || idx.failed[b] || idx.open[b] {
			continue
		}
		if tip, ok := localTips[b]; ok {
			if it, del := o.classifyLocal(ctx, b, tip, idx, now, grace, hasTarget, targetRef); del {
				items = append(items, it)
			}
		}
		if tip, ok := remoteTips[b]; ok {
			if it, del := o.classifyRemote(ctx, b, tip, idx, now, grace, rulesetAllowed); del {
				items = append(items, it)
				remoteDeletes++
			}
		}
	}
	return BranchSweepManifest{
		Items:      items,
		RemoteKept: len(remoteTips) - remoteDeletes,
	}
}

func (o BranchSweepOptions) refExists(ctx context.Context, ref string) bool {
	_, err := o.git(ctx, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

func (o BranchSweepOptions) classifyLocal(ctx context.Context, branch, tip string, idx *prIndex, now time.Time, grace time.Duration, hasTarget bool, targetRef string) (BranchSweepItem, bool) {
	if pr, ok := idx.matchMerged(branch, tip); ok {
		return BranchSweepItem{
			Side:   BranchSideLocal,
			Branch: branch,
			Tip:    tip,
			PR:     pr.Number,
			Class:  "B1",
			Action: BranchActionDelete,
			Reason: prMergedReason(pr.Number),
		}, true
	}
	if pr, ok := idx.matchClosedPastGrace(branch, tip, now, grace); ok {
		return BranchSweepItem{
			Side:   BranchSideLocal,
			Branch: branch,
			Tip:    tip,
			PR:     pr.Number,
			Class:  "B3",
			Action: BranchActionDelete,
			Reason: prClosedReason(pr.Number),
		}, true
	}
	if hasTarget && o.localLandedB2(ctx, branch, tip, targetRef) {
		return BranchSweepItem{
			Side:   BranchSideLocal,
			Branch: branch,
			Tip:    tip,
			Class:  "B2",
			Action: BranchActionDelete,
			Reason: "landed on " + strings.TrimPrefix(targetRef, "refs/remotes/") + " with own commits, saved to a record first",
		}, true
	}
	return BranchSweepItem{}, false
}

func (o BranchSweepOptions) localLandedB2(ctx context.Context, branch, tip, targetRef string) bool {
	g := gitexec.Git{Dir: o.ProjectRoot, Exec: o.Exec}
	ref := "refs/heads/" + branch
	own, err := branchHasOwnWork(ctx, g, ref, tip)
	if err != nil || !own {
		return false
	}
	_, _, code, err := g.Capture(ctx, "merge-base", "--is-ancestor", ref, targetRef)
	if err == nil && code == 0 {
		return true
	}
	res, err := branchLandedBySquash(ctx, g, targetRef, branch)
	return err == nil && res.deletable()
}

func (o BranchSweepOptions) classifyRemote(ctx context.Context, branch, tip string, idx *prIndex, now time.Time, grace time.Duration, cache map[string]bool) (BranchSweepItem, bool) {
	if pr, ok := idx.matchMerged(branch, tip); ok && o.rulesetAllows(ctx, branch, cache) {
		return BranchSweepItem{
			Side:   BranchSideRemote,
			Branch: branch,
			Tip:    tip,
			PR:     pr.Number,
			Class:  "B1",
			Action: BranchActionDelete,
			Reason: prMergedReason(pr.Number),
		}, true
	}
	if pr, ok := idx.matchClosedPastGrace(branch, tip, now, grace); ok && o.rulesetAllows(ctx, branch, cache) {
		return BranchSweepItem{
			Side:   BranchSideRemote,
			Branch: branch,
			Tip:    tip,
			PR:     pr.Number,
			Class:  "B3",
			Action: BranchActionDelete,
			Reason: prClosedReason(pr.Number),
		}, true
	}
	return BranchSweepItem{}, false
}

func (o BranchSweepOptions) rulesetAllows(ctx context.Context, branch string, cache map[string]bool) bool {
	if allowed, ok := cache[branch]; ok {
		return allowed
	}
	protected, err := o.API.RulesetProtects(ctx, branch)
	allowed := err == nil && !protected
	cache[branch] = allowed
	return allowed
}

func ApplyBranchSweep(ctx context.Context, o BranchSweepOptions, m BranchSweepManifest) error {
	if len(m.Items) == 0 {
		return nil
	}
	stamp := cmp.Or(o.Stamp, o.nowTime().UTC().Format("20060102T150405Z"))
	remote := o.remoteName()
	var remoteItems []RemoteItem
	var errs []error
	for _, it := range m.Items {
		if it.Action != BranchActionDelete {
			continue
		}
		if it.Side == BranchSideRemote {
			remoteItems = append(remoteItems, RemoteItem{
				Branch: it.Branch,
				Head:   it.Tip,
				Action: RemoteActionDelete,
				Reason: it.Reason,
				Class:  it.Class,
				PR:     it.PR,
			})
			continue
		}
		if err := o.applyLocalOne(ctx, stamp, it); err != nil {
			errs = append(errs, err)
		}
	}
	if len(remoteItems) > 0 {
		ro := RemoteSweepOptions{
			ProjectRoot: o.ProjectRoot,
			EvolveDir:   o.EvolveDir,
			Remote:      remote,
			Stage:       o.Stage,
			Exec:        o.Exec,
			API:         o.API,
			Log:         o.Log,
			Stamp:       stamp,
		}
		if err := ApplyRemoteSweep(ctx, ro, RemoteManifest{Items: remoteItems}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (o BranchSweepOptions) applyLocalOne(ctx context.Context, stamp string, it BranchSweepItem) error {
	if o.Stage != RemoteStageEnforce {
		fmt.Fprintf(o.Log, "gc: local WOULD-DELETE %s %.12s (%s)\n", it.Branch, it.Tip, it.Reason)
		return nil
	}
	wt := WorktreeOptions{ProjectRoot: o.ProjectRoot, EvolveDir: o.EvolveDir, Exec: o.Exec}
	if err := wt.refuseCheckedOut(it.Branch); err != nil {
		return err
	}
	if got := wt.branchHead(it.Branch); got != it.Tip {
		return fmt.Errorf("gc: refuse to delete %s: %w (tip is %q, the plan saw %q)", it.Branch, errBranchTipMoved, got, it.Tip)
	}
	git := func(_ string, args ...string) (string, error) { return o.git(ctx, args...) }
	if err := recordBranchDelete(git, o.ProjectRoot, o.EvolveDir, stamp, BranchSideLocal, it.Branch, "refs/heads/"+it.Branch, it.Tip, it.PR, it.Class); err != nil {
		return fmt.Errorf("gc: refuse to delete %s: %w: %w", it.Branch, ErrBranchKept, err)
	}
	if _, err := o.git(ctx, "update-ref", "-d", "refs/heads/"+it.Branch, it.Tip); err != nil {
		return err
	}
	fmt.Fprintf(o.Log, "gc: local deleted %s %.12s (%s) — restore: git branch %s %s\n", it.Branch, it.Tip, it.Reason, it.Branch, it.Tip)
	return nil
}

func recordBranchDelete(git gitRun, projectRoot, evolveDir, stamp string, side BranchSide, branch, ref, tip string, pr int, class string) error {
	if class == "B3" {
		archive := "refs/archive/branches/" + stamp + "/" + branch
		if _, err := git(projectRoot, "update-ref", archive, tip); err != nil {
			return err
		}
		got, err := git(projectRoot, "rev-parse", "--verify", archive)
		if err != nil {
			return fmt.Errorf("verify %s: %w", archive, err)
		}
		if strings.TrimSpace(got) != tip {
			return fmt.Errorf("verify %s: %w (got %q, want %q)", archive, errArchiveTipMismatch, strings.TrimSpace(got), tip)
		}
		return nil
	}
	gcRoot := filepath.Join(OperatorSalvageDir(evolveDir), "branch-gc")
	bundleDir := filepath.Join(gcRoot, stamp)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		return fmt.Errorf("create branch-gc dir: %w", err)
	}
	bundleFile := filepath.Join(bundleDir, fmt.Sprintf("%s-%s-%.12s.bundle", side, strings.ReplaceAll(branch, "/", "-"), tip))
	if err := bundleVerified(git, projectRoot, bundleFile, ref, tip); err != nil {
		return fmt.Errorf("bundle failed, branch kept: %w", err)
	}
	tsvPath := filepath.Join(gcRoot, stamp+".tsv")
	f, err := os.OpenFile(tsvPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open branch-gc tsv: %w", err)
	}
	prCell := "-"
	if pr > 0 {
		prCell = fmt.Sprintf("#%d", pr)
	}
	_, writeErr := fmt.Fprintf(f, "%s\t%s\t%s\t%s\t%s\n", side, branch, tip, prCell, class)
	return errors.Join(writeErr, f.Close())
}

type branchCheck struct {
	api RemoteAPI
	now func() time.Time
	log io.Writer
}

func (branchCheck) Name() string { return "branches" }

func (b branchCheck) options(cc CheckContext, allowOffline bool) BranchSweepOptions {
	execFn := cc.Worktrees.Exec
	if execFn == nil {
		execFn = sysexec.DefaultRunner
	}
	evolveDir := cc.EvolveDir
	if evolveDir == "" && cc.ProjectRoot != "" {
		evolveDir = filepath.Join(cc.ProjectRoot, ".evolve")
	}
	api := b.api
	if api == nil {
		api = GhRemoteAPI{Exec: execFn, Dir: cc.ProjectRoot}
	}
	logOut := b.log
	if logOut == nil {
		logOut = io.Discard
	}
	return BranchSweepOptions{
		ProjectRoot:  cc.ProjectRoot,
		EvolveDir:    evolveDir,
		Remote:       "origin",
		Policy:       cc.Policy,
		Stage:        RemoteStage(cc.Policy.RemoteStage()),
		Exec:         execFn,
		API:          api,
		Now:          b.now,
		Log:          logOut,
		AllowOffline: allowOffline,
	}
}

func (b branchCheck) Scan(ctx context.Context, cc CheckContext) ([]CheckItem, error) {
	m, err := PlanBranchSweep(ctx, b.options(cc, true))
	if err != nil {
		return nil, err
	}
	items := make([]CheckItem, 0, len(m.Items))
	for _, it := range m.Items {
		prTag := "none"
		if it.PR > 0 {
			prTag = fmt.Sprintf("#%d", it.PR)
		}
		subj := fmt.Sprintf("%s:%s", it.Side, it.Branch)
		if it.Side == BranchSideRemote {
			subj = fmt.Sprintf("remote:origin/%s", it.Branch)
		}
		items = append(items, CheckItem{
			Subject:  subj,
			Action:   string(it.Action),
			Rule:     "branches." + it.Class,
			Evidence: fmt.Sprintf("tip=%.12s pr=%s %s", it.Tip, prTag, it.Reason),
		})
	}
	return items, nil
}

func (b branchCheck) Apply(ctx context.Context, cc CheckContext) error {
	opts := b.options(cc, false)
	m, err := PlanBranchSweep(ctx, opts)
	if err != nil {
		return err
	}
	return ApplyBranchSweep(ctx, opts, m)
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
		return fmt.Errorf("gc: refuse to delete %s: bundle failed, %w: %w", it.Branch, ErrBranchKept, err)
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
	if err := bundleVerified(o.git, o.ProjectRoot, filepath.Join(dir, salvageBundleFile), "refs/heads/"+it.Branch, it.Head); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, salvageHeadFile), []byte(it.Head+" "+it.Branch+"\n"), 0o644)
}
