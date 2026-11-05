package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

const CauseCI = "ci"

type PRHost interface {
	Ready(ctx context.Context) ([]int, error)
	Comment(ctx context.Context, pr int, body string) error
	Close(ctx context.Context, pr int, body string) error
}

type InboxFunc func(ctx context.Context, ids []string) ([]string, error)

type Train struct {
	Composer Composer
	CI       CI
	Main     Main
	Host     PRHost
	Inbox    InboxFunc
	Root     string
	Wave     string
}

type Plan struct {
	Order   []int
	Kept    []DeliverableRecord
	Landed  []DeliverableRecord
	Ejected []*Ejection
}

const (
	refMain         = "refs/evolve/main"
	noBindingPrefix = "cannot record a review binding: "
	alreadyLanded   = "The commits of this PR are already on main. This PR is closed. Its branch is unchanged."
)

type Refusal struct {
	PR     int
	Reason string
}

func (r *Refusal) Error() string {
	return fmt.Sprintf("integration: PR #%d %s", r.PR, r.Reason)
}

func (t Train) ReadyPRs(ctx context.Context) ([]int, error) { return t.Host.Ready(ctx) }

func (t Train) Plan(ctx context.Context, prs []int) (Plan, error) {
	recs, err := t.records(prs)
	if err != nil {
		return Plan{}, err
	}
	recs = orderRecords(recs)
	plan := Plan{Order: refsOf(recs)}
	if err := t.sync(ctx, plan.Order); err != nil {
		return Plan{}, err
	}
	if err := t.verifyHeads(ctx, recs); err != nil {
		return Plan{}, err
	}
	open, err := t.splitLanded(ctx, recs, &plan)
	if err != nil {
		return Plan{}, err
	}
	base, err := t.Main.Head(ctx)
	if err != nil {
		return Plan{}, err
	}
	plan.Kept, plan.Ejected, err = t.dryCompose(ctx, open, base)
	return plan, err
}

func refsOf(recs []DeliverableRecord) []int {
	refs := make([]int, len(recs))
	for i, r := range recs {
		refs[i] = r.Ref()
	}
	return refs
}

func (t Train) prHead(ctx context.Context, pr int) (string, error) {
	git := t.Composer.Repo
	out, err := git.Git(ctx, git.Dir, "", "rev-parse", "--verify", "refs/evolve/pr/"+strconv.Itoa(pr)+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("integration: read the head of PR #%d: %w", pr, err)
	}
	return strings.TrimSpace(out), nil
}

func (t Train) verifyHeads(ctx context.Context, recs []DeliverableRecord) error {
	var refusals []error
	for _, rec := range recs {
		head, err := t.prHead(ctx, rec.PR)
		if err != nil {
			return err
		}
		if head != rec.Commits[len(rec.Commits)-1].SHA {
			refusals = append(refusals, &Refusal{PR: rec.PR, Reason: "head moved since its review was recorded: record the review again"})
		}
	}
	return errors.Join(refusals...)
}

func (t Train) movedHeads(ctx context.Context, recs []DeliverableRecord) ([]*Ejection, error) {
	if err := t.sync(ctx, refsOf(recs)); err != nil {
		return nil, err
	}
	var moved []*Ejection
	for _, rec := range recs {
		ej, err := t.headEjection(ctx, rec, false)
		if err != nil {
			return nil, err
		}
		if ej != nil {
			moved = append(moved, ej)
		}
	}
	return moved, nil
}

func (t Train) headEjection(ctx context.Context, rec DeliverableRecord, landed bool) (*Ejection, error) {
	head, err := t.prHead(ctx, rec.PR)
	if err != nil {
		return nil, err
	}
	last := rec.Commits[len(rec.Commits)-1].SHA
	if head == last {
		return nil, nil
	}
	from := rec.Commits[0].BaseSHA
	if landed {
		from = last
	}
	git := t.Composer.Repo
	out, err := git.Git(ctx, git.Dir, "", "rev-list", "--reverse", "--end-of-options", head, "^"+from)
	if err != nil {
		return nil, fmt.Errorf("integration: list the commits of PR #%d that did not land: %w", rec.PR, err)
	}
	reason := fmt.Errorf("the head moved from %s to %s while the batch ran; the commits that did not land: %s", last, head, strings.Join(strings.Fields(out), " "))
	return withErr(ejectionOf(rec, CauseHead), reason), nil
}

