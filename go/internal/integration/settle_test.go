package integration

import (
	"context"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestSettleExportsAreNamedByAnExecutedAssertion(t *testing.T) {
	t.Parallel()
	var gh GhFunc = func(_ context.Context, _ string, args ...string) (string, int, error) {
		switch args[1] {
		case "view":
			return `{"state":"MERGED"}`, 0, nil
		}
		return `[{"number":7,"headRefName":"integration/w-b1"},{"number":8,"headRefName":"feat/x"}]`, 0, nil
	}
	h := GhHost{CI: GhCI{Gh: gh}}
	state, err := h.State(context.Background(), 3)
	if err != nil || state != PRState("MERGED") || state == stateOpen {
		t.Errorf("State = %q, %v, want MERGED", state, err)
	}
	batches, err := h.OpenBatches(context.Background())
	if err != nil || !slices.Equal(batches, []int{7}) {
		t.Errorf("OpenBatches = %v, %v, want only the batch PR 7", batches, err)
	}
	var host PRHost = &fakeHost{}
	if s, err := host.State(context.Background(), 1); err != nil || s != stateOpen {
		t.Errorf("PRHost.State = %q, %v", s, err)
	}
	if b, err := host.OpenBatches(context.Background()); err != nil || len(b) != 0 {
		t.Errorf("PRHost.OpenBatches = %v, %v", b, err)
	}
}

func TestSettlementLineNamesThePRAndWhatSettled(t *testing.T) {
	t.Parallel()
	record := Settlement{PR: 5, kind: settleClosed}.Line()
	if !strings.Contains(record, "PR #5") || !strings.Contains(record, "spent") || strings.HasPrefix(record, "step:") {
		t.Errorf("record line = %q", record)
	}
	batch := Settlement{PR: 60, Moved: []string{"i"}, kind: settleBatchPR}.Line()
	if !strings.Contains(batch, "batch PR #60") || !strings.Contains(batch, "closed") {
		t.Errorf("batch line = %q", batch)
	}
}

func TestSourcePRsReadsEachSourceTrailerAndSkipsOtherLines(t *testing.T) {
	t.Parallel()
	got := sourcePRs("evolve-pr 4: integrate\n\nSource-PR: #4\nSource-PR: #x\nSource-PR: #0\n Source-PR: #9 \nCloses-Inbox: i")
	if !slices.Equal(got, []int{4, 9}) {
		t.Errorf("sourcePRs = %v, want [4 9]", got)
	}
	if got := sourcePRs("no trailer"); len(got) != 0 {
		t.Errorf("sourcePRs of a message with no trailer = %v", got)
	}
}

func (r *trainRig) landLeavingRecords(t *testing.T, prs ...int) {
	t.Helper()
	ctx := context.Background()
	plan, err := r.train.Plan(ctx, prs)
	if err != nil {
		t.Fatal(err)
	}
	r.host.closeErr = errors.New("close refused")
	if _, err := r.train.Land(ctx, plan); err == nil {
		t.Fatal("a refused close must fail the land")
	}
	r.host.closeErr = nil
	r.host.closed = nil
	r.train.Wave = "t2"
}

func (r *trainRig) batchPR(t *testing.T, n int, messages ...string) {
	t.Helper()
	g := r.f.g
	g.Git("checkout", "-q", "-b", "batch-"+strconv.Itoa(n), r.originMain())
	for i, m := range messages {
		file := "batch-" + strconv.Itoa(n) + "-" + strconv.Itoa(i) + ".txt"
		g.Write(file, m+"\n")
		g.Git("add", file)
		g.Git("commit", "-q", "-m", m)
	}
	g.Git("push", "-q", "origin", "+HEAD:refs/pull/"+strconv.Itoa(n)+"/head")
	g.Git("checkout", "-q", "main")
}

func memberMessage(pr int) string {
	return "evolve-pr " + strconv.Itoa(pr) + ": integrate\n\nSource-PR: #" + strconv.Itoa(pr)
}

func settledPRs(settled []Settlement) []int {
	var prs []int
	for _, s := range settled {
		prs = append(prs, s.PR)
	}
	return prs
}

func TestSettleSpendsTheRecordOfAClosedPRKeepsAnOpenOneAndSkipsLaneRecords(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.pr(t, 2, "feat: b", map[string]string{"b.txt": "b\n"})
	lane := r.f.lane(t, 7, map[string]string{"l.txt": "l\n"})
	if _, err := WriteRecord(WaveDir(r.root, "t1"), lane); err != nil {
		t.Fatal(err)
	}
	r.host.states = map[int]PRState{1: "CLOSED"}
	settled, err := r.train.Settle(context.Background())
	if err != nil || !slices.Equal(settledPRs(settled), []int{1}) {
		t.Fatalf("Settle = %v, %v, want only PR 1", settled, err)
	}
	if _, ok := FindPRRecord(r.root, 1); ok {
		t.Error("the record of a closed PR must be spent")
	}
	if _, ok := FindPRRecord(r.root, 2); !ok {
		t.Error("the record of an open PR that is not on main must stay")
	}
	if _, ok := ListRecords(r.root).ByCommit(lane.Commits[0].SHA); !ok {
		t.Error("a lane record is not a console record and must stay")
	}
	if len(r.host.closed) != 0 {
		t.Errorf("a PR the host closed already needs no close: %v", r.host.closed)
	}
}

func TestSettleClosesALandedPRMovesItsInboxAndSpendsItsRecord(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", map[string]string{"a.txt": "a\n"})
	r.landLeavingRecords(t, 1)
	settled, err := r.train.Settle(context.Background())
	if err != nil || len(settled) != 1 || !slices.Equal(settled[0].Moved, []string{"item-a"}) {
		t.Fatalf("Settle = %+v, %v, want PR 1 with item-a moved", settled, err)
	}
	if body := r.host.closed[1]; body != alreadyLanded {
		t.Errorf("close comment = %q, want %q", body, alreadyLanded)
	}
	if _, ok := FindPRRecord(r.root, 1); ok {
		t.Error("the record of a landed PR must be spent")
	}
	if line := settled[0].Line(); !strings.Contains(line, "PR #1") || !strings.Contains(line, "spent") {
		t.Errorf("line = %q", line)
	}
}

func TestSettleKeepsAnOpenPRWhoseHeadMovedSinceTheRecord(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.batchPR(t, 1, "feat: a moved")
	settled, err := r.train.Settle(context.Background())
	if err != nil || len(settled) != 0 {
		t.Fatalf("Settle = %+v, %v, want nothing settled", settled, err)
	}
	if _, ok := FindPRRecord(r.root, 1); !ok {
		t.Error("the record must stay while the head differs from the recorded head")
	}
}

func TestSettleReportsEachFailureAndKeepsTheRecordsItCouldNotSettle(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.pr(t, 2, "feat: b", map[string]string{"b.txt": "b\n"})
	r.host.stateErrs = map[int]error{1: errors.New("view refused")}
	r.origin.Git("update-ref", "-d", "refs/pull/2/head")
	_, err := r.train.Settle(context.Background())
	if err == nil || !strings.Contains(err.Error(), "view refused") || !strings.Contains(err.Error(), "PR #2") {
		t.Fatalf("Settle = %v, want both failures", err)
	}
	for _, pr := range []int{1, 2} {
		if _, ok := FindPRRecord(r.root, pr); !ok {
			t.Errorf("the record of PR %d must stay after a failure", pr)
		}
	}
}

func TestSettleKeepsTheRecordWhenTheLandedCloseIsRefused(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.landLeavingRecords(t, 1)
	r.host.closeErr = errors.New("close refused")
	settled, err := r.train.Settle(context.Background())
	if err == nil || len(settled) != 0 {
		t.Fatalf("Settle = %+v, %v, want the refusal", settled, err)
	}
	if _, ok := FindPRRecord(r.root, 1); !ok {
		t.Error("the record must stay when the close is refused")
	}
}

func TestSettleFailsWhenTheRecordOfAClosedPRCannotBeSpent(t *testing.T) {
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.host.states = map[int]PRState{1: "CLOSED"}
	dir := WaveDir(r.root, ConsoleWave)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if _, err := r.train.Settle(context.Background()); err == nil || !strings.Contains(err.Error(), "spend the record of PR #1") {
		t.Fatalf("Settle = %v, want a spend failure", err)
	}
}

func TestSettleClosesABatchPROnlyWhenEveryMemberIsLandedOrClosed(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.batchPR(t, 60, memberMessage(1), memberMessage(2))
	r.batchPR(t, 61, memberMessage(3))
	r.batchPR(t, 62, "feat: no source trailer")
	r.batchPR(t, 63)
	r.host.batches = []int{60, 61, 62, 63}
	r.host.states = map[int]PRState{1: "CLOSED", 2: "MERGED", 3: stateOpen}
	settled, err := r.train.Settle(context.Background())
	if err != nil || !slices.Equal(settledPRs(settled), []int{1, 60}) {
		t.Fatalf("Settle = %v, %v, want PR 1 and batch PR 60", settled, err)
	}
	if body := r.host.closed[60]; body != batchSettled {
		t.Errorf("batch close comment = %q", body)
	}
	for _, pr := range []int{61, 62, 63} {
		if _, closed := r.host.closed[pr]; closed {
			t.Errorf("batch PR %d must stay open", pr)
		}
	}
	if line := settled[1].Line(); !strings.Contains(line, "batch PR #60") {
		t.Errorf("line = %q", line)
	}
}

func TestSettleReportsABatchPRThatCannotBeRead(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.batchPR(t, 60, memberMessage(1))
	r.batchPR(t, 61, memberMessage(2))
	r.batchPR(t, 62, memberMessage(3))
	r.origin.Git("update-ref", "-d", "refs/pull/60/head")
	r.host.batches = []int{60, 61, 62}
	r.host.stateErrs = map[int]error{2: errors.New("view refused")}
	r.host.states = map[int]PRState{3: "CLOSED"}
	r.host.closeErr = errors.New("close refused")
	settled, err := r.train.Settle(context.Background())
	if len(settled) != 0 || err == nil {
		t.Fatalf("Settle = %+v, %v, want only failures", settled, err)
	}
	for _, want := range []string{"PR #60", "view refused", "close refused"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}

func TestSettleFailsWhenTheCommitsOfABatchPRCannotBeListed(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.batchPR(t, 60, memberMessage(1))
	r.host.batches = []int{60}
	real := r.train.Composer.Repo.Git
	r.train.Composer.Repo.Git = func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "log" {
			return "", errors.New("log refused")
		}
		return real(ctx, dir, stdin, args...)
	}
	if _, err := r.train.Settle(context.Background()); err == nil || !strings.Contains(err.Error(), "batch PR #60") {
		t.Fatalf("Settle = %v, want an error naming batch PR #60", err)
	}
}

func TestSettleFailsWhenTheOpenBatchPRsCannotBeListed(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.host.batchesErr = errors.New("list refused")
	if _, err := r.train.Settle(context.Background()); err == nil || !strings.Contains(err.Error(), "list refused") {
		t.Fatalf("Settle = %v, want the list failure", err)
	}
}

func TestGhHostSettleReadsRefuseTheirFailureModes(t *testing.T) {
	t.Parallel()
	garbage := GhHost{CI: GhCI{Gh: func(context.Context, string, ...string) (string, int, error) { return "not json", 0, nil }}}
	if _, err := garbage.State(context.Background(), 1); err == nil {
		t.Error("State accepted output that is not JSON")
	}
	if _, err := garbage.OpenBatches(context.Background()); err == nil {
		t.Error("OpenBatches accepted output that is not JSON")
	}
	full := "[" + strings.Repeat(`{"number":1,"headRefName":"integration/x"},`, readyLimit-1) + `{"number":2,"headRefName":"x"}]`
	capped := GhHost{CI: GhCI{Gh: func(context.Context, string, ...string) (string, int, error) { return full, 0, nil }}}
	if _, err := capped.OpenBatches(context.Background()); err == nil || !strings.Contains(err.Error(), "most it lists") {
		t.Errorf("OpenBatches = %v, want the listing limit error", err)
	}
}
