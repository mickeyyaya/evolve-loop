package codeindextest_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
)

type recorder struct{ problems []string }

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.problems = append(r.problems, fmt.Sprintf(format, args...))
}

func (r *recorder) all() string { return strings.Join(r.problems, "\n") }

type scripted struct {
	caps   codeindex.Capabilities
	answer func(codeindex.Request) ([]codeindex.Hit, error)
}

func (s scripted) Name() string                         { return "scripted" }
func (s scripted) Capabilities() codeindex.Capabilities { return s.caps }

func (s scripted) Callers(_ context.Context, symbol string, dir codeindex.Direction) ([]codeindex.Hit, error) {
	return s.answer(codeindex.Request{Query: codeindex.QueryCallers, Arg: symbol, Direction: dir})
}

func (s scripted) API(_ context.Context, file string) ([]codeindex.Hit, error) {
	return s.answer(codeindex.Request{Query: codeindex.QueryAPI, Arg: file})
}

func (s scripted) Find(_ context.Context, query string) ([]codeindex.Hit, error) {
	return s.answer(codeindex.Request{Query: codeindex.QueryFind, Arg: query})
}

func (s scripted) Map(_ context.Context, dir string) ([]codeindex.Hit, error) {
	return s.answer(codeindex.Request{Query: codeindex.QueryMap, Arg: dir})
}

func fakeHarness(caps codeindex.Capabilities) codeindextest.Harness {
	var probes []codeindextest.Probe = codeindextest.StandardProbes()
	return codeindextest.Harness{
		New: func() codeindex.Backend {
			var fake codeindextest.Fake = codeindextest.NewFake("fake", caps, codeindextest.StandardIndex())
			return fake
		},
		NewAbsent: func() codeindex.Backend { return codeindextest.NewAbsentFake("fake", caps) },
		Probes:    probes,
	}
}

func scriptedHarness(caps codeindex.Capabilities, answer func(codeindex.Request) ([]codeindex.Hit, error)) codeindextest.Harness {
	h := fakeHarness(caps)
	h.New = func() codeindex.Backend { return scripted{caps: caps, answer: answer} }
	return h
}

func TestRun_AFullFakePassesTheSuite(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	var reporter codeindextest.Reporter = rec

	codeindextest.Run(reporter, fakeHarness(codeindex.AllCapabilities()))

	if len(rec.problems) != 0 {
		t.Errorf("problems = %s", rec.all())
	}
}

func TestRun_APartialFakePassesBecauseItAnswersUnsupportedForWhatItDoesNotServe(t *testing.T) {
	t.Parallel()
	rec := &recorder{}

	codeindextest.Run(rec, fakeHarness(codeindex.Capabilities{Find: true}))

	if len(rec.problems) != 0 {
		t.Errorf("problems = %s", rec.all())
	}
}

func TestRun_ABackendThatAnswersEverythingWithAGoodHitButNeverFailsIsCaughtOnNotFound(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	good := codeindextest.NewFake("good", codeindex.AllCapabilities(), codeindextest.StandardIndex())
	answer := func(req codeindex.Request) ([]codeindex.Hit, error) {
		probe := codeindextest.StandardProbes()
		for _, p := range probe {
			if p.Missing == req {
				return []codeindex.Hit{{File: "x.go"}}, nil
			}
		}
		return codeindex.Dispatch(context.Background(), good, req)
	}

	codeindextest.Run(rec, scriptedHarness(codeindex.AllCapabilities(), answer))

	if !strings.Contains(rec.all(), "no answer must be ErrNotFound") {
		t.Errorf("problems = %s", rec.all())
	}
}

func TestRun_ABackendThatErrorsOnAKnownQueryIsReported(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	answer := func(codeindex.Request) ([]codeindex.Hit, error) { return nil, codeindex.ErrAbsent }

	codeindextest.Run(rec, scriptedHarness(codeindex.AllCapabilities(), answer))

	if !strings.Contains(rec.all(), "known query failed") {
		t.Errorf("problems = %s", rec.all())
	}
}