func (t Train) splitLanded(ctx context.Context, recs []DeliverableRecord, plan *Plan) ([]DeliverableRecord, error) {
	var open []DeliverableRecord
	for _, rec := range recs {
		landed, err := t.Composer.Repo.CheckLanded(ctx, rec, refMain)
		if err != nil {
			return nil, fmt.Errorf("integration: check whether %s is on main: %w", rec.Label(), err)
		}
		if landed {
			plan.Landed = append(plan.Landed, rec)
			continue
		}
		open = append(open, rec)
	}
	return open, nil
}

func (t Train) records(prs []int) ([]DeliverableRecord, error) {
	var recs []DeliverableRecord
	var refusals []error
	for _, n := range prs {
		rec, ok := FindPRRecord(t.Root, n)
		if !ok {
			refusals = append(refusals, &Refusal{PR: n, Reason: "has no recorded review binding"})
			continue
		}
		if err := checkComposable(rec); err != nil {
			refusals = append(refusals, &Refusal{PR: n, Reason: "has an unsound review binding: " + err.Error()})
			continue
		}
		recs = append(recs, rec)
	}
	return recs, errors.Join(refusals...)
}

func orderRecords(recs []DeliverableRecord) []DeliverableRecord {
	items := make([]Deliverable, len(recs))
	byID := make(map[string]DeliverableRecord, len(recs))
	for i, r := range recs {
		items[i] = r.Deliverable()
		byID[r.id()] = r
	}
	ordered := make([]DeliverableRecord, 0, len(recs))
	for _, d := range OrderDeliverables(items) {
		ordered = append(ordered, byID[d.ID])
	}
	return ordered
}

func (t Train) sync(ctx context.Context, prs []int) error {
	git := t.Composer.Repo
	if _, err := git.Git(ctx, git.Dir, "", "fetch", "-q", "origin", "+"+branchRef(mainBranch)+":"+refMain); err != nil {
		return fmt.Errorf("integration: fetch origin main: %w", err)
	}
	for _, n := range prs {
		num := strconv.Itoa(n)
		if _, err := git.Git(ctx, git.Dir, "", "fetch", "-q", "origin", "+refs/pull/"+num+"/head:refs/evolve/pr/"+num); err != nil {
			return fmt.Errorf("integration: fetch the head of PR #%d: %w", n, err)
		}
	}
	return nil
}

func (t Train) dryCompose(ctx context.Context, recs []DeliverableRecord, base string) ([]DeliverableRecord, []*Ejection, error) {
	repo := t.Composer.Repo
	b, err := repo.StartBatch(ctx, Batch{Wave: t.Wave + "-plan", Index: 1, Base: base})
	if err != nil {
		return nil, nil, err
	}
	defer repo.dropBranch(ctx, b)
	comp, err := t.Composer.composeAll(ctx, b, recs)
	return comp.Kept, comp.Ejected, err
}

func (c Composer) composeAll(ctx context.Context, b Batch, recs []DeliverableRecord) (Composition, error) {
	comp := Composition{Batch: b}
	for _, rec := range recs {
		st, err := c.Compose(ctx, comp.Batch, rec)
		var ej *Ejection
		switch {
		case errors.As(err, &ej):
			comp.Ejected = append(comp.Ejected, ej)
		case err != nil:
			return Composition{}, err
		default:
			comp.Batch = st.Batch
			comp.Kept = append(comp.Kept, rec)
		}
	}
	return comp, nil
}

func (r Repo) dropBranch(ctx context.Context, b Batch) {
	_, _ = r.Git(ctx, r.Dir, "", "update-ref", "-d", "--end-of-options", branchRef(b.Branch))
}

func (t Train) Land(ctx context.Context, p Plan) (Outcome, error) {
	var errs []error
	var out Outcome
	for _, e := range p.Ejected {
		errs = append(errs, t.Host.Comment(ctx, e.PR, Brief(e)))
		out.Ejected = append(out.Ejected, e.Ref())
	}
	if len(p.Kept) > 0 {
		b := &batcher{t: t}
		m := Merger{CI: t.CI, Main: t.Main, Compose: b.compose, Heads: t.movedHeads, Eject: t.eject}
		merged, err := m.Land(ctx, p.Kept)
		merged.Ejected = append(out.Ejected, merged.Ejected...)
		out = merged
		errs = append(errs, err)
	}
	moved, err := t.settle(ctx, p, out)
	out.Inbox = moved
	return out, errors.Join(append(errs, err)...)
}

