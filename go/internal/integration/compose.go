package integration

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	CauseConflict = "conflict"
	CauseGate     = "gate"
)

type Batch struct {
	Wave   string
	Index  int
	Base   string
	Branch string
	Tip    string
}

type Gate func(ctx context.Context, tip string, tier overlap.Tier) error

type OverlapFacts struct {
	Module   overlap.Module
	Catalogs overlap.Catalogs
}

type Composer struct {
	Repo    Repo
	Stage   string
	Gate    Gate
	Overlap *OverlapFacts
}

type Composed struct {
	Batch Batch
	Proof overlap.Proof
}

type Ejection struct {
	Cycle int
	Cause string
	Paths []string
	Diff  string
	Err   error
}

func (e *Ejection) Error() string {
	return fmt.Sprintf("integration: cycle %d ejected on a %s %v: %v", e.Cycle, e.Cause, e.Paths, e.Err)
}

var (
	waveName       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	artifactDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func batchBranch(wave string, index int) string {
	return "integration/w" + wave + "-b" + strconv.Itoa(index)
}

func branchRef(branch string) string { return "refs/heads/" + branch }

func (b Batch) named() error {
	if !waveName.MatchString(b.Wave) || b.Index < 1 {
		return fmt.Errorf("integration: wave %q batch %d is not a batch name", b.Wave, b.Index)
	}
	return fullObjectID(b.Base)
}

func (b Batch) check() error {
	if err := b.named(); err != nil {
		return err
	}
	if b.Branch != batchBranch(b.Wave, b.Index) {
		return fmt.Errorf("integration: branch %q is not the branch of wave %s batch %d", b.Branch, b.Wave, b.Index)
	}
	return fullObjectID(b.Tip)
}

func (r Repo) StartBatch(ctx context.Context, b Batch) (Batch, error) {
	if err := b.named(); err != nil {
		return Batch{}, err
	}
	b.Branch, b.Tip = batchBranch(b.Wave, b.Index), b.Base
	if _, err := r.Git(ctx, r.Dir, "", "update-ref", "--end-of-options", branchRef(b.Branch), b.Base, ""); err != nil {
		return Batch{}, fmt.Errorf("integration: cut %s at %s: %w", b.Branch, b.Base, err)
	}
	return b, nil
}

func checkComposable(rec DeliverableRecord) error {
	if err := rec.Validate(); err != nil {
		return err
	}
	if rec.Trailer != CycleTrailer(rec.Cycle) {
		return fmt.Errorf("integration: record trailer %q is not the trailer of cycle %d", rec.Trailer, rec.Cycle)
	}
	if !artifactDigest.MatchString(rec.Audit.ArtifactSHA) {
		return fmt.Errorf("integration: cycle %d has no audit artifact digest", rec.Cycle)
	}
	ids := []string{rec.Audit.TreeSHA}
	for _, c := range rec.Commits {
		ids = append(ids, c.SHA, c.BaseSHA)
	}
	for _, id := range ids {
		if err := fullObjectID(id); err != nil {
			return fmt.Errorf("integration: cycle %d: %w", rec.Cycle, err)
		}
	}
	return nil
}

func (c Composer) ready(b Batch) error {
	switch {
	case c.Stage != policy.FleetIntegrationShadow && c.Stage != policy.FleetIntegrationEnforce:
		return fmt.Errorf("integration: compose stage %q is not %s or %s", c.Stage, policy.FleetIntegrationShadow, policy.FleetIntegrationEnforce)
	case c.Stage == policy.FleetIntegrationEnforce && c.Gate == nil:
		return errors.New("integration: an enforcing compose needs a gate")
	}
	return b.check()
}

func (c Composer) Compose(ctx context.Context, b Batch, rec DeliverableRecord) (Composed, error) {
	if err := errors.Join(c.ready(b), checkComposable(rec)); err != nil {
		return Composed{}, err
	}
	st, err := c.attempt(ctx, b, rec)
	if err != nil {
		return Composed{}, err
	}
	if err := c.Repo.moveBranch(ctx, b, st.Batch.Tip); err != nil {
		return Composed{}, err
	}
	return st, nil
}

func (r Repo) moveBranch(ctx context.Context, b Batch, tip string) error {
	if _, err := r.Git(ctx, r.Dir, "", "update-ref", "--end-of-options", branchRef(b.Branch), tip, b.Tip); err != nil {
		return fmt.Errorf("integration: move %s from %s: %w", b.Branch, b.Tip, err)
	}
	return nil
}

func (c Composer) attempt(ctx context.Context, b Batch, rec DeliverableRecord) (Composed, error) {
	st, err := c.apply(ctx, b, rec)
	if err != nil || c.Stage == policy.FleetIntegrationShadow {
		return st, err
	}
	if err := c.Gate(ctx, st.Batch.Tip, st.Proof.Tier); err != nil {
		return Composed{}, c.eject(ctx, b, &Ejection{Cycle: rec.Cycle, Cause: CauseGate, Err: err})
	}
	return st, nil
}

func (c Composer) apply(ctx context.Context, b Batch, rec DeliverableRecord) (Composed, error) {
	msg := composeMessage(rec)
	tip := b.Tip
	for _, commit := range rec.Commits {
		tree, conflicted, err := c.Repo.mergeTree(ctx, commit.SHA, tip)
		if err != nil {
			return Composed{}, err
		}
		if len(conflicted) > 0 {
			return Composed{}, c.eject(ctx, b, &Ejection{Cycle: rec.Cycle, Cause: CauseConflict, Paths: conflicted, Err: errors.New("textual conflict")})
		}
		if tip, err = c.Repo.commitTree(ctx, tree, b.Tip, msg); err != nil {
			return Composed{}, err
		}
	}
	lane, laneErr := c.Repo.changedPaths(ctx, b.Tip, tip)
	peer, peerErr := c.Repo.changedPaths(ctx, b.Base, b.Tip)
	if err := errors.Join(laneErr, peerErr); err != nil {
		return Composed{}, err
	}
	b.Tip = tip
	return Composed{Batch: b, Proof: c.prove(lane, peer)}, nil
}

func (c Composer) prove(lane, peer []string) overlap.Proof {
	if c.Overlap == nil || c.Overlap.Catalogs == nil {
		return overlap.Proof{Tier: overlap.T3, Rules: []overlap.Rule{overlap.RuleUnknown}}
	}
	return overlap.Prove(overlap.Input{Lane: lane, Peer: peer, Merge: overlap.MergeClean, Module: c.Overlap.Module, Catalogs: c.Overlap.Catalogs})
}

func (c Composer) eject(ctx context.Context, b Batch, e *Ejection) error {
	diff, err := c.Repo.Git(ctx, c.Repo.Dir, "", "diff", "--end-of-options", b.Base, b.Tip)
	if err != nil {
		return fmt.Errorf("integration: read the diff that cycle %d met: %w", e.Cycle, err)
	}
	e.Diff = diff
	return e
}

func composeMessage(rec DeliverableRecord) string {
	return fmt.Sprintf("evolve-cycle %d: integrate\n", rec.Cycle) +
		evidence.Trailer{Cycle: rec.Cycle, ArtifactSHA: rec.Audit.ArtifactSHA}.Build() +
		evidence.KeyAuditTree + ": " + rec.Audit.TreeSHA + "\n"
}

func (r Repo) mergeTree(ctx context.Context, commit, onto string) (string, []string, error) {
	out, err := r.Git(ctx, r.Dir, "", "merge-tree", "--write-tree", "--name-only", "-z", "--merge-base="+commit+"^", "--end-of-options", onto, commit)
	var ge *GitError
	conflict := errors.As(err, &ge) && ge.Code == 1
	if conflict {
		out = ge.Stdout
	} else if err != nil {
		return "", nil, fmt.Errorf("integration: compose %s onto %s: %w", commit, onto, err)
	}
	fields := strings.Split(out, "\x00")
	if !conflict {
		return strings.TrimSpace(fields[0]), nil, nil
	}
	end := slices.Index(fields[1:], "")
	return "", fields[1 : end+1], nil
}

func (r Repo) commitTree(ctx context.Context, tree, parent, msg string) (string, error) {
	if err := fullObjectID(tree); err != nil {
		return "", err
	}
	out, err := r.Git(ctx, r.Dir, msg, "commit-tree", tree, "-p", parent)
	if err != nil {
		return "", fmt.Errorf("integration: commit the tree %s: %w", tree, err)
	}
	return strings.TrimSpace(out), nil
}

func (r Repo) changedPaths(ctx context.Context, from, to string) ([]string, error) {
	out, err := r.Git(ctx, r.Dir, "", "diff-tree", "-r", "--name-only", "-z", "--end-of-options", from, to)
	if err != nil {
		return nil, fmt.Errorf("integration: list the paths from %s to %s: %w", from, to, err)
	}
	return slices.DeleteFunc(strings.Split(out, "\x00"), func(p string) bool { return p == "" }), nil
}
