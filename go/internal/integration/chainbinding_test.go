package integration

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
)

func capture(t *testing.T, r *trainRig, pr int) Evidence {
	t.Helper()
	var got Evidence
	bind := func(ev Evidence) (AuditBinding, error) {
		got = ev
		return ClassBinding(ev, "docs-review", allPaths)
	}
	if _, err := r.train.Record(context.Background(), pr, bind); err != nil {
		t.Fatalf("Record: %v", err)
	}
	return got
}

func twoCommitPR(t *testing.T, r *trainRig, pr int) DeliverableRecord {
	t.Helper()
	rec := r.f.consoleSteps(t, pr, map[string]string{"a.txt": "a\n"}, map[string]string{"b.txt": "b\n"})
	r.f.g.Git("push", "-q", "origin", "pr-7:refs/pull/7/head")
	return rec
}

func TestEvidence_CarriesTheChangeOfEachCommit(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := twoCommitPR(t, r, 7)
	ev := capture(t, r, 7)
	first, second := rec.Commits[0].SHA, rec.Commits[1].SHA
	if !slices.Equal(ev.Commits, []string{first, second}) {
		t.Fatalf("Commits = %v, want the ordered SHAs", ev.Commits)
	}
	raw := func(from, to string) string {
		out, err := r.f.repo.Git(context.Background(), r.f.g.Dir, "", "diff", from, to)
		if err != nil {
			t.Fatal(err)
		}
		return Digest([]byte(out))
	}
	want := []CommitChange{
		{SHA: first, BaseSHA: r.f.base, DiffSHA: raw(r.f.base, first), Paths: []string{"a.txt"}},
		{SHA: second, BaseSHA: first, DiffSHA: raw(first, second), Paths: []string{"b.txt"}},
	}
	if len(ev.Changes) != 2 {
		t.Fatalf("Changes = %+v, want 2 entries", ev.Changes)
	}
	for i, w := range want {
		g := ev.Changes[i]
		if g.SHA != w.SHA || g.BaseSHA != w.BaseSHA || g.DiffSHA != w.DiffSHA || !slices.Equal(g.Paths, w.Paths) {
			t.Errorf("Changes[%d] = %+v, want %+v", i, g, w)
		}
	}
}

func TestEvidence_AFailedCommitChangeIsReported(t *testing.T) {
	t.Parallel()
	cases := map[string]func(args []string, nameCalls *int) bool{
		"the diff of a commit": func(args []string, _ *int) bool { return len(args) == 3 && args[0] == "diff" },
		"the paths of a commit": func(args []string, n *int) bool {
			if args[0] != "diff-tree" || !slices.Contains(args, "-z") {
				return false
			}
			*n++
			return *n == 2
		},
	}
	for name, fails := range cases {
		r := newTrainRig(t)
		twoCommitPR(t, r, 7)
		real := r.train.Composer.Repo.Git
		n := 0
		r.train.Composer.Repo.Git = func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
			if fails(args, &n) {
				return "", errors.New("git down")
			}
			return real(ctx, dir, stdin, args...)
		}
		if _, err := r.train.Record(context.Background(), 7, classBinder("docs-review")); err == nil {
			t.Errorf("a failure of %s was hidden", name)
		}
	}
}

func chainEvidence() Evidence {
	return Evidence{
		Base:    strings.Repeat("0", 39) + "1",
		Commits: []string{strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)},
		DiffSHA: strings.Repeat("d", 64),
		Changes: []CommitChange{
			{SHA: strings.Repeat("a", 40), BaseSHA: strings.Repeat("0", 39) + "1", DiffSHA: strings.Repeat("1", 64)},
			{SHA: strings.Repeat("b", 40), BaseSHA: strings.Repeat("a", 40), DiffSHA: strings.Repeat("2", 64)},
			{SHA: strings.Repeat("c", 40), BaseSHA: strings.Repeat("b", 40), DiffSHA: strings.Repeat("3", 64)},
		},
	}
}

