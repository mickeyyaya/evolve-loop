package integration

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func classBinder(class string) Binder {
	return func(ev Evidence) (AuditBinding, error) { return ClassBinding(ev, class, allPaths) }
}

func TestTrainRecordWritesTheRecordOfAPR(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	want := r.f.console(t, 6, "feat: a\n\nCloses-Inbox: item-a", map[string]string{"a.txt": "a\n"})
	r.f.g.Git("push", "-q", "origin", "pr-6:refs/pull/6/head")
	path, err := r.train.Record(context.Background(), 6, classBinder("docs-review"))
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if path != PRRecordPath(WaveDir(r.root, ConsoleWave), 6) {
		t.Errorf("path = %s", path)
	}
	got, ok := FindPRRecord(r.root, 6)
	if !ok || got.Cycle != 0 || got.Source != SourceConsolePR || got.Audit.Class != "docs-review" {
		t.Fatalf("record = %+v, %v", got, ok)
	}
	if len(got.Commits) != 1 || got.Commits[0].SHA != want.Commits[0].SHA || got.Commits[0].BaseSHA != r.f.base || got.Commits[0].PatchID != want.Commits[0].PatchID {
		t.Errorf("commits = %+v, want %+v", got.Commits, want.Commits)
	}
}

func TestTrainRecordFailsLoudly(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	ctx := context.Background()
	if _, err := r.train.Record(ctx, 8, classBinder("docs-review")); err == nil || !strings.Contains(err.Error(), "PR #8") {
		t.Errorf("a PR with no head: %v", err)
	}
	r.f.console(t, 9, "feat: a", map[string]string{"a.txt": "a\n"})
	r.f.g.Git("push", "-q", "origin", "pr-9:refs/pull/9/head")
	refuse := func(Evidence) (AuditBinding, error) { return AuditBinding{}, errors.New("no review") }
	if _, err := r.train.Record(ctx, 9, refuse); err == nil || !strings.Contains(err.Error(), "no review") {
		t.Errorf("a refused binding: %v", err)
	}
	r.f.g.Git("push", "-q", "origin", "main:refs/pull/10/head")
	if _, err := r.train.Record(ctx, 10, classBinder("docs-review")); err == nil || !strings.Contains(err.Error(), "no commits") {
		t.Errorf("a PR with no commits: %v", err)
	}
	bad := r.train
	bad.Composer.Repo.Git = func(context.Context, string, string, ...string) (string, error) { return "", errors.New("git down") }
	if _, err := bad.Record(ctx, 9, classBinder("docs-review")); err == nil {
		t.Error("a git failure was hidden")
	}
}

func TestTrainRecordReportsEachGitStepThatFails(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"rev-list", "rev-parse", "diff-tree", "diff"} {
		r := newTrainRig(t)
		r.f.console(t, 9, "feat: a", map[string]string{"a.txt": "a\n"})
		r.f.g.Git("push", "-q", "origin", "pr-9:refs/pull/9/head")
		real := r.train.Composer.Repo.Git
		r.train.Composer.Repo.Git = func(ctx context.Context, dir, in string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == verb {
				return "", errors.New(verb + " failed")
			}
			return real(ctx, dir, in, args...)
		}
		if _, err := r.train.Record(context.Background(), 9, classBinder("docs-review")); err == nil || !strings.Contains(err.Error(), verb+" failed") {
			t.Errorf("%s: %v", verb, err)
		}
	}
}

func TestTrainRecordHandsTheBinderTheEvidenceOfThePR(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := r.f.console(t, 6, "feat: a", map[string]string{"docs/a.md": "a\n", "b.go": "package b\n"})
	r.f.g.Git("push", "-q", "origin", "pr-6:refs/pull/6/head")
	var got Evidence
	spy := func(ev Evidence) (AuditBinding, error) {
		got = ev
		return ClassBinding(ev, "docs-review", allPaths)
	}
	if _, err := r.train.Record(context.Background(), 6, spy); err != nil {
		t.Fatal(err)
	}
	diff := r.f.g.Git("diff", r.f.base, rec.Commits[0].SHA) + "\n"
	if got.DiffSHA != Digest([]byte(diff)) || !slices.Equal(got.Commits, []string{rec.Commits[0].SHA}) || !slices.Equal(got.Paths, []string{"b.go", "docs/a.md"}) {
		t.Errorf("evidence = %+v, want the digest of the PR diff, its commit and its paths", got)
	}
}

func TestTrainRecordRefusesAnAttestationOfAnotherChange(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.f.console(t, 6, "feat: a", map[string]string{"a.md": "a\n"})
	r.f.g.Git("push", "-q", "origin", "pr-6:refs/pull/6/head")
	wrong := func(ev Evidence) (AuditBinding, error) {
		return AttestedBinding(ev, Digest([]byte("elsewhere")), Digest([]byte("raw")))
	}
	_, err := r.train.Record(context.Background(), 6, wrong)
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.PR != 6 {
		t.Fatalf("Record = %v, want a refusal of PR 6", err)
	}
	if _, ok := FindPRRecord(r.root, 6); ok {
		t.Error("a refused binding left a record")
	}
}

func TestTrainRecordReportsAFailedPathListing(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.f.console(t, 9, "feat: a", map[string]string{"a.txt": "a\n"})
	r.f.g.Git("push", "-q", "origin", "pr-9:refs/pull/9/head")
	real := r.train.Composer.Repo.Git
	r.train.Composer.Repo.Git = func(ctx context.Context, dir, in string, args ...string) (string, error) {
		if slices.Contains(args, "--name-only") {
			return "", errors.New("listing failed")
		}
		return real(ctx, dir, in, args...)
	}
	if _, err := r.train.Record(context.Background(), 9, classBinder("docs-review")); err == nil || !strings.Contains(err.Error(), "listing failed") {
		t.Errorf("Record = %v, want the listing error", err)
	}
}
