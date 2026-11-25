package codeindex_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
)

var callersRun = codeindex.Request{Query: codeindex.QueryCallers, Arg: "run", Direction: codeindex.DirectionIn}

type failingBackend struct {
	codeindextest.Fake
	err error
}

func (f failingBackend) Callers(context.Context, string, codeindex.Direction) ([]codeindex.Hit, error) {
	return nil, f.err
}

func factoriesOf(backends ...codeindex.Backend) (names []string, factories map[string]codeindex.Factory) {
	factories = map[string]codeindex.Factory{}
	for _, backend := range backends {
		names = append(names, backend.Name())
		factories[backend.Name()] = func() (codeindex.Backend, error) { return backend, nil }
	}
	return names, factories
}

func routeOf(t *testing.T, backends []codeindex.Backend, opts ...codeindex.Option) codeindex.Backends {
	t.Helper()
	names, factories := factoriesOf(backends...)
	set, err := codeindex.New(names, factories, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return set
}

func absentGotypesThenTrail() []codeindex.Backend {
	return []codeindex.Backend{
		codeindextest.NewAbsentFake("gotypes", codeindex.Capabilities{Callers: true, API: true}),
		codeindextest.NewFake("trail", codeindex.AllCapabilities(), map[codeindex.Request][]codeindex.Hit{callersRun: servedHit()}),
	}
}

func TestBackendsOnAbsent_WithoutTheOptionAnAbsentBackendRefusesAndTheNextIsNotAsked(t *testing.T) {
	t.Parallel()
	set := routeOf(t, absentGotypesThenTrail())

	hits, served, err := set.SearchServed(context.Background(), callersRun)

	if !errors.Is(err, codeindex.ErrAbsent) || len(hits) != 0 || served != "gotypes" {
		t.Errorf("SearchServed = %v, %q, %v; want ErrAbsent from gotypes and no hit", hits, served, err)
	}
}

func TestBackendsOnAbsent_WithTheOptionTheNextCapableBackendAnswersAndIsNamed(t *testing.T) {
	t.Parallel()
	set := routeOf(t, absentGotypesThenTrail(), codeindex.WithAbsentFallThrough())

	hits, served, err := set.SearchServed(context.Background(), callersRun)

	if err != nil || len(hits) != 1 || served != "trail" {
		t.Errorf("SearchServed = %v, %q, %v; want 1 hit from trail", hits, served, err)
	}
}

func TestBackendsOnAbsent_SearchFallsThroughLikeSearchServed(t *testing.T) {
	t.Parallel()
	set := routeOf(t, absentGotypesThenTrail(), codeindex.WithAbsentFallThrough())

	hits, err := set.Search(context.Background(), callersRun)

	if err != nil || len(hits) != 1 {
		t.Errorf("Search = %v, %v; want the hit of trail", hits, err)
	}
}

func TestBackendsOnAbsent_APresentBackendAnswersAndTheOptionChangesNothing(t *testing.T) {
	t.Parallel()
	backends := []codeindex.Backend{
		codeindextest.NewFake("gotypes", codeindex.Capabilities{Callers: true}, map[codeindex.Request][]codeindex.Hit{callersRun: servedHit()}),
		codeindextest.NewFake("trail", codeindex.AllCapabilities(), map[codeindex.Request][]codeindex.Hit{callersRun: {{File: "other.go", Line: 9, Symbol: "x"}}}),
	}

	hits, served, err := routeOf(t, backends, codeindex.WithAbsentFallThrough()).SearchServed(context.Background(), callersRun)

	if err != nil || served != "gotypes" || len(hits) != 1 || hits[0].File != "a.go" {
		t.Errorf("SearchServed = %v, %q, %v; want the hit of gotypes", hits, served, err)
	}
}

func TestBackendsOnAbsent_ANotFoundAnswerNeverFallsThrough(t *testing.T) {
	t.Parallel()
	backends := []codeindex.Backend{
		codeindextest.NewFake("gotypes", codeindex.Capabilities{Callers: true}, nil),
		codeindextest.NewFake("trail", codeindex.AllCapabilities(), map[codeindex.Request][]codeindex.Hit{callersRun: servedHit()}),
	}

	hits, served, err := routeOf(t, backends, codeindex.WithAbsentFallThrough()).SearchServed(context.Background(), callersRun)

	if !errors.Is(err, codeindex.ErrNotFound) || len(hits) != 0 || served != "gotypes" {
		t.Errorf("SearchServed = %v, %q, %v; want ErrNotFound from gotypes", hits, served, err)
	}
}

func TestBackendsOnAbsent_AnErrorThatIsNotAbsentNeverFallsThrough(t *testing.T) {
	t.Parallel()
	broken := errors.New("gotypes: index is corrupt")
	backends := []codeindex.Backend{
		failingBackend{Fake: codeindextest.NewFake("gotypes", codeindex.Capabilities{Callers: true}, nil), err: broken},
		codeindextest.NewFake("trail", codeindex.AllCapabilities(), map[codeindex.Request][]codeindex.Hit{callersRun: servedHit()}),
	}

	hits, served, err := routeOf(t, backends, codeindex.WithAbsentFallThrough()).SearchServed(context.Background(), callersRun)

	if !errors.Is(err, broken) || len(hits) != 0 || served != "gotypes" {
		t.Errorf("SearchServed = %v, %q, %v; want the corrupt-index error from gotypes", hits, served, err)
	}
}

func TestBackendsOnAbsent_EveryCapableBackendAbsentSurfacesTheFirstAbsentError(t *testing.T) {
	t.Parallel()
	backends := []codeindex.Backend{
		codeindextest.NewAbsentFake("gotypes", codeindex.Capabilities{Callers: true}),
		codeindextest.NewAbsentFake("trail", codeindex.AllCapabilities()),
	}

	hits, served, err := routeOf(t, backends, codeindex.WithAbsentFallThrough()).SearchServed(context.Background(), callersRun)

	if !errors.Is(err, codeindex.ErrAbsent) || len(hits) != 0 || served != "gotypes" {
		t.Errorf("SearchServed = %v, %q, %v; want ErrAbsent naming the first backend", hits, served, err)
	}
}

func TestBackendsOnAbsent_TheChainWalksPastEachAbsentBackendInOrder(t *testing.T) {
	t.Parallel()
	backends := []codeindex.Backend{
		codeindextest.NewAbsentFake("gotypes", codeindex.Capabilities{Callers: true}),
		codeindextest.NewAbsentFake("scip", codeindex.Capabilities{Callers: true}),
		codeindextest.NewFake("trail", codeindex.AllCapabilities(), map[codeindex.Request][]codeindex.Hit{callersRun: servedHit()}),
	}

	_, served, err := routeOf(t, backends, codeindex.WithAbsentFallThrough()).SearchServed(context.Background(), callersRun)

	if err != nil || served != "trail" {
		t.Errorf("served = %q, err = %v; want trail", served, err)
	}
}

func TestBackendsOnAbsent_AFallBackNeverReachesABackendThatDoesNotServeTheQuery(t *testing.T) {
	t.Parallel()
	backends := []codeindex.Backend{
		codeindextest.NewAbsentFake("gotypes", codeindex.Capabilities{Callers: true}),
		codeindextest.NewFake("trail", codeindex.Capabilities{Find: true, Map: true}, nil),
	}

	hits, served, err := routeOf(t, backends, codeindex.WithAbsentFallThrough()).SearchServed(context.Background(), callersRun)

	if !errors.Is(err, codeindex.ErrAbsent) || errors.Is(err, codeindex.ErrUnsupported) || len(hits) != 0 || served != "gotypes" {
		t.Errorf("SearchServed = %v, %q, %v; want ErrAbsent from gotypes", hits, served, err)
	}
}

func TestBackendsOnAbsent_ALaterBackendThatFailsOtherwiseSurfacesItsOwnError(t *testing.T) {
	t.Parallel()
	broken := errors.New("trail: exit 7")
	backends := []codeindex.Backend{
		codeindextest.NewAbsentFake("gotypes", codeindex.Capabilities{Callers: true}),
		failingBackend{Fake: codeindextest.NewFake("trail", codeindex.AllCapabilities(), nil), err: broken},
	}

	_, served, err := routeOf(t, backends, codeindex.WithAbsentFallThrough()).SearchServed(context.Background(), callersRun)

	if !errors.Is(err, broken) || served != "trail" {
		t.Errorf("served = %q, err = %v; want the error of trail", served, err)
	}
}

func TestBackendsOnAbsent_ALaterNotFoundIsReportedByTheBackendThatAnswered(t *testing.T) {
	t.Parallel()
	backends := []codeindex.Backend{
		codeindextest.NewAbsentFake("gotypes", codeindex.Capabilities{Callers: true}),
		codeindextest.NewFake("trail", codeindex.AllCapabilities(), nil),
	}

	_, served, err := routeOf(t, backends, codeindex.WithAbsentFallThrough()).SearchServed(context.Background(), callersRun)

	if !errors.Is(err, codeindex.ErrNotFound) || served != "trail" {
		t.Errorf("served = %q, err = %v; want ErrNotFound from trail", served, err)
	}
}