func TestChainedBinding_BindsEveryCommitRecordInOrder(t *testing.T) {
	t.Parallel()
	ev := chainEvidence()
	var seen []string
	var check ChangeCheck = func(c CommitChange) (string, error) {
		seen = append(seen, c.DiffSHA)
		return "rec-" + c.DiffSHA[:1], nil
	}
	got, err := ChainedBinding(ev, check)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{ev.Changes[0].DiffSHA, ev.Changes[1].DiffSHA, ev.Changes[2].DiffSHA}; !slices.Equal(seen, want) {
		t.Errorf("checked %v, want %v in order", seen, want)
	}
	wantArtifact := digest("rec-1", "rec-2", "rec-3")
	if got.Kind != KindCommitChain || got.TreeSHA != ev.DiffSHA || got.ArtifactSHA != wantArtifact || got.Class != "" {
		t.Errorf("binding = %+v, want kind %s tree %s artifact %s", got, KindCommitChain, ev.DiffSHA, wantArtifact)
	}
	reordered := chainEvidence()
	reordered.Changes[0], reordered.Changes[1] = reordered.Changes[1], reordered.Changes[0]
	other, err := ChainedBinding(reordered, func(c CommitChange) (string, error) { return "rec-" + c.DiffSHA[:1], nil })
	if err != nil || other.ArtifactSHA == got.ArtifactSHA {
		t.Errorf("the artifact digest must depend on the order: %v %+v", err, other)
	}
}

func TestChainedBinding_ACommitWithNoRecordIsRefusedByName(t *testing.T) {
	t.Parallel()
	ev := chainEvidence()
	calls := 0
	_, err := ChainedBinding(ev, func(c CommitChange) (string, error) {
		calls++
		if c.SHA == ev.Changes[1].SHA {
			return "", errors.New("no review record in /store")
		}
		return "rec", nil
	})
	if err == nil {
		t.Fatal("a commit with no record was bound")
	}
	if calls != 2 {
		t.Errorf("check ran %d times, want 2: the first failure stops the walk", calls)
	}
	for _, want := range []string{"commit " + ShortSHA(ev.Changes[1].SHA) + " (change " + ShortSHA(ev.Changes[1].DiffSHA) + ")", "no review record in /store", "at " + ShortSHA(ev.Changes[1].BaseSHA), "git restore --source=" + ShortSHA(ev.Changes[1].SHA) + " --staged --worktree -- .", "evolve commit-gate run"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal lacks %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), ShortSHA(ev.Changes[2].SHA)) {
		t.Errorf("the refusal names a commit after the first failure: %v", err)
	}
	if _, err := ChainedBinding(Evidence{DiffSHA: ev.DiffSHA}, func(CommitChange) (string, error) { return "", nil }); err == nil {
		t.Error("an evidence with no commit change was bound")
	}
}

func TestChainedBinding_TheKindNamesTheChain(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := twoCommitPR(t, r, 7)
	bind := func(ev Evidence) (AuditBinding, error) {
		return ChainedBinding(ev, func(c CommitChange) (string, error) { return Digest([]byte(c.SHA)), nil })
	}
	path, err := r.train.Record(context.Background(), 7, bind)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(data)
	if !strings.Contains(raw, `"kind": "commit-chain"`) {
		t.Errorf("the record JSON lacks the kind: %s", raw)
	}
	got, ok := FindPRRecord(r.root, 7)
	if !ok || got.Audit.Kind != KindCommitChain || got.Audit.Class != "" {
		t.Fatalf("record = %+v, %v", got, ok)
	}
	if err := checkComposable(got); err != nil {
		t.Errorf("a chain record is not composable: %v", err)
	}
	msg, err := r.train.Composer.message(context.Background(), got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "\nEvolve-Audit-Kind: commit-chain\n") {
		t.Errorf("the composed message lacks the kind trailer: %q", msg)
	}
	whole := rec
	whole.Audit = AuditBinding{TreeSHA: treeSHA40, ArtifactSHA: fixtureArtifact}
	plain, err := r.train.Composer.message(context.Background(), whole)
	if err != nil || strings.Contains(plain, "Evolve-Audit-Kind") {
		t.Errorf("a whole-change record must carry no kind trailer: %v %q", err, plain)
	}
}

