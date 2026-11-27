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

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type RemoteStage string

const (
	RemoteStageShadow  RemoteStage = "shadow"
	RemoteStageEnforce RemoteStage = "enforce"
)

type RemoteAction string

const (
	RemoteActionDelete RemoteAction = "delete"
	RemoteActionKeep   RemoteAction = "keep"
)

var ErrRemoteDeleteFailed = errors.New("remote delete failed")

type remoteDeleteError struct {
	branch string
	err    error
}

func (e remoteDeleteError) Error() string {
	return fmt.Sprintf("gc: delete remote branch %s: %v", e.branch, e.err)
}

func (e remoteDeleteError) Unwrap() error { return e.err }

func (e remoteDeleteError) Is(target error) bool { return target == ErrRemoteDeleteFailed }

type RemotePR struct {
	Number     int
	Branch     string
	State      string
	HeadRefOid string
	ClosedAt   time.Time
}

type RemoteAPI interface {
	PullRequests(ctx context.Context) ([]RemotePR, error)
	RulesetProtects(ctx context.Context, branch string) (bool, error)
	DeleteBranch(ctx context.Context, branch string) error
}

type CandidatePRAPI interface {
	OpenPullRequests(ctx context.Context) ([]RemotePR, bool, error)
	AllPullRequests(ctx context.Context) ([]RemotePR, bool, error)
	BranchPullRequests(ctx context.Context, branch string) ([]RemotePR, bool, error)
}

type RemoteItem struct {
	Branch string       `json:"branch"`
	Head   string       `json:"head"`
	Action RemoteAction `json:"action"`
	Reason string       `json:"reason"`
	Class  string       `json:"class,omitempty"`
	PR     int          `json:"pr,omitempty"`
}

type RemoteManifest struct {
	Items []RemoteItem `json:"items"`
}

type RemoteSweepOptions struct {
	ProjectRoot string
	EvolveDir   string
	Remote      string
	Stage       RemoteStage
	Exec        sysexec.RunFunc
	API         RemoteAPI
	Log         io.Writer
	Stamp       string
}

const (
	prStateMerged = "MERGED"
	prStateClosed = "CLOSED"
	prStateOpen   = "OPEN"
)

func (o RemoteSweepOptions) remoteGit(ctx context.Context, args ...string) (string, error) {
	return sysexec.Output(ctx, o.Exec, o.ProjectRoot, "git", args...)
}

func (o RemoteSweepOptions) remoteTips(ctx context.Context) (map[string]string, error) {
	out, err := o.remoteGit(ctx, "ls-remote", "--heads", o.Remote)
	if err != nil {
		return nil, err
	}
	return parseRemoteHeads(out), nil
}

func parseRemoteHeads(out string) map[string]string {
	tips := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		if name, ok := strings.CutPrefix(f[1], "refs/heads/"); ok {
			tips[name] = f[0]
		}
	}
	return tips
}

type prIndex struct {
	open   map[string]bool
	failed map[string]bool
	merged map[string]RemotePR
	closed map[string][]RemotePR
}

func newPRIndex() *prIndex {
	return &prIndex{
		open:   map[string]bool{},
		failed: map[string]bool{},
		merged: map[string]RemotePR{},
		closed: map[string][]RemotePR{},
	}
}

func (idx *prIndex) add(p RemotePR) {
	switch p.State {
	case prStateOpen:
		idx.open[p.Branch] = true
	case prStateMerged:
		key := p.Branch + "@" + p.HeadRefOid
		if cur, ok := idx.merged[key]; !ok || p.Number > cur.Number {
			idx.merged[key] = p
		}
	case prStateClosed:
		key := p.Branch + "@" + p.HeadRefOid
		idx.closed[key] = append(idx.closed[key], p)
	}
}

func (idx *prIndex) matchMerged(branch, tip string) (RemotePR, bool) {
	p, ok := idx.merged[branch+"@"+tip]
	return p, ok
}

func (idx *prIndex) matchClosedPastGrace(branch, tip string, now time.Time, grace time.Duration) (RemotePR, bool) {
	var best RemotePR
	found := false
	for _, p := range idx.closed[branch+"@"+tip] {
		if p.ClosedAt.IsZero() || now.Sub(p.ClosedAt) < grace {
			continue
		}
		if !found || p.ClosedAt.After(best.ClosedAt) {
			best = p
			found = true
		}
	}
	return best, found
}

