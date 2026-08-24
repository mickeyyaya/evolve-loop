package main

// scope_path_resolver_test.go — the composition-root half of the cycle-1548
// fix, tested against a REAL temp inbox in the live namesake shape.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedInbox(t *testing.T, root, sub, id string) string {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox")
	if sub != "" {
		dir = filepath.Join(dir, sub)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	b, _ := json.Marshal(map[string]any{"id": id})
	p := filepath.Join(dir, "2026-08-22T15-02-52Z-"+id+".json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestScopePathResolver_LiveBeatsNamesakes(t *testing.T) {
	root := t.TempDir()
	live := seedInbox(t, root, "", "pipeline-defect-pipeline-blocker")
	seedInbox(t, root, "consumed", "pipeline-defect-pipeline-blocker")
	seedInbox(t, root, "processed/cycle-0", "pipeline-defect-pipeline-blocker")

	if got := scopePathResolver(root, "pipeline-defect-pipeline-blocker"); got != live {
		t.Fatalf("the resolver must hand back the LIVE record; got %q want %q", got, live)
	}
}

// Mutation note: dropping scopePathResolver's StatePending guard is an
// EQUIVALENT mutant today — ResolveDispatchState populates Path only for
// Pending, so the guard is belt-and-braces against a future resolver change
// that populates Path for other states. It stays because the invariant it
// defends (never hand an agent a non-pending record) is the entire fix.
func TestScopePathResolver_ConsumedOnlyResolvesEmpty(t *testing.T) {
	root := t.TempDir()
	seedInbox(t, root, "consumed", "cured-task")
	if got := scopePathResolver(root, "cured-task"); got != "" {
		t.Fatalf("a consumed-only namesake must NOT be handed to an agent; got %q", got)
	}
	if got := scopePathResolver(root, "never-existed"); got != "" {
		t.Fatalf("an unknown id resolves empty (fail-open); got %q", got)
	}
}

// The resolved path must be inside the inbox root — a defensive pin so a future
// resolver change can never hand an agent a path outside the lifecycle tree.
func TestScopePathResolver_PathStaysUnderTheInbox(t *testing.T) {
	root := t.TempDir()
	seedInbox(t, root, "", "a-task")
	got := scopePathResolver(root, "a-task")
	if got == "" || !strings.Contains(got, filepath.Join(".evolve", "inbox")) {
		t.Fatalf("resolved path must live under .evolve/inbox; got %q", got)
	}
}

// THE WIRING (the ninth NOT-WIRED of the week, and the reason this test
// exists): wireOrchestratorDeps must REGISTER the resolver on the orchestrator
// it builds. The function-level tests above pass even when the composition
// root never wires it — this drives the real builder and probes the result.
func TestWireOrchestratorDeps_RegistersTheScopePathResolver(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	live := seedInbox(t, root, "", "pipeline-defect-pipeline-blocker")
	seedInbox(t, root, "consumed", "pipeline-defect-pipeline-blocker")

	deps := wireOrchestratorDeps(root, evolveDir)
	got, wired := deps.Orchestrator.ScopePathProbe(root, "pipeline-defect-pipeline-blocker")
	if !wired {
		t.Fatalf("the composition root must register the scope-path resolver")
	}
	if got != live {
		t.Fatalf("the wired resolver must hand back the LIVE record; got %q want %q", got, live)
	}
}
