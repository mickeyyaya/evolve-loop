package registry

import (
	"context"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// fakeRunner implements core.PhaseRunner with deterministic behavior
// for registry-level tests. The runner doesn't do real dispatch; the
// registry only cares that a Factory returns a valid PhaseRunner.
type fakeRunner struct{ name string }

func (f *fakeRunner) Name() string { return f.name }
func (f *fakeRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{Phase: f.name}, nil
}

func factory(name string) Factory {
	return func(req core.PhaseRequest) core.PhaseRunner { return &fakeRunner{name: name} }
}

// TestRegister_RoundTrip — register then look up.
func TestRegister_RoundTrip(t *testing.T) {
	ResetForTesting()
	Register("alpha", factory("alpha"))
	f, ok := For("alpha")
	if !ok {
		t.Fatal("For(alpha) not found after Register")
	}
	runner := f(core.PhaseRequest{})
	if runner.Name() != "alpha" {
		t.Errorf("runner name=%q, want alpha", runner.Name())
	}
}

// TestRegister_DuplicatePanic — registering twice panics so init-time
// conflicts surface at startup.
func TestRegister_DuplicatePanic(t *testing.T) {
	ResetForTesting()
	Register("dup", factory("dup"))
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("duplicate Register did not panic")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "duplicate Register") {
			t.Errorf("panic msg=%v, want substring 'duplicate Register'", r)
		}
	}()
	Register("dup", factory("dup-again"))
}

// TestRegister_EmptyNamePanics — defense against silent registration.
func TestRegister_EmptyNamePanics(t *testing.T) {
	ResetForTesting()
	defer func() {
		if recover() == nil {
			t.Fatal("Register(\"\") did not panic")
		}
	}()
	Register("", factory("x"))
}

// TestRegister_NilFactoryPanics — defense against nil-factory lookups
// later producing nil derefs.
func TestRegister_NilFactoryPanics(t *testing.T) {
	ResetForTesting()
	defer func() {
		if recover() == nil {
			t.Fatal("Register(nil) did not panic")
		}
	}()
	Register("nilf", nil)
}

// TestFor_MissingReturnsFalse — unknown name → (nil, false).
func TestFor_MissingReturnsFalse(t *testing.T) {
	ResetForTesting()
	f, ok := For("nope")
	if ok {
		t.Error("For(nope) returned ok=true unexpectedly")
	}
	if f != nil {
		t.Error("For(nope) returned non-nil factory")
	}
}

// TestNames_Sorted — caller-visible API guarantee that Names() is
// stable across runs (sorted) so docs-contract tests can golden-compare.
func TestNames_Sorted(t *testing.T) {
	ResetForTesting()
	Register("zeta", factory("zeta"))
	Register("alpha", factory("alpha"))
	Register("mu", factory("mu"))

	got := Names()
	want := []string{"alpha", "mu", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("Names() len=%d, want %d (got %v)", len(got), len(want), got)
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("Names() not sorted: %v", got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("Names()[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

// TestNames_Empty — fresh registry returns an empty (non-nil) slice.
func TestNames_Empty(t *testing.T) {
	ResetForTesting()
	got := Names()
	if got == nil {
		t.Error("Names() returned nil; want empty slice")
	}
	if len(got) != 0 {
		t.Errorf("Names() len=%d, want 0", len(got))
	}
}

// TestConcurrentLookupsAreSafe — RWMutex correctness under -race.
// Registers a few factories then runs N parallel lookups.
func TestConcurrentLookupsAreSafe(t *testing.T) {
	ResetForTesting()
	Register("a", factory("a"))
	Register("b", factory("b"))
	Register("c", factory("c"))

	var wg sync.WaitGroup
	const workers = 16
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = For("a")
				_, _ = For("b")
				_, _ = For("c")
				_ = Names()
			}
		}()
	}
	wg.Wait()
}

// TestResetForTesting_Clears — confirm tests can rebuild the registry
// from scratch.
func TestResetForTesting_Clears(t *testing.T) {
	ResetForTesting()
	Register("temp", factory("temp"))
	if _, ok := For("temp"); !ok {
		t.Fatal("setup: temp not registered")
	}
	ResetForTesting()
	if _, ok := For("temp"); ok {
		t.Error("ResetForTesting did not clear")
	}
	if len(Names()) != 0 {
		t.Error("ResetForTesting left names behind")
	}
}
