//go:build acs

// Package cycle1676 materializes the acceptance criteria of the ONE inbox item
// this fleet lane committed (lane-scope.json todo_ids ∩ triage-report.md
// ## top_n) — and nothing else (R9.3):
//
//	crossartifact-invariant-stack  (medium, weight 0.85, feature)
//
// THE FEATURE. One deterministic per-cycle aggregate over a cycle's own
// artifacts, running the four weak verifiers the inbox record names — embedded
// sentinel == standalone verdict JSON, claimed test counts == independently
// recounted runner results, every cited evidence path resolves on disk, and the
// provenance/phase-order chain is intact. Weaver (arXiv:2506.18203): a stack of
// weak deterministic verifiers approaches strong-verifier power at near-zero
// cost and is immune to LLM-judge bias. Every invariant ships ADVISORY until
// its false-positive rate is evidenced ~0 (the 1054/1060 breaker lesson), which
// is the inbox record's own rule and the hardest thing for this suite to keep
// honest — hence 004.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 the suite reports sentinel/verdict disagreement, test-count
//	    disagreement, missing referenced paths and invalid phase order,
//	    each with concrete evidence                                     → 001
//	AC2 malformed/absent artifacts fail safe and stay distinguishable
//	    from a verified match; an arbitrary PASS string cannot satisfy
//	    the suite                                                       → 002
//	AC3 the aggregate is deterministic, keeps phasecontract as the
//	    canonical sentinel parser, and runs against the lane
//	    workspace/worktree rather than a project-root substitute        → 003
//	AC4 findings stay advisory pending a separate false-positive-rate
//	    graduation; existing verdict-coherence behaviour stays green    → 004, 005
//	AC5 the materialized eval exists with behavioral [code] checks      → 006
//	house rule: the enrolled package's public-API gate stays green      → 007
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 002's prose-PASS
// and fail-safe bindings (a greppable implementation greens 001 and fails
// there), 003's lane-vs-project-root pair, and 004's "four violations still
// close the cycle PASS" — the one an over-eager implementation fails by
// blocking. EDGE/OOD: truncated JSON, wrong-typed counts, an empty results
// array, an object where an array belongs, unparseable timestamps, an empty
// audit report. SEMANTIC: four distinct invariants, determinism, advisory
// wiring, ADR-0072 no-regression, and the eval's own durability — five distinct
// behaviours, not one restated.
//
// Flaky-shape contract: every `go test` names ONE package and is -run narrowed
// (internal/core is a known-slow suite), every go invocation is `go -C` anchored
// to the lane's module root, no wall-clock bounds, no literal PIDs, every git
// call is -C anchored, and the coverage profile 007 needs is written to the
// test's own temp dir — never into the tree.
//
// Reachability probe (cycle-644 rule): this package imports only pkg/acsassert
// and the standard library — a leaf. The frozen bindings are in-package
// (internal/coherence, internal/core); internal/core ALREADY imports
// internal/coherence, so no new import edge is pinned. `go list -deps
// ./internal/phasetiming` carries no edge back to internal/coherence, and a
// compiler probe confirmed internal/coherence -> internal/phasetiming builds.
package cycle1676

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	coherencePkg = "./internal/coherence"
	corePkg      = "./internal/core"

	evalPath = ".evolve/evals/crossartifact-invariant-stack.md"
)

// goModRoot is the lane's Go module root — every `go` invocation is anchored
// there with `go -C`, so a predicate resolves the same package patterns from
// the main tree, this worktree, or any fleet lane's cwd.
func goModRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// assertBindingsPass shells `go -C <module> test -run '^(names)$' -count=1 -v
// <pkg>` against ONE named package and requires EVERY name to print a
// `--- PASS: <name>` line. Asserting on the PASS line, never exit 0, is
// load-bearing: a -run pattern matching NO test exits 0 with "no tests to run",
// so a still-missing binding would false-GREEN.
func assertBindingsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goModRoot(t), "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