func loadPRIndex(ctx context.Context, api RemoteAPI, candidates []string) (*prIndex, error) {
	idx := newPRIndex()
	capi, ok := api.(CandidatePRAPI)
	if !ok {
		prs, err := api.PullRequests(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range prs {
			idx.add(p)
		}
		return idx, nil
	}
	openRows, openTrunc, err := capi.OpenPullRequests(ctx)
	if err != nil {
		return nil, err
	}
	allRows, allTrunc, err := capi.AllPullRequests(ctx)
	if err != nil {
		return nil, err
	}
	if !openTrunc && !allTrunc {
		for _, p := range openRows {
			idx.add(p)
		}
		for _, p := range allRows {
			idx.add(p)
		}
		return idx, nil
	}
	for _, b := range candidates {
		rows, bTrunc, err := capi.BranchPullRequests(ctx, b)
		if err != nil || bTrunc {
			idx.failed[b] = true
			continue
		}
		for _, p := range rows {
			idx.add(p)
		}
	}
	return idx, nil
}

func PlanRemoteSweep(ctx context.Context, o RemoteSweepOptions) (RemoteManifest, error) {
	tips, err := o.remoteTips(ctx)
	if err != nil {
		return RemoteManifest{}, err
	}
	prs, err := o.API.PullRequests(ctx)
	if err != nil {
		return RemoteManifest{}, fmt.Errorf("gc: read pull requests: %w", err)
	}
	open, landed := map[string]bool{}, map[string]bool{}
	for _, p := range prs {
		switch p.State {
		case prStateOpen:
			open[p.Branch] = true
		case prStateMerged:
			landed[p.Branch+"@"+p.HeadRefOid] = true
		}
	}
	var m RemoteManifest
	for _, branch := range slices.Sorted(maps.Keys(tips)) {
		m.Items = append(m.Items, o.judgeRemote(ctx, branch, tips[branch], open, landed))
	}
	return m, nil
}

func (o RemoteSweepOptions) judgeRemote(ctx context.Context, branch, tip string, open, landed map[string]bool) RemoteItem {
	it := RemoteItem{Branch: branch, Head: tip, Action: RemoteActionKeep}
	switch {
	case protectedBranches[branch]:
		it.Reason = "default branch"
	case open[branch]:
		it.Reason = "open pull request"
	case !landed[branch+"@"+tip]:
		it.Reason = "no merged pull request at this tip"
	default:
		it.Action, it.Reason = o.rulesetVerdict(ctx, branch)
	}
	return it
}

func prMergedReason(num int) string {
	if num > 0 {
		return fmt.Sprintf("landed: merged pull request #%d at this tip, saved to a verified bundle first", num)
	}
	return "landed: merged pull request at this tip, saved to a verified bundle first"
}

func prClosedReason(num int) string {
	return fmt.Sprintf("closed pull request #%d past grace at this tip, archived to ref first", num)
}

func (o RemoteSweepOptions) rulesetVerdict(ctx context.Context, branch string) (RemoteAction, string) {
	protected, err := o.API.RulesetProtects(ctx, branch)
	switch {
	case err != nil:
		return RemoteActionKeep, "ruleset lookup failed: " + err.Error()
	case protected:
		return RemoteActionKeep, "a repository ruleset protects it"
	}
	return RemoteActionDelete, "landed: merged pull request at this tip, saved to a verified bundle first"
}

func ApplyRemoteSweep(ctx context.Context, o RemoteSweepOptions, m RemoteManifest) error {
	var failures []error
	for _, it := range m.Items {
		if it.Action != RemoteActionDelete {
			continue
		}
		if err := o.applyOne(ctx, it); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (o RemoteSweepOptions) applyOne(ctx context.Context, it RemoteItem) error {
	if o.Stage != RemoteStageEnforce {
		o.logLine("WOULD-DELETE", it)
		return nil
	}
	if err := o.saveTip(ctx, it); err != nil {
		return fmt.Errorf("gc: refuse to delete remote branch %s: %w", it.Branch, err)
	}
	if err := o.API.DeleteBranch(ctx, it.Branch); err != nil {
		return remoteDeleteError{branch: it.Branch, err: err}
	}
	o.logDeleted(it)
	return nil
}

func (o RemoteSweepOptions) logLine(verb string, it RemoteItem) {
	fmt.Fprintf(o.Log, "gc: remote %s %s/%s %.12s (%s)\n", verb, o.Remote, it.Branch, it.Head, it.Reason)
}

func (o RemoteSweepOptions) logDeleted(it RemoteItem) {
	fmt.Fprintf(o.Log, "gc: remote deleted %s/%s %.12s (%s) — restore: git push %s %s:refs/heads/%s\n",
		o.Remote, it.Branch, it.Head, it.Reason, o.Remote, it.Head, it.Branch)
}

func (o RemoteSweepOptions) saveTip(ctx context.Context, it RemoteItem) error {
	ref := "refs/remotes/" + o.Remote + "/" + it.Branch
	if _, err := o.remoteGit(ctx, "fetch", "--quiet", o.Remote, "+refs/heads/"+it.Branch+":"+ref); err != nil {
		return err
	}
	got, err := o.remoteGit(ctx, "rev-parse", ref)
	if err != nil {
		return err
	}
	if strings.TrimSpace(got) != it.Head {
		return fmt.Errorf("tip is %q, the plan saw %q", strings.TrimSpace(got), it.Head)
	}
	if o.Stamp != "" {
		git := func(_ string, args ...string) (string, error) { return o.remoteGit(ctx, args...) }
		return recordBranchDelete(git, o.ProjectRoot, o.EvolveDir, o.Stamp, BranchSideRemote, it.Branch, ref, it.Head, it.PR, cmp.Or(it.Class, "B1"))
	}
	dir := filepath.Join(OperatorSalvageDir(o.EvolveDir), fmt.Sprintf("remote-%s-%s-%.12s", o.Remote, strings.ReplaceAll(it.Branch, "/", "-"), it.Head))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create bundle dir: %w", err)
	}
	return o.bundleRef(ctx, dir, ref, it)
}

func (o RemoteSweepOptions) bundleRef(ctx context.Context, dir, ref string, it RemoteItem) error {
	git := func(_ string, args ...string) (string, error) { return o.remoteGit(ctx, args...) }
	if err := bundleVerified(git, o.ProjectRoot, filepath.Join(dir, salvageBundleFile), ref, it.Head); err != nil {
		return fmt.Errorf("bundle failed, branch kept: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, salvageHeadFile), []byte(it.Head+" "+it.Branch+"\n"), 0o644)
}
