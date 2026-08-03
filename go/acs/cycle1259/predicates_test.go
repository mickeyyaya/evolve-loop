//go:build acs

// Package cycle1259 materialises the cycle-1259 acceptance criteria for the one
// task this fleet lane committed in triage `## top_n`:
//
//	wire-importerclosure-into-acssuite-scope (M — predicates 001-003)
//
// Both deferred items (`ciparity-digest-importerclosure-wiring`,
// `egps-regression-tia-selection-full-design`) carry NO predicates, per the R9.3
// floor-binding rule: a predicate may only gate work this cycle committed to.
//
// # The task
//
// `changedpkgs.ImporterClosure` (importerclosure.go:42) landed in cycle 1253 to
// close the cycle-1250 reverse-dependency blind spot — a change confined to
// `internal/router` never selects `internal/routingtest`, which imports router
// and owns the keystone parity invariant (main stayed red for 5 commits). The
// function is unit-tested green and WIRED NOWHERE: `grep -rn ImporterClosure`
// across go/ hits only its own definition, its unit tests, and the cycle-1253
// predicate that checks it compiles. Every real consumer still calls the
// forward-only derivations — including `changedPackagesForCycle`
// (acssuite.go:716), the function that populates CHANGED_PACKAGES, the env var
// EGPS Go predicates scope `go test` to. The fix for the miss exists, compiles,
// and is dead in production; that is why the class recurred a 3rd time
// (inbox item egps-regression-tia-selection, P1, weight 0.91).
//
// # Predicate strategy
//
// Every predicate drives the EXPORTED production entry `acssuite.Run` through
// its `Options.GoExec` seam and asserts on the CHANGED_PACKAGES value Run
// actually hands the predicate env — the same env every real predicate process
// inherits. This is the wiring proof: widening the unexported helper matters
// only if the widened value REACHES the env, and a test that called a helper
// directly would pass on a wiring that never gets there. No predicate greps
// production source (the cycle-85 degenerate-predicate ban), sweeps `/...`, or
// shells a 40s+ suite (the flaky-shape ban; measured baselines are
// internal/acssuite 1.20s, internal/changedpkgs 2.0s).
//
// Fixture roots are a temp dir whose `go` entry is a SYMLINK to the real module,
// so ImporterClosure has the real import graph to trace (the router→routingtest
// edge IS the reproducer) while the `.evolve/runs/` handoff — the other half of
// what projectRoot means to the gate runner — stays inside the temp dir. No
// predicate writes runtime state into the repository.
//
//   - 001 is the crux: the cycle-1250 reproducer through the production env,
//     plus its anti-no-op negatives (a blanket `./...` or a return-everything
//     widening must FAIL).
//   - 002 pins the SECOND handoff branch (handoff-builder.json): a widening
//     applied at one return site only is the same defect, half-fixed.
//   - 003 is the never-narrow floor plus the no-regression check on the two
//     packages the change touches.
//
// # BLOCKER — read the test-report before implementing
//
// `go/internal/acssuite/` is a PROTECTED CONTROL-PLANE SURFACE
// (guards.ProtectedSurfaceManifest, "the gate runner"). The role guard denies
// Edit/Write there from EVERY in-cycle phase, so the production one-liner these
// predicates demand cannot be landed by this cycle's Builder; it requires
// human-gated `evolve ship --class manual` outside a cycle. These predicates are
// the correct, executable RED contract for that operator change — they are
// expected to stay RED until it lands.
package cycle1259

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// The expected CHANGED_PACKAGES entries are DERIVED from the same production
// mapping that produces them (changedpkgs.FileToPackage), never hand-written as
// pattern literals: the SSOT decides the form, and the predicate cannot drift
// from it. routerSourceRel is the fixture's only changed file; routingtest is
// its reverse dependent (the cycle-1250 edge); gitexec is a leaf that cannot
// transitively import router — the negative probe for a return-everything
// widening; and a module-root .go file maps to the blanket pattern, the other
// shape a no-op widening takes.
const (
	routerSourceRel      = "go/internal/router/digest.go"
	routingtestSourceRel = "go/internal/routingtest/agent.go"
	gitexecSourceRel     = "go/internal/gitexec/gitexec.go"
	moduleRootSourceRel  = "go/doc.go"

	// fixtureCycle is the cycle number the fixture handoff is filed under. It is
	// the fixture's own coordinate, independent of the live cycle.
	fixtureCycle = 1259

	// The two packages under test, as full module paths so the assertion is
	// independent of the acs package's cwd.
	acssuitePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	changedpkgsPkg = "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
)