func (t Train) settle(ctx context.Context, p Plan, out Outcome) ([]string, error) {
	var errs []error
	var inbox []string
	for _, rec := range p.Kept {
		if via, ok := out.Via[rec.PR]; ok {
			moved, err := t.finish(ctx, rec, via)
			inbox, errs = append(inbox, moved...), append(errs, err)
		}
	}
	for _, rec := range p.Landed {
		moved, err := t.complete(ctx, rec, func() error { return t.Host.Close(ctx, rec.PR, alreadyLanded) })
		inbox, errs = append(inbox, moved...), append(errs, err)
	}
	return inbox, errors.Join(errs...)
}

func (t Train) finish(ctx context.Context, rec DeliverableRecord, via int) ([]string, error) {
	if err := t.sync(ctx, []int{rec.PR}); err != nil {
		return nil, err
	}
	ej, err := t.headEjection(ctx, rec, true)
	if err != nil {
		return nil, err
	}
	if ej == nil {
		return t.complete(ctx, rec, func() error { return t.Host.Close(ctx, rec.PR, landedNote(via)) })
	}
	moved, err := t.complete(ctx, rec, func() error { return t.Host.Comment(ctx, rec.PR, Brief(ej)) })
	return moved, errors.Join(err, &Refusal{PR: rec.PR, Reason: "head moved after the landing: its new commits did not land, and the PR stays open"})
}

func (t Train) complete(ctx context.Context, rec DeliverableRecord, act func() error) ([]string, error) {
	moved, err := t.closeInbox(ctx, rec)
	if err != nil {
		return moved, err
	}
	if err := act(); err != nil {
		return moved, err
	}
	if err := os.Remove(PRRecordPath(WaveDir(t.Root, ConsoleWave), rec.PR)); err != nil {
		return moved, fmt.Errorf("integration: spend the record of PR #%d: %w", rec.PR, err)
	}
	return moved, nil
}

func (t Train) closeInbox(ctx context.Context, rec DeliverableRecord) ([]string, error) {
	ids, err := t.Composer.Repo.closingIDs(ctx, rec.Commits)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	if t.Inbox == nil {
		return nil, fmt.Errorf("integration: %s names inbox items %v and the train has no inbox to close them in", rec.Label(), ids)
	}
	return t.Inbox(ctx, ids)
}

func (t Train) eject(ctx context.Context, rec DeliverableRecord, cause error) error {
	var ej *Ejection
	if !errors.As(cause, &ej) {
		ej = &Ejection{PR: rec.PR, Cause: CauseCI, Err: cause}
	}
	return t.Host.Comment(ctx, rec.PR, Brief(ej))
}

type batcher struct {
	t     Train
	index int
}

func (b *batcher) compose(ctx context.Context, recs []DeliverableRecord) (Composition, error) {
	moved, err := b.t.movedHeads(ctx, recs)
	if err != nil {
		return Composition{}, err
	}
	kept := withoutEjected(recs, moved)
	if len(kept) == 0 {
		return Composition{Ejected: moved}, nil
	}
	base, err := b.t.Main.Head(ctx)
	if err != nil {
		return Composition{}, err
	}
	b.index++
	batch, err := b.t.Composer.Repo.StartBatch(ctx, Batch{Wave: b.t.Wave, Index: b.index, Base: base})
	if err != nil {
		return Composition{}, err
	}
	comp, err := b.t.Composer.composeAll(ctx, batch, kept)
	comp.Ejected = append(moved, comp.Ejected...)
	return comp, err
}

func withoutEjected(recs []DeliverableRecord, ejected []*Ejection) []DeliverableRecord {
	gone := make(map[int]bool, len(ejected))
	for _, ej := range ejected {
		gone[ej.Ref()] = true
	}
	return slices.DeleteFunc(slices.Clone(recs), func(r DeliverableRecord) bool { return gone[r.Ref()] })
}

func landedNote(batch int) string {
	return "Landed in batch PR #" + strconv.Itoa(batch) + ". The commits of this PR are on main in one fast-forward. This PR is closed. Its branch is unchanged."
}