// TestC1676_001_FourInvariantsReportIndependentlyWithEvidence — AC1. Each of
// the four invariants is driven over a real fixture workspace and must report
// its own disagreement with evidence that names the disagreeing values: both
// sides of a verdict mismatch, the claimed and the recounted test numbers, the
// missing path by name, and the broken phase chain. The all-ok pole is bound
// too, so an implementation that simply reports "violated" everywhere fails.
func TestC1676_001_FourInvariantsReportIndependentlyWithEvidence(t *testing.T) {
	assertBindingsPass(t, coherencePkg,
		"TestCrossArtifactInvariants_CoherentWorkspaceIsAllOK",
		"TestCrossArtifactInvariants_VerdictDisagreementIsViolatedWithBothSides",
		"TestCrossArtifactInvariants_TestCountDisagreementNamesClaimedAndCounted",
		"TestCrossArtifactInvariants_TestCountTotalDisagreementIsViolated",
		"TestCrossArtifactInvariants_MissingReferencedPathIsViolatedAndNamed",
		"TestCrossArtifactInvariants_PhaseOrderViolationsAreReported",
	)
}

// TestC1676_002_AbsentOrMalformedArtifactsFailSafeAndCannotBeGreppedGreen —
// AC2, the NEGATIVE pole. An empty workspace, truncated JSON, wrong-typed
// counts, an object where an array belongs, unparseable timestamps and an empty
// audit report must each read as indeterminate: never a verified match (which
// is how a presence-only implementation games the suite) and never a violation
// (which is how an advisory earns a false-positive rate and gets switched off).
// The prose-PASS binding is the anti-grep proof — a report stuffed with the word
// PASS, including a placeholder echo of the contract's own example, carries no
// canonical sentinel and must not satisfy the verdict invariant.
func TestC1676_002_AbsentOrMalformedArtifactsFailSafeAndCannotBeGreppedGreen(t *testing.T) {
	assertBindingsPass(t, coherencePkg,
		"TestCrossArtifactInvariants_ProseVerdictCannotSatisfyTheSentinelInvariant",
		"TestCrossArtifactInvariants_AbsentArtifactsAreIndeterminateNotOK",
		"TestCrossArtifactInvariants_MalformedArtifactsAreIndeterminateNotOK",
		"TestCrossArtifactInvariants_ViolationsReturnsOnlyTheViolated",
	)
}

// TestC1676_003_AggregateIsDeterministicAndLaneBound — AC3. Two evaluations of
// one unchanged workspace must be byte-identical over exactly the four named
// invariants in a stable order (an unstable report cannot be diffed across
// cycles, and that diff is the only way an advisory's false-positive rate ever
// becomes measurable). The lane half runs at BOTH levels: cited paths resolve
// under the workspace and the passed worktree in the unit, and the production
// caller hands it the LANE worktree rather than the project root at the seam
// (#612 — a project-root snapshot is not evidence of what a lane changed).
func TestC1676_003_AggregateIsDeterministicAndLaneBound(t *testing.T) {
	assertBindingsPass(t, coherencePkg,
		"TestCrossArtifactInvariants_ReportShapeIsDeterministic",
		"TestCrossArtifactInvariants_ReferencedPathsResolveUnderWorkspaceThenWorktree",
	)
	assertBindingsPass(t, corePkg,
		"TestFinalizeCycle_CrossArtifactBindsTheLaneWorktreeNotTheProjectRoot",
	)
}

// TestC1676_004_FindingsAreRecordedAtTheRealSeamAndStayAdvisory — AC4's
// advisory half, and the wiring proof. The bindings drive the REAL cycle-close
// path (finalizeCycle, the terminal segment RunCycle always reaches): it must
// leave a decodable crossartifact-invariants.json naming all four invariants,
// and four violations must still close the cycle PASS with no system failure. A
// checker nothing calls is the defect class the stack was written to catch
// (#373); an advisory that quietly gained teeth is the 1054/1060 breaker lesson
// the inbox record forbids repeating.
func TestC1676_004_FindingsAreRecordedAtTheRealSeamAndStayAdvisory(t *testing.T) {
	assertBindingsPass(t, corePkg,
		"TestFinalizeCycle_EmitsAdvisoryCrossArtifactInvariantsArtifact",
		"TestFinalizeCycle_CrossArtifactViolationsNeverBlockTheCycle",
	)
}

