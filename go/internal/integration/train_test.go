package integration

import (
	"context"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type fakeHost struct {
	mu         sync.Mutex
	ready      []int
	readyErr   error
	commentErr error
	closeErr   error
	comments   map[int][]string
	closed     map[int]string
}

func (h *fakeHost) Ready(context.Context) ([]int, error) { return h.ready, h.readyErr }

func (h *fakeHost) Comment(_ context.Context, pr int, body string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.comments == nil {
		h.comments = map[int][]string{}
	}
	h.comments[pr] = append(h.comments[pr], body)
	return h.commentErr
}

func (h *fakeHost) Close(_ context.Context, pr int, body string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed == nil {
		h.closed = map[int]string{}
	}
	h.closed[pr] = body
	return h.closeErr
}

type trainRig struct {
	f      composeFixture
	origin *gittest.Repo
	root   string
	host   *fakeHost
	ci     *fakeCI
	train  Train
	inbox  []string
}

func newTrainRig(t *testing.T) *trainRig {
	t.Helper()
	f := newComposeFixture(t)
	origin := gittest.Bare(t)
	f.g.Git("remote", "add", "origin", origin.Dir)
	f.g.Git("push", "-q", "origin", "main")
	r := &trainRig{f: f, origin: origin, root: t.TempDir(), host: &fakeHost{}, ci: &fakeCI{}}
	var inbox InboxFunc = func(_ context.Context, ids []string) ([]string, error) {
		r.inbox = append(r.inbox, ids...)
		return ids, nil
	}
	r.train = Train{
		Composer: Composer{Repo: f.repo, Stage: "shadow"},
		CI:       r.ci,
		Main:     GitMain{Repo: f.repo},
		Host:     r.host,
		Inbox:    inbox,
		Root:     r.root,
		Wave:     "t1",
	}
	return r
}

func (r *trainRig) pr(t *testing.T, n int, message string, files map[string]string) DeliverableRecord {
	t.Helper()
	rec := r.f.console(t, n, message, files)
	r.f.g.Git("push", "-q", "origin", "pr-"+strconv.Itoa(n)+":refs/pull/"+strconv.Itoa(n)+"/head")
	if _, err := WriteRecord(WaveDir(r.root, ConsoleWave), rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func (r *trainRig) originMain() string {
	return strings.TrimSpace(r.origin.Git("rev-parse", "refs/heads/main"))
}

const (
	lineOne   = "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
	lineOneX  = "X\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
	lineOneY  = "Y\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
	lineTenOK = "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nTEN\n"
)

func TestTrainPlanOrdersAndEjectsTheConflict(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", map[string]string{"a.txt": "a\n"})
	r.pr(t, 2, "feat: x", map[string]string{sharedFile: lineOneX})
	r.pr(t, 3, "feat: y", map[string]string{sharedFile: lineOneY})
	before := r.originMain()
	plan, err := r.train.Plan(context.Background(), []int{3, 1, 2})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !slices.Equal(plan.Order, []int{1, 2, 3}) {
		t.Errorf("Order = %v, want [1 2 3]", plan.Order)
	}
	if len(plan.Kept) != 2 || plan.Kept[0].PR != 1 || plan.Kept[1].PR != 2 {
		t.Errorf("Kept = %v", plan.Kept)
	}
	if len(plan.Ejected) != 1 || plan.Ejected[0].PR != 3 || plan.Ejected[0].Cause != CauseConflict || !slices.Equal(plan.Ejected[0].Paths, []string{sharedFile}) {
		t.Errorf("Ejected = %+v", plan.Ejected)
	}
	if r.originMain() != before || len(r.ci.opened) != 0 || len(r.host.comments)+len(r.host.closed) != 0 {
		t.Error("a plan must push nothing and touch no PR")
	}
	if gittest.BranchExists(t, r.f.g.Dir, "integration/wt1-plan-b1") {
		t.Error("the plan branch must be deleted")
	}
}

func TestTrainPlanRefusesAPRWithoutAReviewBinding(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	_, err := r.train.Plan(context.Background(), []int{1, 9, 12})
	if err == nil {
		t.Fatal("Plan accepted PRs with no record")
	}
	for _, want := range []string{"PR #9", "PR #12", "no recorded review binding"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal lacks %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "PR #1 ") {
		t.Errorf("the reviewed PR 1 must not be named: %v", err)
	}
}

func TestTrainPlanRefusesAnUnsoundBinding(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	rec.Audit.TreeSHA = "zz"
	if _, err := WriteRecord(WaveDir(r.root, ConsoleWave), rec); err != nil {
		t.Fatal(err)
	}
	if _, err := r.train.Plan(context.Background(), []int{1}); err == nil || !strings.Contains(err.Error(), "PR #1") {
		t.Fatalf("Plan = %v, want a refusal naming PR #1", err)
	}
}

func TestTrainPlanFailsLoudlyWhenAPRHeadIsMissing(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.origin.Git("update-ref", "-d", "refs/pull/1/head")
	if _, err := r.train.Plan(context.Background(), []int{1}); err == nil || !strings.Contains(err.Error(), "PR #1") {
		t.Fatalf("Plan = %v, want a fetch error naming PR #1", err)
	}
}

func TestTrainPlanFailsLoudlyWhenMainCannotBeRead(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.f.g.Git("remote", "set-url", "origin", r.root+"/missing")
	if _, err := r.train.Plan(context.Background(), []int{1}); err == nil {
		t.Fatal("Plan ignored an unreachable origin")
	}
}

func TestTrainPlanSurfacesAComposeErrorThatIsNotAnEjection(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.train.Composer.Stage = "off"
	if _, err := r.train.Plan(context.Background(), []int{1}); err == nil || !strings.Contains(err.Error(), "stage") {
		t.Fatalf("Plan = %v, want a stage error", err)
	}
}

func TestTrainLandsTwoPRsWithOneFastForwardAndBriefsTheThird(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", map[string]string{"a.txt": "a\n"})
	r.pr(t, 2, "feat: x", map[string]string{sharedFile: lineOneX})
	r.pr(t, 3, "feat: y", map[string]string{sharedFile: lineOneY})
	before := r.originMain()
	plan, err := r.train.Plan(context.Background(), []int{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.train.Land(context.Background(), plan)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !slices.Equal(out.Landed, []int{1, 2}) {
		t.Errorf("Landed = %v, want [1 2]", out.Landed)
	}
	after := r.originMain()
	if after == before || strings.TrimSpace(r.origin.Git("rev-list", "--count", before+".."+after)) != "2" {
		t.Errorf("main must advance by the two PRs in one fast-forward: %s -> %s", before, after)
	}
	if strings.TrimSpace(r.origin.Git("merge-base", before, after)) != before {
		t.Error("the move of main is not a fast-forward")
	}
	if log := r.origin.Git("log", "--format=%B", before+".."+after); !strings.Contains(log, "Closes-Inbox: item-a") {
		t.Errorf("the Closes-Inbox trailer is lost:\n%s", log)
	}
	for _, pr := range []int{1, 2} {
		if body, ok := r.host.closed[pr]; !ok || !strings.Contains(body, "#1") {
			t.Errorf("PR %d close comment = %q, want a link to batch #1", pr, body)
		}
	}
	if _, closed := r.host.closed[3]; closed {
		t.Error("the ejected PR must stay open")
	}
	brief := strings.Join(r.host.comments[3], "\n")
	if !strings.Contains(brief, sharedFile) || !strings.Contains(brief, CauseConflict) {
		t.Errorf("PR 3 brief = %q", brief)
	}
}

func TestTrainLandWithNothingKeptOpensNoBatch(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	out, err := r.train.Land(context.Background(), Plan{Ejected: []*Ejection{{PR: 5, Cause: CauseGate, Err: errors.New("red")}}})
	if err != nil || len(out.Landed) != 0 || len(r.ci.opened) != 0 {
		t.Fatalf("Land = %+v, %v, opened %d", out, err, len(r.ci.opened))
	}
	if len(r.host.comments[5]) != 1 {
		t.Errorf("PR 5 must get its brief: %v", r.host.comments)
	}
}

func TestTrainLandBriefsTheSingleRedPRAndLandsTheRest(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.pr(t, 2, "feat: b", map[string]string{"b.txt": "b\n"})
	r.ci.red = func(b Batch, _ int) bool {
		return strings.Contains(r.f.g.Git("ls-tree", "-r", "--name-only", b.Tip), "b.txt")
	}
	plan, err := r.train.Plan(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.train.Land(context.Background(), plan)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !slices.Equal(out.Landed, []int{1}) || !slices.Equal(out.Ejected, []int{2}) {
		t.Errorf("Landed %v Ejected %v, want [1] and [2]", out.Landed, out.Ejected)
	}
	if _, ok := r.host.closed[1]; !ok {
		t.Error("PR 1 landed and must close")
	}
	if got := strings.Join(r.host.comments[2], "\n"); !strings.Contains(got, "CI") {
		t.Errorf("PR 2 brief = %q, want a CI red brief", got)
	}
}

func TestTrainLandReportsAFailedCloseAndAFailedBrief(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	plan, err := r.train.Plan(context.Background(), []int{1})
	if err != nil {
		t.Fatal(err)
	}
	r.host.closeErr = errors.New("close refused")
	if _, err := r.train.Land(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "close refused") {
		t.Errorf("Land = %v, want the close error", err)
	}
	r.host.commentErr = errors.New("comment refused")
	if _, err := r.train.Land(context.Background(), Plan{Ejected: []*Ejection{{PR: 7, Cause: CauseConflict, Err: errors.New("x")}}}); err == nil || !strings.Contains(err.Error(), "comment refused") {
		t.Errorf("Land = %v, want the comment error", err)
	}
}

func TestTrainLandStopsOnAMergerError(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	plan, err := r.train.Plan(context.Background(), []int{1})
	if err != nil {
		t.Fatal(err)
	}
	r.ci.openErr = errors.New("gh down")
	if _, err := r.train.Land(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "gh down") {
		t.Fatalf("Land = %v, want the CI error", err)
	}
	if len(r.host.closed) != 0 {
		t.Error("nothing landed, so nothing may close")
	}
}

func TestTrainReadyListsThePRsOfTheHost(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.host.ready = []int{4, 2}
	got, err := r.train.ReadyPRs(context.Background())
	if err != nil || !slices.Equal(got, []int{4, 2}) {
		t.Fatalf("ReadyPRs = %v, %v", got, err)
	}
	r.host.readyErr = errors.New("gh down")
	if _, err := r.train.ReadyPRs(context.Background()); err == nil {
		t.Error("ReadyPRs hid a host error")
	}
}

func TestBriefNamesTheCauseAndThePaths(t *testing.T) {
	t.Parallel()
	conflict := &Ejection{PR: 3, Cause: CauseConflict, Paths: []string{"a.go", "b.go"}, Err: errors.New("textual conflict")}
	got := Brief(conflict)
	for _, want := range []string{"PR #3", "conflict", "a.go", "b.go", "Rebase"} {
		if !strings.Contains(got, want) {
			t.Errorf("brief lacks %q:\n%s", want, got)
		}
	}
	if red := Brief(&Ejection{PR: 4, Cause: CauseGate, Err: errors.New("lint")}); !strings.Contains(red, "lint") {
		t.Errorf("gate brief = %q", red)
	}
}

type trainHeadFails struct{ GitMain }

func (trainHeadFails) Head(context.Context) (string, error) { return "", errors.New("origin gone") }

func TestTrainPlanStopsWhenTheHeadOfMainIsUnreadable(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.train.Main = trainHeadFails{}
	if _, err := r.train.Plan(context.Background(), []int{1}); err == nil || !strings.Contains(err.Error(), "origin gone") {
		t.Fatalf("Plan = %v", err)
	}
}

func TestTrainPlanRefusesABadWaveName(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.train.Wave = "bad wave"
	if _, err := r.train.Plan(context.Background(), []int{1}); err == nil {
		t.Fatal("Plan accepted a bad wave name")
	}
}

func TestTrainBatcherFailsLoudlyAtEachStep(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	ctx := context.Background()
	good := &batcher{t: r.train}
	if b, err := good.compose(ctx, []DeliverableRecord{rec}); err != nil || b.Batch.Index != 1 {
		t.Fatalf("compose = %+v, %v", b, err)
	}
	headless := r.train
	headless.Main = trainHeadFails{}
	if _, err := (&batcher{t: headless}).compose(ctx, []DeliverableRecord{rec}); err == nil {
		t.Error("compose ignored an unreadable head")
	}
	badWave := r.train
	badWave.Wave = "bad wave"
	if _, err := (&batcher{t: badWave}).compose(ctx, []DeliverableRecord{rec}); err == nil {
		t.Error("compose ignored a bad wave")
	}
	if _, err := (&batcher{t: r.train, index: 0}).compose(ctx, []DeliverableRecord{rec}); err == nil {
		t.Error("compose must refuse to cut a branch that exists")
	}
	rec.Commits[0].SHA = treeSHA40
	moved, err := (&batcher{t: r.train, index: 5}).compose(ctx, []DeliverableRecord{rec})
	if err != nil || len(moved.Kept) != 0 || len(moved.Ejected) != 1 || moved.Ejected[0].Cause != CauseHead {
		t.Errorf("compose = %+v, %v, want the PR ejected because its head is not the recorded commit", moved, err)
	}
	r.f.g.Git("remote", "set-url", "origin", r.root+"/missing")
	if _, err := (&batcher{t: r.train, index: 9}).compose(ctx, nil); err == nil {
		t.Error("compose ignored an unreachable origin")
	}
}

func TestTrainPlanRefusesAPRWhoseHeadMovedAfterItsRecord(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.f.g.Git("checkout", "-q", "pr-1")
	r.f.g.Write("late.txt", "late\n")
	r.f.g.Git("add", "late.txt")
	r.f.g.Git("commit", "-q", "-m", "fix: late")
	r.f.g.Git("push", "-q", "origin", "pr-1:refs/pull/1/head")
	r.f.g.Git("checkout", "-q", "main")
	_, err := r.train.Plan(context.Background(), []int{1})
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.PR != 1 || !strings.Contains(err.Error(), "record the review again") {
		t.Fatalf("Plan = %v, want a refusal of PR 1 that says to record the review again", err)
	}
}

func TestTrainPlanReportsAHeadItCannotRead(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	real := r.train.Composer.Repo.Git
	r.train.Composer.Repo.Git = func(ctx context.Context, dir, in string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "rev-parse" {
			return "", errors.New("rev-parse failed")
		}
		return real(ctx, dir, in, args...)
	}
	if _, err := r.train.Plan(context.Background(), []int{1}); err == nil || !strings.Contains(err.Error(), "PR #1") {
		t.Fatalf("Plan = %v, want an error naming PR #1", err)
	}
}

func TestTrainPlanSkipsALandedPRAndLandSettlesItOnce(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", map[string]string{"a.txt": "a\n"})
	ctx := context.Background()
	plan, err := r.train.Plan(ctx, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	r.host.closeErr = errors.New("close refused")
	if _, err := r.train.Land(ctx, plan); err == nil {
		t.Fatal("a refused close must fail the land")
	}
	landed := r.originMain()
	if _, ok := FindPRRecord(r.root, 1); !ok {
		t.Fatal("the record must stay while the PR is open")
	}
	r.host.closeErr = nil
	r.train.Wave = "t2"
	again, err := r.train.Plan(ctx, []int{1})
	if err != nil || len(again.Kept) != 0 || len(again.Landed) != 1 || again.Landed[0].PR != 1 {
		t.Fatalf("Plan = %+v, %v, want PR 1 as landed and nothing kept", again, err)
	}
	opened := len(r.ci.opened)
	out, err := r.train.Land(ctx, again)
	if err != nil || len(out.Landed) != 0 || len(r.ci.opened) != opened || r.originMain() != landed {
		t.Fatalf("Land = %+v, %v opened %d: a landed PR must not land again", out, err, len(r.ci.opened))
	}
	if body := r.host.closed[1]; body != alreadyLanded {
		t.Errorf("close comment = %q, want %q", body, alreadyLanded)
	}
	if _, ok := FindPRRecord(r.root, 1); ok {
		t.Error("the record of a landed PR must be spent")
	}
	if !slices.Contains(r.inbox, "item-a") {
		t.Errorf("inbox ids = %v, want item-a closed", r.inbox)
	}
}

func TestTrainLandClosesTheInboxBeforeThePR(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a, item-b", map[string]string{"a.txt": "a\n"})
	r.pr(t, 2, "feat: b", map[string]string{"b.txt": "b\n"})
	plan, err := r.train.Plan(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	r.train.Inbox = func(_ context.Context, ids []string) ([]string, error) {
		order = append(order, "inbox:"+strings.Join(ids, ","))
		return ids, nil
	}
	if _, err := r.train.Land(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"inbox:item-a,item-b"}) {
		t.Errorf("inbox calls = %v, want one call with both ids and none for PR 2", order)
	}
}

func TestTrainLandKeepsTheRecordWhenTheInboxFails(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", map[string]string{"a.txt": "a\n"})
	plan, err := r.train.Plan(context.Background(), []int{1})
	if err != nil {
		t.Fatal(err)
	}
	r.train.Inbox = func(context.Context, []string) ([]string, error) { return nil, errors.New("inbox down") }
	if _, err := r.train.Land(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "inbox down") {
		t.Fatalf("Land = %v, want the inbox error", err)
	}
	if _, closed := r.host.closed[1]; closed {
		t.Error("the PR must stay open until its inbox items close")
	}
	if _, ok := FindPRRecord(r.root, 1); !ok {
		t.Error("the record must stay for the rerun")
	}
}

func TestTrainLandRefusesInboxIDsItCannotClose(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := r.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", map[string]string{"a.txt": "a\n"})
	r.train.Inbox = nil
	if _, err := r.train.closeInbox(context.Background(), rec); err == nil || !strings.Contains(err.Error(), "item-a") {
		t.Errorf("closeInbox = %v, want a refusal that names item-a", err)
	}
	rec.Commits[0].SHA = treeSHA40
	if _, err := r.train.closeInbox(context.Background(), rec); err == nil {
		t.Error("closeInbox hid an unreadable commit")
	}
}

func TestTrainCompleteReportsARecordThatIsAlreadyGone(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := r.pr(t, 3, "feat: a", map[string]string{"a.txt": "a\n"})
	if err := os.Remove(PRRecordPath(WaveDir(r.root, ConsoleWave), 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.train.complete(context.Background(), rec, func() error { return nil }); err == nil || !strings.Contains(err.Error(), "spend") {
		t.Errorf("complete = %v, want a spend error", err)
	}
}

func TestTrainEjectBriefsAComposeEjectionWithItsPaths(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := consoleRecord()
	conflict := &Ejection{PR: rec.PR, Cause: CauseConflict, Paths: []string{"x.go"}, Err: errors.New("textual conflict")}
	if err := r.train.eject(context.Background(), rec, conflict); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(r.host.comments[rec.PR], "\n")
	if !strings.Contains(got, "x.go") || !strings.Contains(got, CauseConflict) {
		t.Errorf("brief = %q, want the conflict and its path", got)
	}
}

func TestTrainEjectBriefsAComposeErrorOfTheBatch(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := consoleRecord()
	if err := r.train.eject(context.Background(), rec, ErrCIRed); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.host.comments[rec.PR], "\n"); !strings.Contains(got, CauseCI) {
		t.Errorf("brief = %q, want the ci cause", got)
	}
}
