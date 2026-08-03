package reachabilityprobe

// apicover_named_test.go — repo-wide apicover public-API coverage (House
// Rule 1 / ADR-0069's second gate): names and exercises every exported symbol
// of this package (ImportGraph, CallSite, Violation, CheckCallSite,
// BuildImportGraph, FrozenTestFiles, ExtractFrozenPins, CheckFrozenPins) by
// identifier.

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExportedSymbols_Named names every exported identifier of this package
// and pins the two load-bearing contracts: CheckCallSite detects the
// cycle-644 shape (non-nil Violation with a populated Cycle and Error()) and
// leaves an acyclic pin unchanged (nil).
func TestExportedSymbols_Named(t *testing.T) {
	var graph ImportGraph = ImportGraph{
		"storage": {"core"},
		"core":    {},
	}
	cyclic := CallSite{PinningPackage: "core", ReferencedPackage: "storage", Symbol: "UpdateStateMap"}

	var v *Violation = CheckCallSite(graph, cyclic)
	if v == nil {
		t.Fatalf("CheckCallSite(%+v, %+v) = nil, want a *Violation (cycle-644 shape)", graph, cyclic)
	}
	if v.Site != cyclic {
		t.Errorf("Violation.Site = %+v, want %+v", v.Site, cyclic)
	}
	if len(v.Cycle) == 0 {
		t.Error("Violation.Cycle is empty, want a non-empty import chain")
	}
	if v.Error() == "" {
		t.Error("Violation.Error() returned empty string, want a diagnostic message")
	}

	acyclic := CallSite{PinningPackage: "leaf", ReferencedPackage: "storage", Symbol: "UpdateStateMap"}
	if got := CheckCallSite(graph, acyclic); got != nil {
		t.Errorf("CheckCallSite(%+v, %+v) = %+v, want nil (leaf is absent from graph)", graph, acyclic, got)
	}
}

// TestBuildImportGraph_Named names BuildImportGraph and exercises it against
// the real toolchain: a known direct edge (this package imports
// internal/sysexec) must surface in the returned graph, and an unresolvable
// package pattern must produce a wrapped, non-nil error.
func TestBuildImportGraph_Named(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving module root: %v", err)
	}

	const thisPkg = "github.com/mickeyyaya/evolve-loop/go/internal/reachabilityprobe"
	const sysexecPkg = "github.com/mickeyyaya/evolve-loop/go/internal/sysexec"

	var graph ImportGraph
	graph, err = BuildImportGraph(repoRoot, "./internal/reachabilityprobe")
	if err != nil {
		t.Fatalf("BuildImportGraph(%q, ./internal/reachabilityprobe) returned error: %v", repoRoot, err)
	}
	imports, ok := graph[thisPkg]
	if !ok {
		t.Fatalf("graph missing key %q", thisPkg)
	}
	found := false
	for _, imp := range imports {
		if imp == sysexecPkg {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("graph[%q] = %v, want it to contain %q", thisPkg, imports, sysexecPkg)
	}

	if _, err = BuildImportGraph(repoRoot, "./internal/does/not/exist/nope"); err == nil {
		t.Error("BuildImportGraph(bogus package) = nil error, want non-nil")
	}
}

// TestFrozenPinExports_Named names and exercises the frozen-pin seam
// (FrozenTestFiles, ExtractFrozenPins, CheckFrozenPins) end to end against a
// real throwaway module carrying the cycle-644 shape: storage imports core, so
// a frozen test pinning `storage.UpdateStateMap(` into a core file demands
// core -> storage -> core.
func TestFrozenPinExports_Named(t *testing.T) {
	const module = "example.com/named"
	const frozenTest = "go/internal/core/frozen_cyclic_test.go"

	wt := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(wt, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go/go.mod", "module "+module+"\n\ngo 1.23\n")
	write("go/internal/core/state.go", "package core\n\ntype State struct{}\n")
	write("go/internal/storage/storage.go",
		"package storage\n\nimport \""+module+"/internal/core\"\n\nfunc UpdateStateMap(s *core.State) {}\n")
	write(frozenTest,
		"package core\n\nimport \"testing\"\n\nfunc TestC644(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"storage.UpdateStateMap(\")\n}\n")
	report := "## Handoff to Builder\n\n```json\n{\n  \"testFiles\": [\"" + frozenTest + "\"],\n" +
		"  \"doNotModifyTests\": true\n}\n```\n"
	write("test-report.md", report)

	frozen, err := FrozenTestFiles(filepath.Join(wt, "test-report.md"))
	if err != nil {
		t.Fatalf("FrozenTestFiles returned error: %v", err)
	}
	if len(frozen) != 1 || frozen[0] != frozenTest {
		t.Fatalf("FrozenTestFiles = %v, want [%s]", frozen, frozenTest)
	}

	pins, err := ExtractFrozenPins(wt, frozen)
	if err != nil {
		t.Fatalf("ExtractFrozenPins returned error: %v", err)
	}
	want := CallSite{
		PinningPackage:    module + "/internal/core",
		ReferencedPackage: "storage",
		Symbol:            "UpdateStateMap",
	}
	if len(pins) != 1 || pins[0] != want {
		t.Fatalf("ExtractFrozenPins = %+v, want exactly [%+v]", pins, want)
	}

	violations, err := CheckFrozenPins(wt, frozen)
	if err != nil {
		t.Fatalf("CheckFrozenPins returned error: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("CheckFrozenPins = %d violation(s), want 1 (the cycle-644 shape)", len(violations))
	}
	if len(violations[0].Cycle) == 0 {
		t.Error("Violation.Cycle is empty, want the proving import chain")
	}
}