// TestC1676_005_ExistingVerdictCoherenceBehaviourIsUnchanged — AC4's
// no-regression half, at both levels: the ADR-0072 leaf keeps its forgery
// signature, its reconcile self-heal and its diagnosed-negative exemption, and
// the live floor still halts a recorded-FAIL-with-green-artifacts cycle with
// the advisory aggregate running beside it.
func TestC1676_005_ExistingVerdictCoherenceBehaviourIsUnchanged(t *testing.T) {
	assertBindingsPass(t, coherencePkg,
		"TestCoherence_ResultShape",
		"TestReadCycleVerdicts_Fixture",
		"TestCheckVerdictCoherence",
		"TestCheckVerdictCoherence_Reconcile",
		"TestCheckVerdictCoherence_ForgedStillHalts",
		"TestCrossArtifactInvariants_ExistingVerdictCoherenceIsUntouched",
	)
	assertBindingsPass(t, corePkg,
		"TestDetectVerdictIncoherence_ForgedVerdict_Halts",
		"TestDetectVerdictIncoherence_GenuineFail_NoHalt",
		"TestDetectVerdictIncoherence_ReconcileUsesFullVerify",
		"TestFinalizeCycle_VerdictIncoherenceFloorSurvivesTheAdvisory",
	)
}

// TestC1676_006_MaterializedEvalIsDurableAndBehavioral — AC5. The eval is the
// PERMANENT regression entry (ACS predicates are cycle-scoped and are never
// replayed), so it must exist, must not be gitignored — the cycle-93 shape,
// where a file present on disk was silently dropped at ship — and must carry
// score_cap evidence commands that RUN something. An eval whose evidence is a
// grep for a magic string caps nothing.
func TestC1676_006_MaterializedEvalIsDurableAndBehavioral(t *testing.T) {
	root := acsassert.RepoRoot(t)
	abs := filepath.Join(root, evalPath)
	if !acsassert.FileExists(t, abs) {
		t.Fatalf("RED: %s missing — the cycle's permanent regression entry was never materialized", evalPath)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", evalPath); code == 0 {
		t.Errorf("RED: %s is gitignored — it will be dropped at ship and cap nothing (cycle-93)", evalPath)
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", evalPath, err)
	}
	body := string(raw)
	if !strings.HasPrefix(body, "---\nscore_cap:") {
		t.Errorf("RED: %s has no score_cap frontmatter — nothing caps a future audit score", evalPath)
	}
	if n := strings.Count(body, "- criterion:"); n < 4 {
		t.Errorf("RED: %s declares %d score_cap criteria, want one per behavioral acceptance criterion (>=4)", evalPath, n)
	}
	if n := strings.Count(body, "[code]"); n < 4 {
		t.Errorf("RED: %s carries %d [code] checks, want >=4 behavioral ones", evalPath, n)
	}
	for _, needle := range []string{"go test", "internal/coherence", "internal/core"} {
		if !strings.Contains(body, needle) {
			t.Errorf("RED: %s has no evidence command exercising %q — a non-behavioral eval caps nothing", evalPath, needle)
		}
	}
}

// TestC1676_007_EnrolledPackagePublicAPIGateStaysGreen — the house-rule floor
// for this change. internal/coherence is enrolled in go/.apicover-enforce, so
// every exported symbol this cycle adds must be named by a test that actually
// executes it; an unnamed or false-green export reds the repo-wide gate at
// ship, after the cycle's budget is spent. This runs the SAME gate the Makefile
// composes (`apicover -enforce -cover <profile> <pkgdir>`) over one named
// package, with the coverage profile written to the test's own temp dir so the
// tree is never touched.
func TestC1676_007_EnrolledPackagePublicAPIGateStaysGreen(t *testing.T) {
	mod := goModRoot(t)
	if !acsassert.FileContains(t, filepath.Join(mod, ".apicover-enforce"), coherencePkg) {
		t.Skipf("internal/coherence is not enrolled in .apicover-enforce — nothing for this gate to enforce")
	}

	tmp := t.TempDir()
	profile := filepath.Join(tmp, "coverage.txt")
	if _, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", mod, "test", "-count=1", "-coverprofile="+profile, coherencePkg); code != 0 {
		t.Fatalf("RED: `go test -coverprofile` on %s exited %d: %v\nstderr:\n%s", coherencePkg, code, err, stderr)
	}

	funcProfile := filepath.Join(tmp, "coverage.func.txt")
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", mod, "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover exited %d: %v\nstderr:\n%s", code, err, stderr)
	}
	if werr := os.WriteFile(funcProfile, []byte(out), 0o644); werr != nil {
		t.Fatalf("write func profile: %v", werr)
	}

	pkgDir := filepath.Join(mod, "internal", "coherence")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", mod, "run", "./cmd/apicover", "-enforce", "-cover", funcProfile, pkgDir)
	if code != 0 {
		t.Errorf("RED: the public-API gate reds on %s (exit %d, %v) — an added export is uncovered or named-but-never-executed:\n%s\n%s",
			coherencePkg, code, err, stdout, stderr)
	}
}
