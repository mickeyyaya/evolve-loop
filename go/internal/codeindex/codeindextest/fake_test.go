package codeindextest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
)

func TestFake_AnswersEachQueryFromItsIndex(t *testing.T) {
	t.Parallel()
	fake := codeindextest.NewFake("fake", codeindex.AllCapabilities(), codeindextest.StandardIndex())

	for _, probe := range codeindextest.StandardProbes() {
		hits, err := codeindex.Dispatch(context.Background(), fake, probe.Request)

		if err != nil || len(hits) == 0 {
			t.Errorf("Dispatch(%+v) = %v, %v; want hits", probe.Request, hits, err)
		}
	}
}

func TestFake_AQueryWithNoEntryIsNotFound(t *testing.T) {
	t.Parallel()
	fake := codeindextest.NewFake("fake", codeindex.AllCapabilities(), codeindextest.StandardIndex())

	for _, probe := range codeindextest.StandardProbes() {
		_, err := codeindex.Dispatch(context.Background(), fake, probe.Missing)

		if !errors.Is(err, codeindex.ErrNotFound) {
			t.Errorf("Dispatch(%+v) err = %v, want ErrNotFound", probe.Missing, err)
		}
	}
}

func TestFake_AnUnservedQueryIsUnsupportedEvenWithAnEntry(t *testing.T) {
	t.Parallel()
	fake := codeindextest.NewFake("fake", codeindex.Capabilities{Find: true}, codeindextest.StandardIndex())

	for _, probe := range codeindextest.StandardProbes() {
		_, err := codeindex.Dispatch(context.Background(), fake, probe.Request)

		served := probe.Request.Query == codeindex.QueryFind
		if served == errors.Is(err, codeindex.ErrUnsupported) {
			t.Errorf("Dispatch(%+v) err = %v, served = %v", probe.Request, err, served)
		}
	}
}

func TestAbsentFake_EveryServedQueryIsAbsent(t *testing.T) {
	t.Parallel()
	fake := codeindextest.NewAbsentFake("fake", codeindex.AllCapabilities())

	for _, probe := range codeindextest.StandardProbes() {
		_, err := codeindex.Dispatch(context.Background(), fake, probe.Request)

		if !errors.Is(err, codeindex.ErrAbsent) {
			t.Errorf("Dispatch(%+v) err = %v, want ErrAbsent", probe.Request, err)
		}
	}
}

func TestFake_ReportsItsNameAndCapabilities(t *testing.T) {
	t.Parallel()
	caps := codeindex.Capabilities{API: true}

	fake := codeindextest.NewFake("named", caps, nil)

	if fake.Name() != "named" || fake.Capabilities() != caps {
		t.Errorf("Name = %q, Capabilities = %+v", fake.Name(), fake.Capabilities())
	}
}