func TestRun_ABackendThatNeverReturnsTheExpectedHitIsReported(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	answer := func(codeindex.Request) ([]codeindex.Hit, error) {
		return []codeindex.Hit{{File: "other.go", Symbol: "other"}}, nil
	}

	codeindextest.Run(rec, scriptedHarness(codeindex.AllCapabilities(), answer))

	if !strings.Contains(rec.all(), "no hit matches") {
		t.Errorf("problems = %s", rec.all())
	}
}

func TestRun_AHitWithoutAFileIsReported(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	answer := func(codeindex.Request) ([]codeindex.Hit, error) {
		return []codeindex.Hit{{Symbol: "x"}}, nil
	}

	codeindextest.Run(rec, scriptedHarness(codeindex.AllCapabilities(), answer))

	if !strings.Contains(rec.all(), "hit without a file") {
		t.Errorf("problems = %s", rec.all())
	}
}

func TestRun_AnAbsentIndexThatIsNotReportedAbsentIsCaught(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	h := fakeHarness(codeindex.AllCapabilities())
	h.NewAbsent = h.New

	codeindextest.Run(rec, h)

	if !strings.Contains(rec.all(), "absent index must be ErrAbsent") {
		t.Errorf("problems = %s", rec.all())
	}
}

func TestRun_AnUnservedQueryThatDoesNotAnswerUnsupportedIsCaught(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	answer := func(codeindex.Request) ([]codeindex.Hit, error) { return []codeindex.Hit{{File: "x.go"}}, nil }

	codeindextest.Run(rec, scriptedHarness(codeindex.Capabilities{}, answer))

	if !strings.Contains(rec.all(), "unserved query must be ErrUnsupported") {
		t.Errorf("problems = %s", rec.all())
	}
}

func TestRun_AMissingProbeIsReportedPerQuery(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	h := fakeHarness(codeindex.AllCapabilities())
	h.Probes = h.Probes[:2]

	codeindextest.Run(rec, h)

	if !strings.Contains(rec.all(), "no probe for query find") || !strings.Contains(rec.all(), "no probe for query map") {
		t.Errorf("problems = %s", rec.all())
	}
}

type refreshable struct {
	codeindex.Backend
	err error
}

func (r refreshable) Refresh(context.Context) error { return r.err }

func refreshHarness(healthy, absent error) codeindextest.Harness {
	caps := codeindex.AllCapabilities()
	h := fakeHarness(caps)
	h.New = func() codeindex.Backend {
		return refreshable{Backend: codeindextest.NewFake("fake", caps, codeindextest.StandardIndex()), err: healthy}
	}
	h.NewAbsent = func() codeindex.Backend {
		return refreshable{Backend: codeindextest.NewAbsentFake("fake", caps), err: absent}
	}
	return h
}

func TestRun_ARefreshableBackendThatRefreshesAndReportsAbsentPasses(t *testing.T) {
	t.Parallel()
	rec := &recorder{}

	codeindextest.Run(rec, refreshHarness(nil, fmt.Errorf("%w: gone", codeindex.ErrAbsent)))

	if rec.all() != "" {
		t.Errorf("problems:\n%s", rec.all())
	}
}

func TestRun_ARefreshableBackendWhoseHealthyRefreshFailsIsReported(t *testing.T) {
	t.Parallel()
	rec := &recorder{}

	codeindextest.Run(rec, refreshHarness(errors.New("boom"), fmt.Errorf("%w: gone", codeindex.ErrAbsent)))

	if !strings.Contains(rec.all(), "refresh of a present index must succeed") {
		t.Errorf("problems:\n%s", rec.all())
	}
}

func TestRun_ARefreshableBackendThatDoesNotReportAbsentIsReported(t *testing.T) {
	t.Parallel()
	rec := &recorder{}

	codeindextest.Run(rec, refreshHarness(nil, nil))

	if !strings.Contains(rec.all(), "refresh of an absent index must be ErrAbsent") {
		t.Errorf("problems:\n%s", rec.all())
	}
}

func TestRun_ABackendThatCannotRefreshNeedsNoRefreshProbe(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	h := fakeHarness(codeindex.AllCapabilities())
	h.New = func() codeindex.Backend {
		return struct{ codeindex.Backend }{codeindextest.NewFake("fake", codeindex.AllCapabilities(), codeindextest.StandardIndex())}
	}

	codeindextest.Run(rec, h)

	if rec.all() != "" {
		t.Errorf("problems:\n%s", rec.all())
	}
}