func Brief(e *Ejection) string {
	var b strings.Builder
	fmt.Fprintf(&b, "The integration batch ejected %s on a %s.\n", e.Label(), e.Cause)
	if e.Err != nil {
		fmt.Fprintf(&b, "Reason: %v\n", e.Err)
	}
	for _, p := range e.Paths {
		fmt.Fprintf(&b, "- %s\n", p)
	}
	b.WriteString("This PR stays open and its branch is unchanged. Rebase it on main, then record its review again.\n")
	return b.String()
}

func (t Train) Record(ctx context.Context, pr int, bind Binder) (string, error) {
	if err := t.sync(ctx, []int{pr}); err != nil {
		return "", err
	}
	commits, err := t.prCommits(ctx, pr)
	if err != nil {
		return "", err
	}
	ev, err := t.evidence(ctx, commits)
	if err != nil {
		return "", err
	}
	if ev.DiffSHA == Digest(nil) {
		return "", &Refusal{PR: pr, Reason: noBindingPrefix + "its commits have no net change. Next: close the PR."}
	}
	binding, err := bind(ev)
	if err != nil {
		return "", &Refusal{PR: pr, Reason: noBindingPrefix + err.Error()}
	}
	rec := DeliverableRecord{Source: SourceConsolePR, Wave: ConsoleWave, PR: pr, Commits: commits, Audit: binding}
	return WriteRecord(WaveDir(t.Root, ConsoleWave), rec)
}

func (t Train) evidence(ctx context.Context, commits []CommitIdentity) (Evidence, error) {
	repo := t.Composer.Repo
	first, last := commits[0].BaseSHA, commits[len(commits)-1].SHA
	diff, err := repo.Git(ctx, repo.Dir, "", "diff", "--no-ext-diff", "--no-textconv", "--end-of-options", first, last)
	if err != nil {
		return Evidence{}, fmt.Errorf("integration: read the change of %s..%s: %w", first, last, err)
	}
	paths, err := repo.changedPaths(ctx, first, last)
	if err != nil {
		return Evidence{}, err
	}
	ev := Evidence{Base: first, Paths: paths, DiffSHA: Digest([]byte(diff))}
	for _, c := range commits {
		ev.Commits = append(ev.Commits, c.SHA)
	}
	return ev, nil
}

func (t Train) prCommits(ctx context.Context, pr int) ([]CommitIdentity, error) {
	repo := t.Composer.Repo
	out, err := repo.Git(ctx, repo.Dir, "", "rev-list", "--reverse", "--parents", "refs/evolve/main..refs/evolve/pr/"+strconv.Itoa(pr))
	if err != nil {
		return nil, fmt.Errorf("integration: list the commits of PR #%d: %w", pr, err)
	}
	var commits []CommitIdentity
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		sha := fields[0]
		if len(fields) > 2 {
			return nil, &Refusal{PR: pr, Reason: noBindingPrefix + mergeReason(sha, fields[1:])}
		}
		names, err := repo.Git(ctx, repo.Dir, "", "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", "--end-of-options", sha)
		if err != nil {
			return nil, fmt.Errorf("integration: read the files of %s: %w", sha, err)
		}
		if strings.TrimSpace(names) == "" {
			return nil, &Refusal{PR: pr, Reason: noBindingPrefix + "commit " + ShortSHA(sha) + " changes nothing. Next: put the other commits on a new branch from origin/main without it."}
		}
		parent, err := repo.Git(ctx, repo.Dir, "", "rev-parse", sha+"^")
		if err != nil {
			return nil, fmt.Errorf("integration: read the parent of %s: %w", sha, err)
		}
		patch, err := repo.PatchID(ctx, sha)
		if err != nil {
			return nil, err
		}
		commits = append(commits, CommitIdentity{SHA: sha, BaseSHA: strings.TrimSpace(parent), PatchID: patch})
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("integration: PR #%d has no commits on main", pr)
	}
	return commits, nil
}

func ShortSHA(sha string) string {
	return sha[:min(7, len(sha))]
}

func mergeReason(sha string, parents []string) string {
	short := make([]string, len(parents))
	for i, p := range parents {
		short[i] = ShortSHA(p)
	}
	return "commit " + ShortSHA(sha) + " is a merge of " + strings.Join(short, " and ") + ", and the train composes a linear PR. Next: put the commits on a new branch from origin/main, open a PR, and run record-pr on it."
}
