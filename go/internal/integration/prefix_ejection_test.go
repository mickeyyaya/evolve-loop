package integration

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func threeDeliverables(t *testing.T, f composeFixture) []DeliverableRecord {
	t.Helper()
	return append(twoDeliverables(t, f),
		scoped(f.lane(t, 703, map[string]string{goFile("c"): "package gamma\n"}), "wave-integration-i3-gamma"))
}

func gammaBreaksATest(_, pkg string, _ int) error {
	if strings.Contains(pkg, "gamma") {
		return errors.New("--- FAIL: TestGamma")
	}
	return nil
}

func TestBatchPrefix_ThreeDeliverablesWhereTheLastBreaksATestFailTheCombinedPrefix(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.testFail = gammaBreaksATest

	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), threeDeliverables(t, f))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if !slices.Contains(s.testRuns, "A+B+C ./internal/gamma/...") {
		t.Errorf("tests ran %v, want the gamma package tested on the prefix A+B+C", s.testRuns)
	}
	if v := got.Verdicts[703]; v.Pass || !slices.Contains(v.Red, "./internal/gamma/...") {
		t.Errorf("verdict of 703 = %+v, want red on the gamma package", v)
	}
	if !got.Verdicts[701].Pass || !got.Verdicts[702].Pass {
		t.Errorf("verdicts of 701 and 702 = %+v %+v, want both green on their own prefixes", got.Verdicts[701], got.Verdicts[702])
	}
	if !slices.Equal(got.Landed, []int{701, 702}) || len(got.Ejected) != 1 || got.Ejected[0].Cycle != 703 {
		t.Errorf("Check = landed %v ejected %+v, want 701 and 702 landed and only 703 ejected", got.Landed, got.Ejected)
	}
	if !f.holdsFile(got.Tip, goFile("a")) || !f.holdsFile(got.Tip, goFile("b")) || f.holdsFile(got.Tip, goFile("c")) {
		t.Errorf("the landed tip %s must hold A and B and not C", got.Tip)
	}
}

func TestBatchPrefix_TheEjectionOfTheThirdHoldsTheDiffOfTheFirstTwoInItsBrief(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.testFail = gammaBreaksATest

	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), threeDeliverables(t, f))
	if err != nil || len(got.Ejected) != 1 {
		t.Fatalf("Check = %+v, %v", got, err)
	}

	ej := got.Ejected[0]
	if ej.Cause != CauseBatch || !slices.Contains(ej.Paths, "./internal/gamma/...") {
		t.Errorf("ejection = %+v, want the batch cause with the red gamma package", ej)
	}
	if !strings.Contains(ej.Diff, goFile("a")) || !strings.Contains(ej.Diff, goFile("b")) || strings.Contains(ej.Diff, goFile("c")) {
		t.Errorf("diff = %q, want the change of A and B that C met and not the change of C", ej.Diff)
	}
	brief := ej.Brief()
	fenced := brief[strings.Index(brief, BriefDataBegin):strings.Index(brief, BriefDataEnd)]
	if !strings.Contains(fenced, goFile("a")) || !strings.Contains(fenced, goFile("b")) || !strings.Contains(brief, "Cause: "+CauseBatch) {
		t.Errorf("brief = %q, want the diff of A+B between the data fences and the cause", brief)
	}
}

func TestBatchPrefix_ATextualConflictOfTheThirdEjectsBeforeAnyTestOrGateRuns(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := []DeliverableRecord{
		f.lane(t, 701, map[string]string{goFile("a"): "package alpha\n", sharedFile: "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"}),
		f.lane(t, 702, map[string]string{goFile("b"): "package beta\n"}),
		f.lane(t, 703, map[string]string{goFile("c"): "package gamma\n", sharedFile: "uno\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"}),
	}
	s := newSeams(f)
	bt := s.batchTest(f)
	gate := &gateLog{}
	bt.Composer.Gate = gate.gate

	got, err := bt.Check(context.Background(), f.batch(t), recs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(got.Ejected) != 1 || got.Ejected[0].Cycle != 703 || got.Ejected[0].Cause != CauseConflict || !slices.Equal(got.Ejected[0].Paths, []string{sharedFile}) {
		t.Fatalf("ejected = %+v, want only 703 on a conflict over %s", got.Ejected, sharedFile)
	}
	if _, ran := got.Verdicts[703]; ran {
		t.Errorf("a conflicted deliverable got the verdict %+v, want no test layer at all", got.Verdicts[703])
	}
	for _, run := range slices.Concat(s.testRuns, s.predRuns) {
		if label := strings.Fields(run)[0]; strings.Contains(label, "C") {
			t.Errorf("run %q touched a tree that holds C, want none: C never composed", run)
		}
	}
	if gate.calls() != 0 {
		t.Errorf("the gate ran %d times, want 0 on a textual conflict", gate.calls())
	}
	if d := got.Ejected[0].Diff; !strings.Contains(d, goFile("a")) || !strings.Contains(d, goFile("b")) || strings.Contains(d, goFile("c")) {
		t.Errorf("diff = %q, want the change of A+B that C met", d)
	}
	if !slices.Equal(got.Landed, []int{701, 702}) {
		t.Errorf("landed = %v, want [701 702]", got.Landed)
	}
}

type briefDir struct{ dir string }

func (b briefDir) Write(_ context.Context, ej *Ejection) (string, error) {
	return ej.WriteBrief(b.dir)
}

func TestEnforceActor_AnEjectionPublishesTheEjectedEventWithTheBriefFileOfItsLane(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ports := &stubEnforcePorts{}
	out := &sink{}
	a := EnforceActor{
		Merger: Merger{
			CI:   stubCI{verdict: Red},
			Main: &stubMain{head: testSHA},
			Compose: func(_ context.Context, recs []DeliverableRecord) (Composition, error) {
				return Composition{Batch: Batch{Base: testSHA, Tip: testSHA}, Kept: recs}, nil
			},
		},
		Briefs:  briefDir{dir: dir},
		Requeue: ports,
		Publish: out,
		Now:     func() time.Time { return time.Unix(100, 0).UTC() },
	}

	res, err := a.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: testSHA, Members: []DeliverableRecord{{Cycle: 9}}})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if res.Members[9].Status != MemberEjected {
		t.Fatalf("member 9 = %+v, want ejected", res.Members[9])
	}
	out.mu.Lock()
	defer out.mu.Unlock()
	var ejected []string
	for _, e := range out.events {
		if e.Code != CodeEjected {
			continue
		}
		ejected = append(ejected, e.Fields[FieldBriefFile])
		if e.Cycle != 9 || e.Fields[FieldCause] != CauseGate || e.Module == "" {
			t.Errorf("event = %+v, want cycle 9, cause %s and its module", e, CauseGate)
		}
	}
	if want := BriefPath(dir, 9); !slices.Equal(ejected, []string{want}) {
		t.Fatalf("ejected events carry the brief files %v, want exactly [%s]", ejected, want)
	}
	if text, err := ReadBrief(BriefPath(dir, 9)); err != nil || !strings.Contains(text, "Cause: "+CauseGate) {
		t.Errorf("brief = %q, %v, want the cause in the file that the event names", text, err)
	}
}