// pkgOf maps a repo-relative Go source path to the CHANGED_PACKAGES pattern the
// production derivation emits for it.
func pkgOf(t *testing.T, repoRelGoFile string) string {
	t.Helper()
	p, ok := changedpkgs.FileToPackage(repoRelGoFile)
	if !ok {
		t.Fatalf("changedpkgs.FileToPackage(%q) rejected a Go source path", repoRelGoFile)
	}
	return p
}

// fixtureRoot builds a projectRoot carrying both halves of what the gate runner
// needs: a `go` symlink to the real module (the import graph) and a handoff
// naming ONLY a router source file. linkModule=false omits the symlink, giving
// the degraded root of predicate 003.
func fixtureRoot(t *testing.T, handoffName string, linkModule bool) string {
	t.Helper()
	root := t.TempDir()
	if linkModule {
		if err := os.Symlink(filepath.Join(acsassert.RepoRoot(t), "go"), filepath.Join(root, "go")); err != nil {
			t.Fatalf("symlink module dir into fixture root: %v", err)
		}
	}
	dir := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", fixtureCycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fixture handoff dir: %v", err)
	}
	doc := map[string]any{
		"thrusts": []any{map[string]any{"files_modified": []string{routerSourceRel}}},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal fixture handoff: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, handoffName), data, 0o644); err != nil {
		t.Fatalf("write fixture handoff: %v", err)
	}
	return root
}

// changedPackagesFromRun drives the production entry and returns the
// CHANGED_PACKAGES value Run exported to the predicate env.
func changedPackagesFromRun(t *testing.T, root string) []string {
	t.Helper()
	var seen []string
	_, err := acssuite.Run(acssuite.Options{
		Root:        root,
		ProjectRoot: root,
		Cycle:       fixtureCycle,
		GoExec: func(_ context.Context, _, _ string, env []string) (string, error) {
			for _, e := range env {
				if strings.HasPrefix(e, "CHANGED_PACKAGES=") {
					seen = strings.Fields(strings.TrimPrefix(e, "CHANGED_PACKAGES="))
				}
			}
			return "", nil
		},
	})
	if err != nil {
		t.Fatalf("acssuite.Run on fixture root %s: %v", root, err)
	}
	return seen
}

