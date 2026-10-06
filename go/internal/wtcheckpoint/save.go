package wtcheckpoint

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type SaveOptions struct {
	Label string
	Keep  int
	Now   func() time.Time
}

type SaveResult struct {
	Worktree string   `json:"worktree"`
	Dir      string   `json:"dir"`
	Status   Status   `json:"status"`
	Ref      string   `json:"ref,omitempty"`
	Pruned   []string `json:"pruned,omitempty"`
}

func Save(ctx context.Context, w Worktree, o SaveOptions) (SaveResult, error) {
	s := saver{tree: gitexec.Isolated(w.Dir), name: w.Name, opts: o}
	res := SaveResult{Worktree: w.Name, Dir: w.Dir}
	if _, _, code, err := s.tree.Capture(ctx, "check-ref-format", refPrefix+w.Name+"/"+stampLayout); err != nil || code != 0 {
		return res, fmt.Errorf("wtcheckpoint: refused: worktree name %q cannot name a ref under %s", w.Name, refPrefix)
	}
	head, err := s.tree.HEAD(ctx)
	if err != nil {
		return res, err
	}
	snap, err := snapshot(ctx, s.tree)
	if err != nil {
		return res, err
	}
	headTree, err := s.tree.Output(ctx, "rev-parse", head+"^{tree}")
	if err != nil {
		return res, err
	}
	if snap.staged == headTree && snap.full == headTree {
		res.Status = StatusClean
		return res, nil
	}
	refs, err := worktreeRefs(ctx, s.tree, w.Name)
	if err != nil {
		return res, err
	}
	if n := len(refs); n > 0 && refs[n-1].tree == snap.full {
		res.Status, res.Ref = StatusUnchanged, refs[n-1].name
		return res, nil
	}
	if res.Ref, err = s.record(ctx, head, snap); err != nil {
		return res, err
	}
	res.Status = StatusSaved
	res.Pruned, err = pruneWorktree(ctx, s.tree, w.Name, o.Keep)
	return res, err
}

type saver struct {
	tree gitexec.Git
	name string
	opts SaveOptions
}

type commitSpec struct{ tree, parent, subject, label string }

func (s saver) record(ctx context.Context, head string, snap trees) (string, error) {
	now := time.Now
	if s.opts.Now != nil {
		now = s.opts.Now
	}
	stamp := now().UTC().Format(stampLayout)
	staged, err := s.commit(ctx, commitSpec{tree: snap.staged, parent: head, subject: "checkpoint " + s.name + " staged " + stamp})
	if err != nil {
		return "", err
	}
	full, err := s.commit(ctx, commitSpec{tree: snap.full, parent: staged, subject: "checkpoint " + s.name + " worktree " + stamp, label: s.opts.Label})
	if err != nil {
		return "", err
	}
	ref := refPrefix + s.name + "/" + stamp
	return ref, s.tree.Run(ctx, "update-ref", ref, full, "")
}

func (s saver) commit(ctx context.Context, c commitSpec) (string, error) {
	args := []string{"-c", "commit.gpgsign=false", "commit-tree", c.tree, "-p", c.parent, "-m", c.subject}
	if label := strings.Join(strings.Fields(c.label), " "); label != "" {
		args = append(args, "-m", labelPrefix+label)
	}
	return s.tree.Output(ctx, args...)
}

func pruneWorktree(ctx context.Context, g gitexec.Git, name string, keep int) ([]string, error) {
	refs, err := worktreeRefs(ctx, g, name)
	if err != nil {
		return nil, err
	}
	return deleteRefs(ctx, g, beyondNewest(refs, keep))
}
