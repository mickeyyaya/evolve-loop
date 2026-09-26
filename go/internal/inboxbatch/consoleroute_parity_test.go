package inboxbatch_test

import (
	"strings"
	"testing"
)

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