func TestChainedBinding_AMalformedKindIsRefused(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := r.f.consoleSteps(t, 7, map[string]string{"a.txt": "a\n"})
	good := AuditBinding{Kind: KindCommitChain, TreeSHA: strings.Repeat("d", 64), ArtifactSHA: fixtureArtifact}
	cases := map[string]func(*AuditBinding){
		"an unknown kind":      func(a *AuditBinding) { a.Kind = "forged" },
		"a chain with a class": func(a *AuditBinding) { a.Class = "docs-review" },
		"a chain with no tree": func(a *AuditBinding) { a.TreeSHA = "" },
	}
	if err := checkComposable(withAudit(rec, good)); err != nil {
		t.Fatalf("the good chain binding: %v", err)
	}
	for name, mutate := range cases {
		a := good
		mutate(&a)
		if err := withAudit(rec, a).bindingErr(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func withAudit(rec DeliverableRecord, a AuditBinding) DeliverableRecord {
	rec.Audit = a
	return rec
}

func TestRepoRunner_CarriesGitOutputAndExitCode(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	repo := Repo{Git: func(context.Context, string, string, ...string) (string, error) { return "plain", nil }}
	if code, err := repo.runner()(context.Background(), "git", "", nil, nil, nil, &out, io.Discard); code != 0 || err != nil || out.String() != "plain" {
		t.Errorf("success = %d %v %q", code, err, out.String())
	}
	out.Reset()
	repo.Git = func(context.Context, string, string, ...string) (string, error) {
		return "", &GitError{Code: 1, Stdout: "partial"}
	}
	if code, err := repo.runner()(context.Background(), "git", "", nil, nil, nil, &out, io.Discard); code != 1 || err != nil || out.String() != "partial" {
		t.Errorf("exit 1 = %d %v %q", code, err, out.String())
	}
	repo.Git = func(context.Context, string, string, ...string) (string, error) { return "", errors.New("spawn") }
	if _, err := repo.runner()(context.Background(), "git", "", nil, nil, nil, &out, io.Discard); err == nil {
		t.Error("a spawn failure was hidden")
	}
}

func TestRepoRunner_AnExitCodeOtherThanOneIsCarriedWithoutAnError(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	repo := Repo{Git: func(context.Context, string, string, ...string) (string, error) {
		return "", &GitError{Code: 128, Stdout: ""}
	}}
	if code, err := repo.runner()(context.Background(), "git", "", nil, nil, nil, &out, io.Discard); code != 128 || err != nil {
		t.Errorf("exit 128 = %d %v", code, err)
	}
}

func TestRepoRunner_CarriesTheGitMessageToTheStderrWriter(t *testing.T) {
	t.Parallel()
	var out, errw strings.Builder
	repo := Repo{Git: func(context.Context, string, string, ...string) (string, error) {
		return "", &GitError{Code: 128, Stderr: "fatal: bad revision"}
	}}
	if code, err := repo.runner()(context.Background(), "git", "", nil, nil, nil, &out, &errw); code != 128 || err != nil || errw.String() != "fatal: bad revision" {
		t.Errorf("exit 128 = %d %v stderr %q", code, err, errw.String())
	}
}

func TestRestoreStep_NamesTheWorktreeAndTheSource(t *testing.T) {
	t.Parallel()
	want := "Next: in a worktree at aaa, run git restore --source=bbb --staged --worktree -- . , run the review seats and evolve commit-gate run, then run record-pr again."
	if got := RestoreStep("aaa", "bbb"); got != want {
		t.Errorf("RestoreStep = %q, want %q", got, want)
	}
}
