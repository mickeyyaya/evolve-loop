package codeindex_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
)

func TestParseDirection_AcceptsInAndOutOnly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw     string
		want    codeindex.Direction
		wantErr bool
	}{
		{"in", codeindex.DirectionIn, false},
		{"out", codeindex.DirectionOut, false},
		{"", "", true},
		{"sideways", "", true},
		{"IN", "", true},
	} {
		got, err := codeindex.ParseDirection(tc.raw)
		if got != tc.want || (err != nil) != tc.wantErr {
			t.Errorf("ParseDirection(%q) = %q, %v; want %q, error=%v", tc.raw, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestParseDirection_TheErrorNamesTheValueAndTheChoices(t *testing.T) {
	t.Parallel()

	_, err := codeindex.ParseDirection("sideways")

	if err == nil || !strings.Contains(err.Error(), `"sideways"`) || !strings.Contains(err.Error(), "in|out") {
		t.Errorf("err = %v, want it to name the value and in|out", err)
	}
}

func TestQueries_ListsTheFourQueriesAndEachCallerGetsItsOwnCopy(t *testing.T) {
	t.Parallel()

	first := codeindex.Queries()
	first[0] = "tampered"
	second := codeindex.Queries()

	want := []codeindex.Query{codeindex.QueryCallers, codeindex.QueryAPI, codeindex.QueryFind, codeindex.QueryMap}
	if len(second) != len(want) {
		t.Fatalf("Queries() = %v, want %v", second, want)
	}
	for i := range want {
		if second[i] != want[i] {
			t.Errorf("Queries()[%d] = %q, want %q", i, second[i], want[i])
		}
	}
}

func TestCapabilities_ServesOnlyTheQueriesItDeclares(t *testing.T) {
	t.Parallel()
	caps := codeindex.Capabilities{Find: true, Map: true}

	for q, want := range map[codeindex.Query]bool{
		codeindex.QueryCallers: false,
		codeindex.QueryAPI:     false,
		codeindex.QueryFind:    true,
		codeindex.QueryMap:     true,
		codeindex.Query("zzz"): false,
	} {
		if got := caps.Serves(q); got != want {
			t.Errorf("Serves(%q) = %v, want %v", q, got, want)
		}
	}
}

func TestAllCapabilities_ServesEveryQuery(t *testing.T) {
	t.Parallel()

	for _, q := range codeindex.Queries() {
		if !codeindex.AllCapabilities().Serves(q) {
			t.Errorf("AllCapabilities does not serve %q", q)
		}
	}
}

func recordingFake(name string, caps codeindex.Capabilities) codeindextest.Fake {
	hit := func(symbol string) []codeindex.Hit {
		return []codeindex.Hit{{File: name + ".go", Line: 1, Symbol: symbol}}
	}
	return codeindextest.NewFake(name, caps, map[codeindex.Request][]codeindex.Hit{
		{Query: codeindex.QueryCallers, Arg: "s", Direction: codeindex.DirectionOut}: hit("callers-out"),
		{Query: codeindex.QueryAPI, Arg: "f.go"}:                                     hit("api"),
		{Query: codeindex.QueryFind, Arg: "words"}:                                   hit("find"),
		{Query: codeindex.QueryMap, Arg: "dir/"}:                                     hit("map"),
	})
}

func TestDispatch_RoutesEachRequestToItsBackendMethodWithItsArguments(t *testing.T) {
	t.Parallel()
	backend := recordingFake("b", codeindex.AllCapabilities())

	for req, want := range map[codeindex.Request]string{
		{Query: codeindex.QueryCallers, Arg: "s", Direction: codeindex.DirectionOut}: "callers-out",
		{Query: codeindex.QueryAPI, Arg: "f.go"}:                                     "api",
		{Query: codeindex.QueryFind, Arg: "words"}:                                   "find",
		{Query: codeindex.QueryMap, Arg: "dir/"}:                                     "map",
	} {
		hits, err := codeindex.Dispatch(context.Background(), backend, req)

		if err != nil || len(hits) != 1 || hits[0].Symbol != want {
			t.Errorf("Dispatch(%+v) = %v, %v; want one hit %q", req, hits, err, want)
		}
	}
}

func TestDispatch_AnUnknownQueryIsUnsupported(t *testing.T) {
	t.Parallel()

	_, err := codeindex.Dispatch(context.Background(), recordingFake("b", codeindex.AllCapabilities()), codeindex.Request{Query: "teleport"})

	if !errors.Is(err, codeindex.ErrUnsupported) || !strings.Contains(err.Error(), "teleport") {
		t.Errorf("err = %v, want ErrUnsupported naming the query", err)
	}
}

func TestBackends_ForReturnsTheFirstBackendThatServesTheQuery(t *testing.T) {
	t.Parallel()
	graph := recordingFake("graph", codeindex.Capabilities{Callers: true, API: true})
	text := recordingFake("text", codeindex.Capabilities{Find: true, API: true})
	set := codeindex.NewBackends(graph, text)

	for q, want := range map[codeindex.Query]string{
		codeindex.QueryCallers: "graph",
		codeindex.QueryAPI:     "graph",
		codeindex.QueryFind:    "text",
	} {
		got, err := set.For(q)

		if err != nil || got.Name() != want {
			t.Errorf("For(%q) = %v, %v; want backend %q", q, got, err, want)
		}
	}
}

func TestBackends_ForReturnsUnsupportedWhenNoBackendServesTheQuery(t *testing.T) {
	t.Parallel()
	set := codeindex.NewBackends(recordingFake("text", codeindex.Capabilities{Find: true}))

	_, err := set.For(codeindex.QueryMap)

	if !errors.Is(err, codeindex.ErrUnsupported) || !strings.Contains(err.Error(), "no backend serves map") {
		t.Errorf("err = %v, want ErrUnsupported with the message no backend serves map", err)
	}
}

func TestBackends_AnEmptySetServesNothing(t *testing.T) {
	t.Parallel()

	_, err := codeindex.NewBackends().For(codeindex.QueryFind)

	if !errors.Is(err, codeindex.ErrUnsupported) {
		t.Errorf("err = %v, want ErrUnsupported", err)
	}
}

func TestBackends_SearchAsksOnlyTheFirstServingBackendAndDoesNotFallThroughOnNotFound(t *testing.T) {
	t.Parallel()
	first := codeindextest.NewFake("first", codeindex.AllCapabilities(), nil)
	second := recordingFake("second", codeindex.AllCapabilities())
	set := codeindex.NewBackends(first, second)

	_, err := set.Search(context.Background(), codeindex.Request{Query: codeindex.QueryFind, Arg: "words"})

	if !errors.Is(err, codeindex.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound from the first backend", err)
	}
}

func TestBackends_SearchReturnsTheHitsOfTheServingBackend(t *testing.T) {
	t.Parallel()
	set := codeindex.NewBackends(
		recordingFake("text", codeindex.Capabilities{Find: true}),
		recordingFake("graph", codeindex.Capabilities{API: true}),
	)

	hits, err := set.Search(context.Background(), codeindex.Request{Query: codeindex.QueryAPI, Arg: "f.go"})

	if err != nil || len(hits) != 1 || hits[0].File != "graph.go" {
		t.Errorf("Search = %v, %v; want the hit of graph", hits, err)
	}
}

func TestBackends_SearchWithNoServingBackendIsUnsupported(t *testing.T) {
	t.Parallel()

	_, err := codeindex.NewBackends().Search(context.Background(), codeindex.Request{Query: codeindex.QueryMap})

	if !errors.Is(err, codeindex.ErrUnsupported) {
		t.Errorf("err = %v, want ErrUnsupported", err)
	}
}

func TestBackends_NamesListsTheChainInOrder(t *testing.T) {
	t.Parallel()
	var set codeindex.Backends = codeindex.NewBackends(recordingFake("a", codeindex.AllCapabilities()), recordingFake("b", codeindex.AllCapabilities()))

	got := strings.Join(set.Names(), ",")

	if got != "a,b" {
		t.Errorf("Names = %q, want a,b", got)
	}
}

func factoryFor(name string) codeindex.Factory {
	return func() (codeindex.Backend, error) {
		return recordingFake(name, codeindex.AllCapabilities()), nil
	}
}

func TestNew_BuildsTheBackendsInTheOrderOfTheNames(t *testing.T) {
	t.Parallel()
	factories := map[string]codeindex.Factory{"graph": factoryFor("graph"), "text": factoryFor("text")}

	set, err := codeindex.New([]string{"text", "graph"}, factories)

	if err != nil || strings.Join(set.Names(), ",") != "text,graph" {
		t.Errorf("New = %v, %v; want the chain text,graph", set.Names(), err)
	}
}

func TestNew_AnUnknownNameFailsLoudlyAndListsTheKnownNames(t *testing.T) {
	t.Parallel()
	factories := map[string]codeindex.Factory{"graph": factoryFor("graph"), "text": factoryFor("text")}

	_, err := codeindex.New([]string{"graph", "gopls"}, factories)

	if err == nil || !strings.Contains(err.Error(), `"gopls"`) || !strings.Contains(err.Error(), "graph, text") {
		t.Errorf("err = %v, want it to name gopls and list graph, text", err)
	}
}

func TestNew_NoNamesFailsLoudly(t *testing.T) {
	t.Parallel()

	_, err := codeindex.New(nil, map[string]codeindex.Factory{"graph": factoryFor("graph")})

	if err == nil || !strings.Contains(err.Error(), "no backend") {
		t.Errorf("err = %v, want a refusal of an empty list", err)
	}
}

func TestNew_ADuplicateNameFailsLoudly(t *testing.T) {
	t.Parallel()

	_, err := codeindex.New([]string{"graph", "graph"}, map[string]codeindex.Factory{"graph": factoryFor("graph")})

	if err == nil || !strings.Contains(err.Error(), `duplicate backend "graph"`) {
		t.Errorf("err = %v, want a duplicate refusal", err)
	}
}

func TestNew_AFactoryErrorIsWrappedWithTheBackendName(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	factories := map[string]codeindex.Factory{"graph": func() (codeindex.Backend, error) { return nil, boom }}

	_, err := codeindex.New([]string{"graph"}, factories)

	if !errors.Is(err, boom) || !strings.Contains(err.Error(), `"graph"`) {
		t.Errorf("err = %v, want boom wrapped with the backend name", err)
	}
}

func TestDispatch_PassesAnAbsentIndexThroughAsErrAbsent(t *testing.T) {
	t.Parallel()
	absent := codeindextest.NewAbsentFake("b", codeindex.AllCapabilities())

	_, err := codeindex.Dispatch(context.Background(), absent, codeindex.Request{Query: codeindex.QueryMap, Arg: "d/"})

	if !errors.Is(err, codeindex.ErrAbsent) {
		t.Errorf("err = %v, want ErrAbsent", err)
	}
}
