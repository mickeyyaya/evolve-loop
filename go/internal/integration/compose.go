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
	CauseHead     = "head move"
)

type Batch struct {
	Wave   string
	Pool   int
	Index  int
	Base   string
	Branch string
	Tip    string
}

type Gate func(ctx context.Context, tip string, tier overlap.Tier) error

type GateRed struct {
	Failures []string
}

func (e *GateRed) Error() string {
	return "integration: the gate is red: " + strings.Join(e.Failures, ", ")
}

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
	PR    int
	Cause string
	Paths []string
	Diff  string
	Err   error
}

func (e *Ejection) Error() string {
	return fmt.Sprintf("integration: %s ejected on a %s %v: %v", e.Label(), e.Cause, e.Paths, e.Err)
}

var (
	waveName       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	artifactDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func (e *Ejection) Ref() int {
	if e.PR > 0 {
		return e.PR
	}
	return e.Cycle
}

func (e *Ejection) Label() string {
	if e.PR > 0 {
		return "PR #" + strconv.Itoa(e.PR)
	}
	return "cycle " + strconv.Itoa(e.Cycle)
}

func ejectionOf(rec DeliverableRecord, cause string) *Ejection {
	return &Ejection{Cycle: rec.Cycle, PR: rec.PR, Cause: cause}
}

func batchBranch(wave string, index int) string {
	return "integration/w" + wave + "-b" + strconv.Itoa(index)
}

func branchRef(branch string) string { return "refs/heads/" + branch }

func (b Batch) branchName() string {
	if b.Pool > 0 {
		return PoolBranch(b.Pool, b.Index)
	}
	return batchBranch(b.Wave, b.Index)
}

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
	if b.Branch != b.branchName() {
		return fmt.Errorf("integration: branch %q is not the branch of wave %s batch %d", b.Branch, b.Wave, b.Index)
	}
	return fullObjectID(b.Tip)
}

func (r Repo) StartBatch(ctx context.Context, b Batch) (Batch, error) {
	if err := b.named(); err != nil {
		return Batch{}, err
	}
	b.Branch, b.Tip = b.branchName(), b.Base
	if _, err := r.Git(ctx, r.Dir, "", "update-ref", "--end-of-options", branchRef(b.Branch), b.Base, ""); err != nil {
		return Batch{}, fmt.Errorf("integration: cut %s at %s: %w", b.Branch, b.Base, err)
	}
	return b, nil
}

func checkComposable(rec DeliverableRecord) error {
	if err := rec.Validate(); err != nil {
		return err
	}
	if !rec.Console() && rec.Trailer != CycleTrailer(rec.Cycle) {
		return fmt.Errorf("integration: record trailer %q is not the trailer of cycle %d", rec.Trailer, rec.Cycle)
	}
	if !artifactDigest.MatchString(rec.Audit.ArtifactSHA) {
		return fmt.Errorf("integration: %s has no audit artifact digest", rec.Label())
	}
	var ids []string
	if rec.Audit.TreeSHA != "" || !rec.Console() {
		ids = append(ids, rec.Audit.TreeSHA)
	}
	for _, c := range rec.Commits {
		ids = append(ids, c.SHA, c.BaseSHA)
	}
	for _, id := range ids {
		if err := fullObjectID(id); err != nil {
			return fmt.Errorf("integration: %s: %w", rec.Label(), err)
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
	err = c.Gate(ctx, st.Batch.Tip, st.Proof.Tier)
	var red *GateRed
	switch {
	case errors.As(err, &red):
		return Composed{}, c.eject(ctx, b, withErr(ejectionOf(rec, CauseGate), err))
	case err != nil:
		return Composed{}, fmt.Errorf("integration: the gate of %s did not run: %w", rec.Label(), err)
	}
	return st, nil
}

func (c Composer) apply(ctx context.Context, b Batch, rec DeliverableRecord) (Composed, error) {
	msg, err := c.message(ctx, rec)
	if err != nil {
		return Composed{}, err
	}
	tip := b.Tip
	for _, commit := range rec.Commits {
		tree, conflicted, err := c.Repo.mergeTree(ctx, commit.SHA, tip)
		if err != nil {
			return Composed{}, err
		}
		if len(conflicted) > 0 {
			return Composed{}, c.eject(ctx, b, withPaths(ejectionOf(rec, CauseConflict), conflicted))
		}
		parent := b.Tip
		if rec.Console() {
			parent = tip
		}
		if tip, err = c.Repo.commitTree(ctx, tree, parent, msg); err != nil {
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
	diff, err := c.Repo.Git(ctx, c.Repo.Dir, "", "diff", "--no-ext-diff", "--no-textconv", "--end-of-options", b.Base, b.Tip)
	if err != nil {
		return fmt.Errorf("integration: read the diff that %s met: %w", e.Label(), err)
	}
	e.Diff = diff
	return e
}

func withErr(e *Ejection, err error) *Ejection {
	e.Err = err
	return e
}

func withPaths(e *Ejection, paths []string) *Ejection {
	e.Paths, e.Err = paths, errors.New("textual conflict")
	return e
}

func composeMessage(rec DeliverableRecord) string {
	return fmt.Sprintf("evolve-cycle %d: integrate\n", rec.Cycle) +
		evidence.Trailer{Cycle: rec.Cycle, ArtifactSHA: rec.Audit.ArtifactSHA, AuditTree: rec.Audit.TreeSHA}.Build()
}

func (r Repo) mergeTree(ctx context.Context, commit, onto string) (string, []string, error) {
	if err := fullObjectID(onto); err != nil {
		return "", nil, err
	}
	out, err := r.Git(ctx, r.Dir, "", "--attr-source="+onto, "merge-tree", "--write-tree", "--name-only", "-z", "--merge-base="+commit+"^", "--end-of-options", onto, commit)
	var ge *GitError
	if errors.As(err, &ge) && ge.Code == 1 && ge.Err == nil {
		return conflictedPaths(ge.Stdout)
	}
	if err != nil {
		return "", nil, fmt.Errorf("integration: compose %s onto %s: %w", commit, onto, err)
	}
	return strings.TrimSpace(strings.Split(out, "\x00")[0]), nil, nil
}

func conflictedPaths(out string) (string, []string, error) {
	fields := strings.Split(out, "\x00")
	end := slices.Index(fields, "")
	if end < 2 {
		return "", nil, fmt.Errorf("integration: merge-tree reported a conflict but named no path: %q", out)
	}
	return "", fields[1:end], nil
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
