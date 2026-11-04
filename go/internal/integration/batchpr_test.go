package integration

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
)

type fakeCI struct {
	mu       sync.Mutex
	red      func(b Batch, run int) bool
	opened   []Batch
	runs     map[string]int
	reruns   int
	openErr  error
	watchErr error
	rerunErr error
}

func (f *fakeCI) Open(_ context.Context, b Batch) (PR, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.openErr != nil {
		return PR{}, f.openErr
	}
	f.opened = append(f.opened, b)
	return PR{Number: len(f.opened), Tip: b.Tip}, nil
}

func (f *fakeCI) Watch(_ context.Context, pr PR) (Verdict, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.watchErr != nil {
		return "", f.watchErr
	}
	if f.runs == nil {
		f.runs = map[string]int{}
	}
	f.runs[pr.Tip]++
	if f.red != nil && f.red(f.opened[pr.Number-1], f.runs[pr.Tip]) {
		return Red, nil
	}
	return Green, nil
}

func (f *fakeCI) Rerun(context.Context, PR) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reruns++
	return f.rerunErr
}

type fakeMain struct {
	mu     sync.Mutex
	head   string
	moves  []string
	landed []string
	ffErr  error
}

func (m *fakeMain) Head(context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.moves) > 0 {
		m.head, m.moves = m.moves[0], m.moves[1:]
	}
	return m.head, nil
}

func (m *fakeMain) FastForward(_ context.Context, tip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ffErr != nil {
		return m.ffErr
	}
	m.landed = append(m.landed, tip)
	m.head = tip
	return nil
}

