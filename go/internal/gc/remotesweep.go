package gc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

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

type RemotePR struct {
	Branch     string
	State      string
	HeadRefOid string
}

type RemoteAPI interface {
	PullRequests(ctx context.Context) ([]RemotePR, error)
	RulesetProtects(ctx context.Context, branch string) (bool, error)
	DeleteBranch(ctx context.Context, branch string) error
}

type RemoteItem struct {
	Branch string       `json:"branch"`
	Head   string       `json:"head"`
	Action RemoteAction `json:"action"`
	Reason string       `json:"reason"`
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
}

const (
	prStateMerged = "MERGED"
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
	return tips, nil
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
		return fmt.Errorf("gc: delete remote branch %s: %w", it.Branch, err)
	}
	o.logLine("deleted", it)
	return nil
}

func (o RemoteSweepOptions) logLine(verb string, it RemoteItem) {
	fmt.Fprintf(o.Log, "gc: remote %s %s/%s %.12s (%s)\n", verb, o.Remote, it.Branch, it.Head, it.Reason)
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
	if got != it.Head {
		return fmt.Errorf("tip is %q, the plan saw %q", got, it.Head)
	}
	dir := filepath.Join(OperatorSalvageDir(o.EvolveDir), fmt.Sprintf("remote-%s-%s-%.12s", o.Remote, strings.ReplaceAll(it.Branch, "/", "-"), it.Head))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create bundle dir: %w", err)
	}
	return o.bundleRef(ctx, dir, ref, it)
}

func (o RemoteSweepOptions) bundleRef(ctx context.Context, dir, ref string, it RemoteItem) error {
	file := filepath.Join(dir, salvageBundleFile)
	for _, step := range [][]string{{"bundle", "create", file, ref}, {"bundle", "verify", file}} {
		if _, err := o.remoteGit(ctx, step...); err != nil {
			return fmt.Errorf("bundle failed, branch kept: %w", err)
		}
	}
	heads, err := o.remoteGit(ctx, "bundle", "list-heads", file)
	if err != nil {
		return err
	}
	if !strings.Contains(heads, it.Head+" "+ref) {
		return fmt.Errorf("bundle %s does not hold %s@%s", file, it.Branch, it.Head)
	}
	return os.WriteFile(filepath.Join(dir, salvageHeadFile), []byte(it.Head+" "+it.Branch+"\n"), 0o644)
}
