package reachabilityprobe

// frozenpins_test.go — the DURABLE (non-acs) regression guard for cycle-1246's
// task `reachabilityprobe-alias-resolution`.
//
// Gap (scout-report.md Key Finding #2): resolvePackage maps the bare identifier
// written at a pinned call site to a full import path by matching
// path.Base(pkg) == ident. An identifier introduced by an IMPORT ALIAS matches
// no package's base name, so the pin is silently skipped — a genuine cycle-644
// shape written as
//
//	import st "example.com/fixture/internal/storage"
//	... acsassert.FileContains(t, "go/internal/core/state.go", "st.UpdateStateMap(")
//
// fails OPEN today and the permanently-unsatisfiable acceptance criterion sails
// through `evolve phase verify tdd`.
//
// Where the alias binding can live: NOT in the pinned production file. If
// core/state.go already imported storage while storage imports core, the module
// is already cyclic and `go list` refuses to produce a graph at all (the gate
// then fails open on infra ambiguity, by design). The one place the binding can
// exist while the module still lists cleanly is the FROZEN TEST FILE that
// carries the pin — `go list -deps -json` ignores _test.go imports — and that
// is exactly where a structural test that also exercises the symbol declares it.
// So: aliases are resolved from the import block of the frozen test file the pin
// was extracted from.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// aliasFixtureModule is the module path of the throwaway fixture module.
const aliasFixtureModule = "example.com/fixture"

// Worktree-relative paths of the fixture frozen test files, in the exact form a
// tdd handoff JSON writes them.
const (
	aliasCyclicFrozenTest      = "go/internal/core/frozen_alias_cyclic_test.go"
	aliasReachableFrozenTest   = "go/internal/core/frozen_alias_reachable_test.go"
	aliasUnboundFrozenTest     = "go/internal/core/frozen_alias_unbound_test.go"
	aliasBlankImportFrozenTest = "go/internal/core/frozen_alias_blank_test.go"
	plainCyclicFrozenTest      = "go/internal/core/frozen_plain_cyclic_test.go"
	plainReachableFrozenTest   = "go/internal/core/frozen_plain_reachable_test.go"
)