func sha(c byte) string {
	b := make([]byte, 40)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

type landRig struct {
	ci      *fakeCI
	main    *fakeMain
	ejected []int
	merger  Merger
	calls   [][]int
}

func newLandRig(headSHA string) *landRig {
	r := &landRig{ci: &fakeCI{}, main: &fakeMain{head: headSHA}}
	r.merger = Merger{
		CI:   r.ci,
		Main: r.main,
		Compose: func(_ context.Context, recs []DeliverableRecord) (Composition, error) {
			r.calls = append(r.calls, cyclesOf(recs))
			head, _ := r.main.Head(context.Background())
			tip := sha(byte('a' + len(r.calls)))
			return Composition{Batch: Batch{Wave: "1", Index: len(r.calls), Base: head, Branch: "integration/w1-b" + string(rune('0'+len(r.calls))), Tip: tip}, Kept: recs}, nil
		},
		Eject: func(_ context.Context, rec DeliverableRecord, _ error) error {
			r.ejected = append(r.ejected, rec.Cycle)
			return nil
		},
	}
	return r
}

func cyclesOf(recs []DeliverableRecord) []int {
	out := make([]int, len(recs))
	for i, r := range recs {
		out[i] = r.Cycle
	}
	return out
}

func recs(cycles ...int) []DeliverableRecord {
	out := make([]DeliverableRecord, len(cycles))
	for i, c := range cycles {
		out[i] = DeliverableRecord{Cycle: c}
	}
	return out
}

func TestMergerLandsAGreenBatchByFastForward(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	out, err := r.merger.Land(context.Background(), recs(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out.Landed, []int{1, 2}) || len(r.main.landed) != 1 || r.ci.reruns != 0 {
		t.Errorf("landed %v ff %v reruns %d, want [1 2], one ff and no rerun", out.Landed, r.main.landed, r.ci.reruns)
	}
}

func TestMergerRerunsARedOnceAndTreatsAPassingRerunAsAFlake(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	r.ci.red = func(_ Batch, run int) bool { return run == 1 }
	out, err := r.merger.Land(context.Background(), recs(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if r.ci.reruns != 1 || !out.Flaked || out.ConfirmedRed || len(r.ejected) != 0 || !slices.Equal(out.Landed, []int{1, 2}) {
		t.Errorf("reruns %d out %+v ejected %v, want one rerun, a flake and no ejection", r.ci.reruns, out, r.ejected)
	}
}

func TestMergerSplitsAfterASecondRedAndEjectsTheCulprit(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	culprit := 3
	badTips := map[string]bool{}
	r.merger.Compose = func(_ context.Context, rs []DeliverableRecord) (Composition, error) {
		r.calls = append(r.calls, cyclesOf(rs))
		head, _ := r.main.Head(context.Background())
		tip := sha(byte('a' + len(r.calls)))
		b := Batch{Wave: "1", Index: len(r.calls), Base: head, Branch: "integration/w1-b1", Tip: tip}
		if slices.Contains(cyclesOf(rs), culprit) {
			badTips[tip] = true
		}
		return Composition{Batch: b, Kept: rs}, nil
	}
	r.ci.red = func(b Batch, _ int) bool { return badTips[b.Tip] }
	out, err := r.merger.Land(context.Background(), recs(1, 2, 3, 4))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.ejected, []int{3}) || !out.ConfirmedRed || !slices.Equal(out.Landed, []int{1, 2, 4}) {
		t.Errorf("ejected %v out %+v, want only cycle 3 ejected and 1 2 4 landed", r.ejected, out)
	}
	for _, c := range r.calls[1:] {
		if len(c) > 2 {
			t.Errorf("a split part has %d deliverables %v, want at most 2", len(c), c)
		}
	}
}

func TestMergerComposesAndTestsAgainWhenMainMovedAfterGreen(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	r.main.moves = []string{sha('0'), sha('9')}
	out, err := r.merger.Land(context.Background(), recs(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 2 || len(r.ci.opened) != 2 || len(r.main.landed) != 1 || r.main.landed[0] != r.ci.opened[1].Tip || !slices.Equal(out.Landed, []int{1}) {
		t.Errorf("composes %d prs %d ff %v, want a second compose, a second CI run and one ff of the second tip", len(r.calls), len(r.ci.opened), r.main.landed)
	}
}

func TestMergerGivesUpWhenMainKeepsMoving(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	n := 0
	r.merger.Main = movingMain{fakeMain: r.main, next: func() string { n++; return sha(byte('0' + n)) }}
	_, err := r.merger.Land(context.Background(), recs(1))
	if err == nil || len(r.main.landed) != 0 {
		t.Errorf("err %v landed %v, want an error and no fast-forward", err, r.main.landed)
	}
}

type movingMain struct {
	*fakeMain
	next func() string
}

func (m movingMain) Head(context.Context) (string, error) { return m.next(), nil }

func TestMergerPropagatesPortErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	cases := map[string]func(r *landRig){
		"open":  func(r *landRig) { r.ci.openErr = boom },
		"watch": func(r *landRig) { r.ci.watchErr = boom },
		"rerun": func(r *landRig) {
			r.ci.red = func(Batch, int) bool { return true }
			r.ci.rerunErr = boom
		},
		"ff": func(r *landRig) { r.main.ffErr = boom },
		"compose": func(r *landRig) {
			r.merger.Compose = func(context.Context, []DeliverableRecord) (Composition, error) { return Composition{}, boom }
		},
		"eject": func(r *landRig) {
			r.ci.red = func(Batch, int) bool { return true }
			r.merger.Eject = func(context.Context, DeliverableRecord, error) error { return boom }
		},
		"head": func(r *landRig) { r.merger.Main = headFails{fakeMain: r.main, err: boom} },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newLandRig(sha('0'))
			setup(r)
			if _, err := r.merger.Land(context.Background(), recs(1)); !errors.Is(err, boom) {
				t.Errorf("err %v, want boom", err)
			}
		})
	}
}

type headFails struct {
	*fakeMain
	err error
}

func (h headFails) Head(context.Context) (string, error) { return "", h.err }

func TestMergerRefusesAnEmptyBatch(t *testing.T) {
	t.Parallel()
	if _, err := newLandRig(sha('0')).merger.Land(context.Background(), nil); err == nil {
		t.Error("Land of no deliverable returned nil, want an error")
	}
}

func TestMergerStopsSplittingWhenTheFirstPartFails(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	boom := errors.New("boom")
	r.ci.red = func(b Batch, _ int) bool { return b.Index == 1 }
	compose := r.merger.Compose
	r.merger.Compose = func(ctx context.Context, rs []DeliverableRecord) (Composition, error) {
		if len(r.calls) == 1 {
			r.calls = append(r.calls, cyclesOf(rs))
			return Composition{}, boom
		}
		return compose(ctx, rs)
	}
	_, err := r.merger.Land(context.Background(), recs(1, 2, 3, 4))
	if !errors.Is(err, boom) || len(r.main.landed) != 0 || len(r.calls) != 2 {
		t.Errorf("err %v landed %v composes %d, want the first part's error and no second part", err, r.main.landed, len(r.calls))
	}
}

func TestMergerRoutesAnEjectionOfTheComposeToEjectAndLandsTheRest(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	compose := r.merger.Compose
	var causes []error
	r.merger.Compose = func(ctx context.Context, rs []DeliverableRecord) (Composition, error) {
		comp, _ := compose(ctx, rs)
		comp.Kept = rs[:1]
		comp.Ejected = []*Ejection{{Cycle: rs[1].Cycle, Cause: CauseConflict, Paths: []string{"x.go"}}}
		return comp, nil
	}
	r.merger.Eject = func(_ context.Context, rec DeliverableRecord, cause error) error {
		r.ejected = append(r.ejected, rec.Cycle)
		causes = append(causes, cause)
		return nil
	}
	out, err := r.merger.Land(context.Background(), recs(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	var ej *Ejection
	if !slices.Equal(out.Landed, []int{1}) || !slices.Equal(out.Ejected, []int{2}) || !slices.Equal(r.ejected, []int{2}) || len(causes) != 1 || !errors.As(causes[0], &ej) || ej.Cause != CauseConflict {
		t.Errorf("landed %v ejected %v routed %v causes %v, want 1 landed and the conflict of 2 routed to Eject", out.Landed, out.Ejected, r.ejected, causes)
	}
	if len(r.ci.opened) != 1 || len(r.main.landed) != 1 {
		t.Errorf("opened %d ff %d, want one batch for the kept PR", len(r.ci.opened), len(r.main.landed))
	}
}

func TestMergerOpensNoBatchWhenEveryDeliverableIsEjected(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	r.merger.Compose = func(_ context.Context, rs []DeliverableRecord) (Composition, error) {
		return Composition{Ejected: []*Ejection{{Cycle: rs[0].Cycle, Cause: CauseGate}}}, nil
	}
	out, err := r.merger.Land(context.Background(), recs(4))
	if err != nil || len(out.Landed) != 0 || !slices.Equal(r.ejected, []int{4}) || len(r.ci.opened) != 0 {
		t.Errorf("out %+v err %v ejected %v opened %d, want one ejection and no batch", out, err, r.ejected, len(r.ci.opened))
	}
}

func TestMergerReportsAFailedEjectionBrief(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	boom := errors.New("brief refused")
	r.merger.Compose = func(_ context.Context, rs []DeliverableRecord) (Composition, error) {
		return Composition{Ejected: []*Ejection{{Cycle: rs[0].Cycle}}}, nil
	}
	r.merger.Eject = func(context.Context, DeliverableRecord, error) error { return boom }
	if _, err := r.merger.Land(context.Background(), recs(4)); !errors.Is(err, boom) {
		t.Errorf("err %v, want the brief error", err)
	}
}

func TestEjectionRefIsThePROrTheCycle(t *testing.T) {
	t.Parallel()
	if (&Ejection{PR: 7}).Ref() != 7 || (&Ejection{Cycle: 9}).Ref() != 9 {
		t.Error("Ejection.Ref must name the PR of a console record and the cycle of a lane record")
	}
}
