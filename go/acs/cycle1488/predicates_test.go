//go:build acs

// Package cycle1488 materialises the acceptance criteria for the single
// fleet-scoped task pinned to this lane: `verdict-cache-fresh-base-collision`.
//
// State of the tree at RED. The inline fresh-base guard already exists at BOTH
// verdict-cache call sites (orchestrator.go's pre-loop ADR-0048 shadow probe and
// phase_bindings.go's audit-binding Put), and the pre-existing regression
// TestVerdictCacheCollisionRegression is GREEN. What does NOT exist is the
// deterministic, exported eligibility predicate the scout report calls for — the
// guard is a comparison duplicated at two sites, so a future enforce-stage
// lookup has nothing to reuse and the two copies can drift independently. The
// cycle-1488 bar is therefore single-sourcing, not re-fixing: expose
// verdictcache.ProbeEligible(baseTreeSHA, candidateTreeSHA) and route both call
// sites through it without changing observed behaviour.
//
// Empty-base semantics are pinned to what SHIPPED, not to a fresh reading of the
// scout text: an empty/unresolvable base tree keeps the candidate ELIGIBLE
// (TestVerdictCacheCollisionRegression's "missing base remains lookup eligible"
// row is frozen). Only an empty CANDIDATE is rejected outright — it carries no
// content identity, matching Lookup/Put's existing empty-SHA no-op.
//
// Predicate strategy (cycle-85 degenerate-predicate ban):
//   - 001 CALLS the predicate directly over a positive/negative/edge table.
//   - 002 runs the real production path (RunCycle) via the named core
//     integration test and asserts the orchestrator's probe decision agrees with
//     the predicate; the source assertion is auxiliary single-sourcing evidence.
//   - 003 runs the frozen pre-existing collision regression (no behaviour change)
//     and asserts the audit-binding Put site is routed through the predicate
//     rather than keeping its own copy of the comparison.
//   - 004 runs the verdictcache package suite and pins the ADR-0069 apicover
//     obligation for the new exported symbol.
package cycle1488

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// runFromGoRoot executes cmd with its working directory pinned to the module
// root and returns combined output plus exit code. Each caller builds its own
// exec.Command literal so the NARROW scoping (one named package, always with a
// -run filter) is visible at the call site — whole-repo sweeps are the
// regression suite's job, never a cycle predicate's.
func runFromGoRoot(t *testing.T, cmd *exec.Cmd) (string, int) {
	t.Helper()
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%v: %v", cmd.Args, err)
	}
	return string(out), code
}

// TestC1488_001_ProbeEligibleRejectsFreshBase exercises the shared predicate
// directly: an unchanged (fresh) worktree tree is not a re-land candidate, a
// candidate with no content identity is never eligible, and a genuinely changed
// tree — or one whose base could not be resolved — stays eligible.
func TestC1488_001_ProbeEligibleRejectsFreshBase(t *testing.T) {
	cases := []struct {
		name      string
		base      string
		candidate string
		want      bool
	}{
		{name: "fresh worktree equals base", base: "tree-aaa", candidate: "tree-aaa", want: false},
		{name: "empty candidate has no identity", base: "tree-aaa", candidate: "", want: false},
		{name: "both identities empty", base: "", candidate: "", want: false},
		{name: "changed worktree", base: "tree-aaa", candidate: "tree-bbb", want: true},
		{name: "unresolvable base stays eligible", base: "", candidate: "tree-bbb", want: true},
	}
	for _, tc := range cases {
		if got := verdictcache.ProbeEligible(tc.base, tc.candidate); got != tc.want {
			t.Errorf("RED %s: ProbeEligible(%q, %q) = %t, want %t",
				tc.name, tc.base, tc.candidate, got, tc.want)
		}
	}
}

// TestC1488_002_ShadowProbeWiredToSharedPredicate is the wiring proof: the
// pre-loop ADR-0048 shadow probe must derive eligibility from the shared
// predicate, proven from the real RunCycle path by the differential oracle in
// internal/core/verdict_cache_probe_wiring_test.go.
func TestC1488_002_ShadowProbeWiredToSharedPredicate(t *testing.T) {
	out, code := runFromGoRoot(t, exec.Command("go", "test", "-tags", "integration", "-count=1",
		"-run", "TestVerdictCacheProbeEligibilityWiring", "./internal/core"))
	if code != 0 {
		t.Errorf("RED: shadow-probe wiring oracle failed (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: TestVerdictCacheProbeEligibilityWiring did not run:\n%s", out)
	}
	// Auxiliary single-sourcing evidence — the behavioural oracle above carries
	// the weight; this pins WHERE the decision comes from.
	orch := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core", "orchestrator.go")
	if !acsassert.FileContains(t, orch, "verdictcache.ProbeEligible(") {
		t.Errorf("RED: %s does not call verdictcache.ProbeEligible — the guard is still a local copy", orch)
	}
}

// TestC1488_003_AuditBindingPutWiredToSharedPredicate pins the second call site.
// recordAuditBinding carries its own copy of the same fresh-base comparison; it
// must route through the predicate, and the frozen pre-existing collision
// regression must stay GREEN (no observable behaviour change).
func TestC1488_003_AuditBindingPutWiredToSharedPredicate(t *testing.T) {
	out, code := runFromGoRoot(t, exec.Command("go", "test", "-tags", "integration", "-count=1",
		"-run", "TestVerdictCacheCollisionRegression", "./internal/core"))
	if code != 0 {
		t.Errorf("RED: pre-existing collision regression broke (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: TestVerdictCacheCollisionRegression did not run — vacuous check:\n%s", out)
	}
	bindings := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core", "phase_bindings.go")
	if !acsassert.FileContains(t, bindings, "verdictcache.ProbeEligible(") {
		t.Errorf("RED: %s does not call verdictcache.ProbeEligible — the Put guard is still a local copy", bindings)
	}
	// FileNotContains, not an inverted FileContains: the positive primitive
	// Errorf's internally on the absent (correct) state, so the inverted idiom
	// is red on every tree — the documented cycle-352 class, relived as the
	// cycle-1488/1492/1495 three-burn anchor before this salvage repaired it.
	if !acsassert.FileNotContains(t, bindings, "worktreeTree == headTree") {
		t.Errorf("RED: %s still holds the duplicated inline comparison alongside the shared predicate", bindings)
	}
}

// TestC1488_004_ProbeEligibleCoveredByPackageSuite runs the verdictcache suite
// (the package is already enrolled in go/.apicover-enforce, so every exported
// symbol must be named in a real assertion) and pins that the new symbol is
// named there rather than left to trip the repo-wide gate.
func TestC1488_004_ProbeEligibleCoveredByPackageSuite(t *testing.T) {
	out, code := runFromGoRoot(t, exec.Command("go", "test", "-count=1",
		"-run", "TestStore|TestProbeEligible", "./internal/verdictcache"))
	if code != 0 {
		t.Errorf("RED: verdictcache suite failed (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: verdictcache suite selection matched nothing — vacuous check:\n%s", out)
	}
	named := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "verdictcache", "apicover_named_test.go")
	if !acsassert.FileContains(t, named, "ProbeEligible") {
		t.Errorf("RED: %s does not name ProbeEligible — the repo-wide apicover gate (ADR-0069) will reject it", named)
	}
}
