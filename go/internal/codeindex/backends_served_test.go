package codeindex_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
)

func servedHit() []codeindex.Hit {
	return []codeindex.Hit{{File: "a.go", Line: 1, Symbol: "s"}}
}

func callersGotypesFindTrail(index map[codeindex.Request][]codeindex.Hit) codeindex.Backends {
	return codeindex.NewBackends(
		codeindextest.NewFake("gotypes", codeindex.Capabilities{Callers: true, API: true}, index),
		codeindextest.NewFake("trail", codeindex.Capabilities{Find: true, Map: true}, index),
	)
}

func TestBackendsSearchServed_ReportsTheBackendThatAnsweredEachQuery(t *testing.T) {
	t.Parallel()
	callers := codeindex.Request{Query: codeindex.QueryCallers, Arg: "run", Direction: codeindex.DirectionIn}
	find := codeindex.Request{Query: codeindex.QueryFind, Arg: "run"}
	backends := callersGotypesFindTrail(map[codeindex.Request][]codeindex.Hit{callers: servedHit(), find: servedHit()})

	for req, want := range map[codeindex.Request]string{callers: "gotypes", find: "trail"} {
		hits, served, err := backends.SearchServed(context.Background(), req)

		if err != nil || len(hits) != 1 || served != want {
			t.Errorf("SearchServed(%s) = %v, %q, %v; want 1 hit from %q", req.Query, hits, served, err, want)
		}
	}
}

func TestBackendsSearchServed_TheReportedNameFollowsTheBackendNotAConstant(t *testing.T) {
	t.Parallel()
	find := codeindex.Request{Query: codeindex.QueryFind, Arg: "run"}
	backends := codeindex.NewBackends(codeindextest.NewFake("renamed", codeindex.AllCapabilities(), map[codeindex.Request][]codeindex.Hit{find: servedHit()}))

	_, served, err := backends.SearchServed(context.Background(), find)

	if err != nil || served != "renamed" {
		t.Errorf("served = %q, err = %v; want renamed", served, err)
	}
}

func TestBackendsSearchServed_ABackendThatFailsIsStillNamed(t *testing.T) {
	t.Parallel()
	find := codeindex.Request{Query: codeindex.QueryFind, Arg: "missing"}
	backends := codeindex.NewBackends(codeindextest.NewAbsentFake("trail", codeindex.AllCapabilities()))

	hits, served, err := backends.SearchServed(context.Background(), find)

	if !errors.Is(err, codeindex.ErrAbsent) || len(hits) != 0 || served != "trail" {
		t.Errorf("SearchServed = %v, %q, %v; want ErrAbsent from trail", hits, served, err)
	}
}

func TestBackendsSearchServed_NoBackendServingTheQueryNamesNone(t *testing.T) {
	t.Parallel()
	backends := codeindex.NewBackends(codeindextest.NewFake("gotypes", codeindex.Capabilities{Callers: true}, nil))

	hits, served, err := backends.SearchServed(context.Background(), codeindex.Request{Query: codeindex.QueryFind, Arg: "x"})

	if !errors.Is(err, codeindex.ErrUnsupported) || len(hits) != 0 || served != "" {
		t.Errorf("SearchServed = %v, %q, %v; want ErrUnsupported and no backend", hits, served, err)
	}
}

func TestBackendsSearchServed_SearchKeepsAnsweringTheSameHits(t *testing.T) {
	t.Parallel()
	find := codeindex.Request{Query: codeindex.QueryFind, Arg: "run"}
	backends := callersGotypesFindTrail(map[codeindex.Request][]codeindex.Hit{find: servedHit()})

	viaSearch, errSearch := backends.Search(context.Background(), find)
	viaServed, _, errServed := backends.SearchServed(context.Background(), find)

	if errSearch != nil || errServed != nil || len(viaSearch) != len(viaServed) || viaSearch[0] != viaServed[0] {
		t.Errorf("Search = %v, %v; SearchServed = %v, %v", viaSearch, errSearch, viaServed, errServed)
	}
}