// writeAliasFixtureFile materialises rel (slash-separated, relative to root).
func writeAliasFixtureFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// aliasFixtureWorktree builds a throwaway worktree whose go/ subdirectory is a
// real, `go list`-resolvable module carrying both shapes the gate must tell
// apart:
//
//	internal/storage  imports internal/core  → pinning it inside a core file
//	                                           is the cycle-644 shape.
//	internal/leafutil imports nothing        → pinning it inside a core file
//	                                           closes no cycle.
//
// Every frozen test file below pins a call site into go/internal/core/state.go;
// the pins differ only in how the referenced package's identifier is bound.
func aliasFixtureWorktree(t *testing.T) string {
	t.Helper()
	wt := t.TempDir()

	writeAliasFixtureFile(t, wt, "go/go.mod", "module "+aliasFixtureModule+"\n\ngo 1.23\n")
	writeAliasFixtureFile(t, wt, "go/internal/core/state.go",
		"package core\n\n// State is the fixture core type.\ntype State struct{}\n")
	writeAliasFixtureFile(t, wt, "go/internal/storage/storage.go",
		"package storage\n\nimport \""+aliasFixtureModule+"/internal/core\"\n\n"+
			"// UpdateStateMap is the cycle-644 symbol: storage already imports core.\n"+
			"func UpdateStateMap(s *core.State) {}\n")
	writeAliasFixtureFile(t, wt, "go/internal/leafutil/leafutil.go",
		"package leafutil\n\n// Helper imports nothing — pinning it closes no cycle.\n"+
			"func Helper() {}\n")

	// ALIASED cycle-644 shape: the binding `st` -> internal/storage is declared
	// in this frozen test file's own import block.
	writeAliasFixtureFile(t, wt, aliasCyclicFrozenTest,
		"package core\n\nimport (\n\t\"testing\"\n\n"+
			"\tst \""+aliasFixtureModule+"/internal/storage\"\n)\n\n"+
			"// TestC644_AliasedStateUsesStorage is the frozen structural pin.\n"+
			"func TestC644_AliasedStateUsesStorage(t *testing.T) {\n"+
			"\t_ = st.UpdateStateMap\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"st.UpdateStateMap(\")\n"+
			"}\n")

	// ALIASED benign shape: `lf` -> internal/leafutil, which imports nothing.
	writeAliasFixtureFile(t, wt, aliasReachableFrozenTest,
		"package core\n\nimport (\n\t\"testing\"\n\n"+
			"\tlf \""+aliasFixtureModule+"/internal/leafutil\"\n)\n\n"+
			"// TestAliasedReachable is a benign aliased structural pin.\n"+
			"func TestAliasedReachable(t *testing.T) {\n"+
			"\t_ = lf.Helper\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"lf.Helper(\")\n"+
			"}\n")

	// UNBOUND identifier: `zz` is bound by nothing. Unresolvable ⇒ fail open.
	writeAliasFixtureFile(t, wt, aliasUnboundFrozenTest,
		"package core\n\nimport \"testing\"\n\n"+
			"// TestUnboundIdentifier pins an identifier no import binds.\n"+
			"func TestUnboundIdentifier(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"zz.Whatever(\")\n"+
			"}\n")

	// BLANK/DOT imports bind no usable identifier ⇒ fail open, and in
	// particular `_` must never be resolved to the storage package.
	writeAliasFixtureFile(t, wt, aliasBlankImportFrozenTest,
		"package core\n\nimport (\n\t\"testing\"\n\n"+
			"\t_ \""+aliasFixtureModule+"/internal/storage\"\n)\n\n"+
			"// TestBlankImport pins through a blank-imported package.\n"+
			"func TestBlankImport(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"_.UpdateStateMap(\")\n"+
			"}\n")

	// UNALIASED control cases — the existing behaviour that must not regress.
	writeAliasFixtureFile(t, wt, plainCyclicFrozenTest,
		"package core\n\nimport \"testing\"\n\n"+
			"// TestPlainCyclic is the unaliased cycle-644 shape.\n"+
			"func TestPlainCyclic(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"storage.UpdateStateMap(\")\n"+
			"}\n")
	writeAliasFixtureFile(t, wt, plainReachableFrozenTest,
		"package core\n\nimport \"testing\"\n\n"+
			"// TestPlainReachable is the unaliased benign shape.\n"+
			"func TestPlainReachable(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"leafutil.Helper(\")\n"+
			"}\n")

	return wt
}

// TestCheckFrozenPins_AliasedImportResolved is the CRUX predicate (scout Task 1
// verifiableBy): a frozen pin written through an import alias must be resolved
// against the real import graph and reported as a violation, not silently
// skipped.
//
// RED today: resolvePackage matches path.Base(pkg) == "st", no package in the
// fixture module is named "st", so ok=false and CheckFrozenPins returns zero
// violations on a compiler-provable cycle-644 shape.
func TestCheckFrozenPins_AliasedImportResolved(t *testing.T) {
	wt := aliasFixtureWorktree(t)

	got, err := CheckFrozenPins(wt, []string{aliasCyclicFrozenTest})
	if err != nil {
		t.Fatalf("CheckFrozenPins(aliased cyclic pin) returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("RED: CheckFrozenPins = %d violation(s), want exactly 1. A pin"+
			" written as st.UpdateStateMap( where the frozen test file declares"+
			" `import st \"%s/internal/storage\"` is the cycle-644 shape (storage"+
			" already imports core); the alias must be resolved from that import"+
			" block instead of being skipped. Got: %+v",
			len(got), aliasFixtureModule, got)
	}

	v := got[0]
	wantReferenced := aliasFixtureModule + "/internal/storage"
	if v.Site.ReferencedPackage != wantReferenced {
		t.Errorf("Violation.Site.ReferencedPackage = %q, want %q — the alias must"+
			" be reported as the FULL import path it binds, so the diagnostic is"+
			" actionable", v.Site.ReferencedPackage, wantReferenced)
	}
	if v.Site.PinningPackage != aliasFixtureModule+"/internal/core" {
		t.Errorf("Violation.Site.PinningPackage = %q, want %q",
			v.Site.PinningPackage, aliasFixtureModule+"/internal/core")
	}
	if v.Site.Symbol != "UpdateStateMap" {
		t.Errorf("Violation.Site.Symbol = %q, want %q", v.Site.Symbol, "UpdateStateMap")
	}
	if len(v.Cycle) == 0 {
		t.Fatal("Violation.Cycle is empty — the proving import chain must be reported")
	}
	if v.Cycle[0] != wantReferenced {
		t.Errorf("Violation.Cycle[0] = %q, want %q (the chain must start at the"+
			" resolved referenced package): %v", v.Cycle[0], wantReferenced, v.Cycle)
	}
	if !strings.Contains(v.Error(), "UpdateStateMap") {
		t.Errorf("Violation.Error() = %q, want it to name the pinned symbol", v.Error())
	}
}

// TestCheckFrozenPins_AliasedImportNoFalsePositive is the false-positive guard
// on the SAME resolution path: teaching the resolver aliases must not make it
// flag every aliased pin. `lf` binds internal/leafutil, which imports nothing,
// so the pin is buildable and must produce no violation.
//
// Without this predicate, a resolver that simply reported any unresolvable-or-
// aliased identifier as a violation would satisfy the crux test and turn the tdd
// gate into a false-HALT generator.
func TestCheckFrozenPins_AliasedImportNoFalsePositive(t *testing.T) {
	wt := aliasFixtureWorktree(t)

	got, err := CheckFrozenPins(wt, []string{aliasReachableFrozenTest})
	if err != nil {
		t.Fatalf("CheckFrozenPins(aliased benign pin) returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("CheckFrozenPins = %+v, want no violations — `lf` binds"+
			" internal/leafutil, which imports nothing, so pinning lf.Helper("+
			" inside package core closes no cycle", got)
	}
}

// TestCheckFrozenPins_AliasUnresolvableFailsOpen pins the fail-open discipline
// for the two ambiguous alias shapes: an identifier no import binds, and a
// blank import (which binds no usable identifier at all). Neither is a
// compiler-provable cycle, so neither may become a confirmed violation — a false
// HALT here taxes every cycle.
func TestCheckFrozenPins_AliasUnresolvableFailsOpen(t *testing.T) {
	wt := aliasFixtureWorktree(t)

	for _, tc := range []struct {
		name   string
		frozen string
	}{
		{name: "identifier_bound_by_nothing", frozen: aliasUnboundFrozenTest},
		{name: "blank_import_binds_no_identifier", frozen: aliasBlankImportFrozenTest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CheckFrozenPins(wt, []string{tc.frozen})
			if err != nil {
				t.Fatalf("CheckFrozenPins(%s) returned error: %v", tc.frozen, err)
			}
			if len(got) != 0 {
				t.Errorf("CheckFrozenPins(%s) = %+v, want no violations — an"+
					" unresolvable identifier is infra ambiguity and must fail OPEN",
					tc.frozen, got)
			}
		})
	}
}

// TestCheckFrozenPins_UnaliasedPathUnchanged is the non-regression guard the
// acceptance criteria summary demands: teaching the resolver aliases must not
// change the verdict on either unaliased shape.
func TestCheckFrozenPins_UnaliasedPathUnchanged(t *testing.T) {
	wt := aliasFixtureWorktree(t)

	bad, err := CheckFrozenPins(wt, []string{plainCyclicFrozenTest})
	if err != nil {
		t.Fatalf("CheckFrozenPins(unaliased cyclic) returned error: %v", err)
	}
	if len(bad) != 1 {
		t.Fatalf("CheckFrozenPins(unaliased cyclic) = %d violation(s), want 1 —"+
			" the pre-existing unaliased path must not regress: %+v", len(bad), bad)
	}
	if bad[0].Site.ReferencedPackage != aliasFixtureModule+"/internal/storage" {
		t.Errorf("unaliased ReferencedPackage = %q, want %q",
			bad[0].Site.ReferencedPackage, aliasFixtureModule+"/internal/storage")
	}

	ok, err := CheckFrozenPins(wt, []string{plainReachableFrozenTest})
	if err != nil {
		t.Fatalf("CheckFrozenPins(unaliased benign) returned error: %v", err)
	}
	if len(ok) != 0 {
		t.Errorf("CheckFrozenPins(unaliased benign) = %+v, want no violations", ok)
	}
}

// TestExtractFrozenPins_AliasIdentifierPreserved pins ExtractFrozenPins'
// documented contract across this change: it reports ReferencedPackage as the
// identifier EXACTLY AS WRITTEN and leaves resolution to CheckFrozenPins, where
// a real import graph is available. An aliased pin must therefore still be
// extracted (not dropped) with the alias identifier intact.
func TestExtractFrozenPins_AliasIdentifierPreserved(t *testing.T) {
	wt := aliasFixtureWorktree(t)

	pins, err := ExtractFrozenPins(wt, []string{aliasCyclicFrozenTest})
	if err != nil {
		t.Fatalf("ExtractFrozenPins returned error: %v", err)
	}
	want := CallSite{
		PinningPackage:    aliasFixtureModule + "/internal/core",
		ReferencedPackage: "st",
		Symbol:            "UpdateStateMap",
	}
	for _, p := range pins {
		if p == want {
			return
		}
	}
	t.Errorf("ExtractFrozenPins = %+v, want it to contain %+v — extraction is"+
		" resolution-free by contract; the alias identifier is resolved later,"+
		" in CheckFrozenPins", pins, want)
}
