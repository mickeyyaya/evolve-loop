package codeindextest

import (
	"context"
	"errors"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

type Probe struct {
	Request    codeindex.Request
	WantSymbol string
	WantFile   string
	Missing    codeindex.Request
}

type Harness struct {
	New       func() codeindex.Backend
	NewAbsent func() codeindex.Backend
	Probes    []Probe
}

type Reporter interface {
	Helper()
	Errorf(format string, args ...any)
}

func Run(t Reporter, h Harness) {
	t.Helper()
	backend := h.New()
	for _, q := range codeindex.Queries() {
		probe, ok := h.probeFor(q)
		if !ok {
			t.Errorf("no probe for query %s", q)
			continue
		}
		for _, problem := range h.problems(backend, probe) {
			t.Errorf("%s %s: %s", backend.Name(), q, problem)
		}
	}
}

func (h Harness) probeFor(q codeindex.Query) (Probe, bool) {
	for _, p := range h.Probes {
		if p.Request.Query == q {
			return p, true
		}
	}
	return Probe{}, false
}

func (h Harness) problems(backend codeindex.Backend, p Probe) []string {
	if !backend.Capabilities().Serves(p.Request.Query) {
		return expect(backend, p.Request, codeindex.ErrUnsupported, "unserved query must be ErrUnsupported")
	}
	problems := knownProblems(backend, p)
	problems = append(problems, expect(backend, p.Missing, codeindex.ErrNotFound, "no answer must be ErrNotFound")...)
	return append(problems, expect(h.NewAbsent(), p.Request, codeindex.ErrAbsent, "absent index must be ErrAbsent")...)
}

func expect(backend codeindex.Backend, req codeindex.Request, want error, message string) []string {
	if _, err := codeindex.Dispatch(context.Background(), backend, req); !errors.Is(err, want) {
		return []string{fmt.Sprintf("%s: got %v", message, err)}
	}
	return nil
}

func knownProblems(backend codeindex.Backend, p Probe) []string {
	hits, err := codeindex.Dispatch(context.Background(), backend, p.Request)
	if err != nil {
		return []string{fmt.Sprintf("known query failed: %v", err)}
	}
	var problems []string
	matched := false
	for _, hit := range hits {
		if hit.File == "" {
			problems = append(problems, "hit without a file")
		}
		matched = matched || p.matches(hit)
	}
	if !matched {
		problems = append(problems, fmt.Sprintf("no hit matches symbol %q file %q in %d hits", p.WantSymbol, p.WantFile, len(hits)))
	}
	return problems
}

func (p Probe) matches(hit codeindex.Hit) bool {
	return (p.WantSymbol == "" || hit.Symbol == p.WantSymbol) && (p.WantFile == "" || hit.File == p.WantFile)
}
