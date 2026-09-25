package inboxbatch_test

import (
	"strings"
	"testing"
)

// consoleroute_parity_test.go — F35 (2026-09-26): the seed refuses at least
// everything triage's breaker would. The breaker and the ship tripwire judge
// MEMBERSHIP of each file with no route exception, so two spellings let a
// declared protected FILE through the seed classifier only for a lane to die
// at triage after paying for scout: an operator route:"lane" on a declared
// protected file (two live items on 2026-09-26, reduced below to their routing
// fields), and a path:line locator ("x.go:178") the path-shape check refused
// to read as a path at all. These tests use the real routing predicate.

// TestConsoleRouted_LaneOverrideCannotRelaxAProtectedFile: a declared protected
// FILE binds whatever the route — the override is refused, loudly.
func TestConsoleRouted_LaneOverrideCannotRelaxAProtectedFile(t *testing.T) {
	for _, rec := range []string{
		`{"id":"cyclerun-seat-helper","kind":"techdebt","route":"lane","files":["go/internal/core/resume.go","go/internal/core/cyclerun.go"]}`,
		`{"id":"bwrap-bind-requires-existing-write-dir","kind":"bug","route":"lane","files":["go/internal/adapters/sandbox/sandbox.go","go/internal/adapters/sandbox/sandbox_test.go"]}`,
		// the kind derivation fires first; the file still binds
		`{"id":"k","kind":"pipeline-repair","route":"lane","files":["go/internal/core/cyclerun.go"]}`,
		// a relaxable directory hit beside a binding file hit: the file binds
		`{"id":"e","kind":"feature","route":"lane","files":["go/internal/core/","go/internal/core/cyclerun.go"]}`,
	} {
		if ok, reason := routed(t, rec); !ok || !strings.Contains(reason, "route:lane cannot relax") || !strings.Contains(reason, ".go") {
			t.Errorf("%s:\n ConsoleRouted = %v %q, want console naming the binding file", rec, ok, reason)
		}
	}
}

// TestConsoleRouted_LaneOverrideStillRelaxesTheHeuristics: scope, mentions and
// kind are derivations a human can correct — a declared DIRECTORY holding
// protected files (triage's card then names the unprotected files the change
// touches, which the breaker passes), a file the text only names, and the
// pipeline-* kind stay overridable by an operator.
func TestConsoleRouted_LaneOverrideStillRelaxesTheHeuristics(t *testing.T) {
	for _, rec := range []string{
		`{"id":"d","kind":"feature","route":"lane","files":["go/internal/core/"]}`,
		`{"id":"m","kind":"bug","route":"lane","fix":"the stall shows in go/internal/core/cyclerun.go"}`,
		`{"id":"p","kind":"pipeline-repair","route":"lane","files":["docs/architecture/routing-notes.md"]}`,
	} {
		if ok, reason := routed(t, rec); ok {
			t.Errorf("%s: an operator route:lane over a heuristic derivation must be honored, got %q", rec, reason)
		}
	}
}

// TestConsoleRouted_LineLocatorsDeclareTheirPath: a trailing source locator is
// part of how authors cite a file, not part of its path.
func TestConsoleRouted_LineLocatorsDeclareTheirPath(t *testing.T) {
	for _, file := range []string{
		"go/internal/core/cyclerun.go:178",
		"go/internal/core/cyclerun.go:178:5",
		"go/internal/core/cyclerun.go:189,205,221",
		"go/internal/core/cyclerun.go:10-20",
		"go/internal/core/cyclerun.go#L10-L20",
		"(go/internal/core/cyclerun.go:42)",
	} {
		rec := `{"id":"l","kind":"bug","files":["` + file + ` (the predicate)"]}`
		if ok, reason := routed(t, rec); !ok || reason != "protected fix surface: go/internal/core/cyclerun.go" {
			t.Errorf("files=[%q]: ConsoleRouted = %v %q, want the located file judged", file, ok, reason)
		}
	}
	if !decode(t, `{"id":"s","files":["go/cmd/evolve/cmd_loop_pool_test.go:189,205,221 (the three 2s deadlines)"]}`).DeclaredSurface() {
		t.Error("a located path declares its surface")
	}
	if ok, reason := routed(t, `{"id":"u","kind":"bug","files":["go/cmd/evolve/cmd_loop_pool_test.go:189"]}`); ok {
		t.Errorf("an unprotected located path stays dispatchable: %q", reason)
	}
}