func containsPkg(pkgs []string, want string) bool {
	for _, p := range pkgs {
		if p == want {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// AC1 — the cycle-1250 reproducer through the production predicate env, and its
// anti-no-op negatives.
// -----------------------------------------------------------------------------

// TestC1259_001_ReverseDependencyReachesPredicateEnv is the crux predicate.
//
// A cycle whose handoff touches only `internal/router` MUST export a
// CHANGED_PACKAGES that also names `internal/routingtest` — the package that
// imports router and holds the parity invariant a forward-only set never runs.
// The changed package itself must survive alongside it: widening never trades
// one package for another.
//
// The two negatives ride in the same predicate deliberately. A widening to the
// blanket `./...`, or one that returns every module package, satisfies the
// reproducer while making CHANGED_PACKAGES worthless — predicates scope
// `go test` to it, so an over-wide set puts them back to sweeping the whole repo
// (cycle-200, the reason the env var exists at all). gitexec is a leaf that
// cannot transitively import router, so it must stay OUT.
func TestC1259_001_ReverseDependencyReachesPredicateEnv(t *testing.T) {
	got := changedPackagesFromRun(t, fixtureRoot(t, "handoff-build.json", true))

	if len(got) == 0 {
		t.Fatalf("Run exported no CHANGED_PACKAGES for a fixture with a handoff")
	}
	if want := pkgOf(t, routerSourceRel); !containsPkg(got, want) {
		t.Errorf("CHANGED_PACKAGES dropped the changed package %s; got %v", want, got)
	}
	if want := pkgOf(t, routingtestSourceRel); !containsPkg(got, want) {
		t.Errorf("CHANGED_PACKAGES omits reverse dependent %s for a router-only change — the cycle-1250 miss; changedpkgs.ImporterClosure is not wired into acssuite.changedPackagesForCycle; got %v", want, got)
	}
	if bad := pkgOf(t, moduleRootSourceRel); containsPkg(got, bad) {
		t.Errorf("CHANGED_PACKAGES widened to the blanket %s — it stops scoping anything; got %v", bad, got)
	}
	if bad := pkgOf(t, gitexecSourceRel); containsPkg(got, bad) {
		t.Errorf("CHANGED_PACKAGES includes non-importer %s (return-everything widening?); got %v", bad, got)
	}
}

// -----------------------------------------------------------------------------
// AC2 — both handoff branches.
// -----------------------------------------------------------------------------

// TestC1259_002_BuilderHandoffBranchWidensToo pins the SECOND return site.
// `changedPackagesForCycle` tries handoff-build.json then handoff-builder.json
// and returns from whichever hits first; a widening applied to only one of them
// leaves every cycle that emits the other filename on the old forward-only
// scope — the same defect, half-fixed and harder to see, since the reproducer
// above would still be green.
func TestC1259_002_BuilderHandoffBranchWidensToo(t *testing.T) {
	got := changedPackagesFromRun(t, fixtureRoot(t, "handoff-builder.json", true))

	if want := pkgOf(t, routerSourceRel); !containsPkg(got, want) {
		t.Errorf("handoff-builder.json branch dropped the changed package %s; got %v", want, got)
	}
	if want := pkgOf(t, routingtestSourceRel); !containsPkg(got, want) {
		t.Errorf("handoff-builder.json branch omits reverse dependent %s (widening applied to only one return site?); got %v", want, got)
	}
}

// -----------------------------------------------------------------------------
// AC3 — the never-narrow floor, and no regression in the two packages involved.
// -----------------------------------------------------------------------------

// TestC1259_003_NeverNarrowsAndSuitesStayGreen pins the two ways this change
// could be worse than the defect it fixes.
//
// Never-narrow: a projectRoot with no module beneath it makes `go list` fail.
// ImporterClosure's documented best-effort contract is an EMPTY added closure —
// never a lost input — so the exported set must degrade to EXACTLY today's
// forward-only value. A widening that can return nil on a bad root would
// silently disable regression scoping on the cycle that most needs it.
//
// No regression: the two packages the change touches must stay green as whole
// suites, so widening cannot be bought by breaking an existing acssuite
// contract (`changedPackagesForCycle` has standing best-effort pins in
// acssuite_adversarial_test.go).
func TestC1259_003_NeverNarrowsAndSuitesStayGreen(t *testing.T) {
	got := changedPackagesFromRun(t, fixtureRoot(t, "handoff-build.json", false))
	if want := pkgOf(t, routerSourceRel); len(got) != 1 || got[0] != want {
		t.Errorf("a projectRoot with no module must degrade to the forward-only set [%s], never narrower and never wider; got %v", want, got)
	}

	for _, pkg := range []string{acssuitePkg, changedpkgsPkg} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", pkg)
		if code != 0 || err != nil {
			t.Errorf("go test %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", pkg, code, err, stdout, stderr)
		}
	}
}
