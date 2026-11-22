package integration

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type stalePullRefRig struct {
	*trainRig
	rec   DeliverableRecord
	stale string
	tip   string
}

func newStalePullRefRig(t *testing.T) *stalePullRefRig {
	t.Helper()
	r := newTrainRig(t)
	rec := r.f.consoleSteps(t, 6, map[string]string{"a.md": "a\n"}, map[string]string{"b.md": "b\n"})
	stale, tip := rec.Commits[0].SHA, rec.Commits[1].SHA
	r.f.g.Git("push", "-q", "origin", "pr-6:refs/heads/pr-6")
	r.f.g.Git("push", "-q", "origin", stale+":refs/pull/6/head")
	r.train.Host = GhHost{CI: GhCI{Repo: r.f.repo, Gh: r.headBranchGh(6, "pr-6")}}
	return &stalePullRefRig{trainRig: r, rec: rec, stale: stale, tip: tip}
}

func (r *trainRig) headBranchGh(pr int, branch string) GhFunc {
	prefix := "pr view " + strconv.Itoa(pr) + " "
	return func(_ context.Context, _ string, args ...string) (string, int, error) {
		if !strings.HasPrefix(strings.Join(args, " "), prefix) {
			return "", 1, nil
		}
		oid := strings.TrimSpace(r.origin.Git("rev-parse", "refs/heads/"+branch))
		return `{"state":"OPEN","number":` + strconv.Itoa(pr) + `,"headRefName":"` + branch + `","headRefOid":"` + oid + `"}` + "\n", 0, nil
	}
}

func withPullRefWait(t *testing.T, train *Train, wait time.Duration) {
	t.Helper()
	field := reflect.ValueOf(train).Elem().FieldByName("PullRefWait")
	if !field.IsValid() || !field.CanSet() || field.Type() != reflect.TypeOf(wait) {
		t.Fatalf("Train needs an exported PullRefWait time.Duration field: the longest time that record-pr waits for the pull ref of a PR to reach its head branch")
	}
	field.SetInt(int64(wait))
}

func (r *stalePullRefRig) catchUpOnFetch(n int) *int {
	var mu sync.Mutex
	fetches := 0
	real := r.train.Composer.Repo.Git
	r.train.Composer.Repo.Git = func(ctx context.Context, dir, in string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "fetch" && strings.Contains(strings.Join(args, " "), "refs/pull/6/head") {
			mu.Lock()
			fetches++
			if fetches == n {
				r.origin.Git("update-ref", "refs/pull/6/head", r.tip)
			}
			mu.Unlock()
		}
		return real(ctx, dir, in, args...)
	}
	return &fetches
}

func TestRecordPRStalePullRefWritesNoRecordOfTheOldCommitList(t *testing.T) {
	r := newStalePullRefRig(t)
	path, err := r.train.Record(context.Background(), 6, classBinder("docs-review"))
	if err == nil {
		t.Fatalf("Record accepted a pull ref one commit behind its head branch and wrote %s", path)
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.PR != 6 {
		t.Errorf("Record = %v, want a refusal of PR 6, which record-pr turns into exit 1", err)
	}
	if rec, ok := FindPRRecord(r.root, 6); ok {
		t.Errorf("a stale pull ref left a record of %d commit(s)", len(rec.Commits))
	}
}

func TestRecordPRStalePullRefThatNeverCatchesUpNamesBothSHAs(t *testing.T) {
	r := newStalePullRefRig(t)
	withPullRefWait(t, &r.train, 300*time.Millisecond)
	_, err := r.train.Record(context.Background(), 6, classBinder("docs-review"))
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.PR != 6 {
		t.Fatalf("Record = %v, want a refusal of PR 6", err)
	}
	for name, sha := range map[string]string{"the pull ref": r.stale, "the head branch": r.tip} {
		if !strings.Contains(refusal.Reason, ShortSHA(sha)) {
			t.Errorf("the refusal does not name the SHA of %s (%s): %s", name, ShortSHA(sha), refusal.Reason)
		}
	}
	if _, ok := FindPRRecord(r.root, 6); ok {
		t.Error("a pull ref that never caught up left a record")
	}
}

func TestRecordPRStalePullRefThatCatchesUpInsideTheBoundRecordsTheFullCommitList(t *testing.T) {
	r := newStalePullRefRig(t)
	withPullRefWait(t, &r.train, 10*time.Second)
	fetches := r.catchUpOnFetch(2)
	if _, err := r.train.Record(context.Background(), 6, classBinder("docs-review")); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if *fetches < 2 {
		t.Errorf("the pull ref was fetched %d time(s); record-pr must fetch it again before it gives up", *fetches)
	}
	got, ok := FindPRRecord(r.root, 6)
	if !ok || len(got.Commits) != 2 || got.Commits[0].SHA != r.stale || got.Commits[1].SHA != r.tip {
		t.Fatalf("record = %+v, %v, want the two commits [%s %s]", got.Commits, ok, ShortSHA(r.stale), ShortSHA(r.tip))
	}
}

func TestRecordPRStalePullRefDoesNotRefuseAPullRefThatIsCurrent(t *testing.T) {
	r := newStalePullRefRig(t)
	r.origin.Git("update-ref", "refs/pull/6/head", r.tip)
	if _, err := r.train.Record(context.Background(), 6, classBinder("docs-review")); err != nil {
		t.Fatalf("Record refused a pull ref equal to its head branch: %v", err)
	}
	got, ok := FindPRRecord(r.root, 6)
	if !ok || len(got.Commits) != 2 {
		t.Errorf("record = %+v, %v, want two commits", got.Commits, ok)
	}
}
