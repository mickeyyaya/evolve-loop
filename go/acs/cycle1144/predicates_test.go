//go:build acs

// Package cycle1144 materialises the cycle-1144 acceptance criteria for this
// lane's two triage-committed top_n tasks:
//
//	docs-floor-architecture-class-gate   (M) — deterministic architecture-class
//	    classifier + a new `missing_architecture_docs` violation wired into the
//	    ADR-0034 `deliverable.Verify` seam, so the `evolve phase verify`
//	    self-check and the host-side reviewer gate enforce the docs floor with
//	    byte-identical logic.
//	docs-floor-backfill-fleet-landing    (S) — document `fleet.landing`
//	    (go/internal/policy/policy.go:1043-1208, values "per-lane" /
//	    "prefix-queue") in control-flags.md, runtime-reference.md and a
//	    standalone ADR: the first enforced instance of that floor, and the live
//	    proof the gap was real.
//
// Predicate strategy. Task 1's predicates are behavioural-via-subprocess (the
// cycle-549…1108 precedent): each shells `go test -run` over the RED contract
// tests authored this cycle in go/internal/deliverable/architecture_docs_test.go,
// every one of which CALLS ArchitectureDocsViolations / VerifyBuildWithChangedPaths
// and asserts on returned values — none is a source-grep of production code (the
// cycle-85 degenerate-predicate ban). Task 2 is a DOCUMENTATION deliverable: the
// doc file IS the artifact under test, so asserting on its emitted content is a
// real-artifact assertion, not a source-grep proxy. Docs are read under
// acsassert.RepoRoot (the WORKTREE — the dual-root pattern, go/acs/README.md),
// because a doc edited this cycle reaches main only at ship.
//
// RED now: ArchitectureDocsViolations / VerifyBuildWithChangedPaths /
// CodeMissingArchitectureDocs do not exist, so the deliverable package fails to
// compile; and `fleet.landing` appears in no doc.
package cycle1144

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const deliverablePkg = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure in
// the target package (the RED signal before Builder implements) surfaces as a
// non-zero exit. code < 0 is a genuine launch failure (toolchain missing /
// killed by signal), not a test verdict, so it is a hard predicate error rather
// than a silent RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// ---------------------------------------------------------------------------
// Task 1 — docs-floor-architecture-class-gate
// ---------------------------------------------------------------------------

// TestC1144_001_ArchitectureClassDiffWithoutDocsIsAViolation — AC2. The
// classifier must flag an architecture-class diff (policy vocabulary key, new
// internal package, new phase spec, trust-kernel surface) that carries no
// documentation delta, with the stable `missing_architecture_docs` code and an
// actionable message naming docs/architecture. Exercised through the real
// function, over four distinct architecture shapes.
func TestC1144_001_ArchitectureClassDiffWithoutDocsIsAViolation(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg,
		"TestArchitectureDocsViolations_ArchitectureClassWithoutDocs|TestArchitectureDocs_CodeConstantIsStable")
	if !ok {
		t.Errorf("architecture-class-without-docs classification is not implemented (or regressed):\n%s", out)
	}
}

// TestC1144_002_DocsDeltaSatisfiesTheFloor — AC3. The SAME architecture-class
// diff must pass once a docs delta rides along (a new ADR, a control-flags.md
// entry, or a runtime-reference.md note). This is the anti-false-positive half:
// a floor that rejects correctly-documented work would block every architecture
// cycle.
func TestC1144_002_DocsDeltaSatisfiesTheFloor(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestArchitectureDocsViolations_DocsDeltaSatisfiesFloor")
	if !ok {
		t.Errorf("a documented architecture-class diff must pass the floor:\n%s", out)
	}
}

// TestC1144_003_NonArchitectureDiffsFailOpen — AC4, the NEGATIVE / edge axis and
// the regression guard for every ordinary cycle. Empty diff, test-only,
// docs-only and workspace-artifact-only diffs must never yield the violation:
// the existing fail-open contract in deliverable.go's package doc governs them.
func TestC1144_003_NonArchitectureDiffsFailOpen(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg,
		"TestArchitectureDocsViolations_NonArchitectureDiffsFailOpen|TestVerify_BuildWithoutDiff_ByteIdentical")
	if !ok {
		t.Errorf("non-architecture diffs must be unaffected by the docs floor:\n%s", out)
	}
}

// TestC1144_004_FloorIsWiredIntoTheVerifySeam — AC5, the WIRING proof. A
// classifier nobody calls is a no-op: the verdict must surface as a Result from
// the ADR-0034 verify seam (!OK, correct Phase, floor code present) on an
// otherwise perfectly well-formed build-report, and must be ADDITIVE — the
// pre-existing well-formedness violations still surface alongside it.
func TestC1144_004_FloorIsWiredIntoTheVerifySeam(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestVerify_ArchitectureClassRequiresDocsDelta")
	if !ok {
		t.Errorf("the docs floor is not wired into deliverable.Verify's seam:\n%s", out)
	}
}

// TestC1144_005_DeliverablePackageStaysGreen — no-regression. The floor is
// additive to a gate that already carries the challenge-token, stray-in-worktree,
// failure-context and verdict checks; the whole package must stay green, not
// just the new tests.
func TestC1144_005_DeliverablePackageStaysGreen(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", deliverablePkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", deliverablePkg, code, err, out)
	}
	if code != 0 {
		t.Errorf("internal/deliverable must stay green with the docs floor added:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Task 2 — docs-floor-backfill-fleet-landing
// ---------------------------------------------------------------------------

// readDoc returns a worktree-relative doc's contents, or fails the predicate
// when it is absent. Paths resolve under acsassert.RepoRoot (the worktree), per
// the dual-root pattern: a doc edited this cycle exists on main only after ship.
func readDoc(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Errorf("cannot read %s: %v", rel, err)
		return ""
	}
	return string(b)
}

// TestC1144_006_FleetLandingDocumentedInControlFlags — AC (Task 2). The
// `fleet.landing` policy key must appear in the control-flags reference together
// with BOTH values of its vocabulary, so an operator reading the reference can
// actually set it. Naming the key alone would be a token the builder could drop
// in without documenting anything.
func TestC1144_006_FleetLandingDocumentedInControlFlags(t *testing.T) {
	const rel = "docs/architecture/control-flags.md"
	doc := readDoc(t, rel)
	for _, needle := range []string{"fleet.landing", "per-lane", "prefix-queue"} {
		if !strings.Contains(doc, needle) {
			t.Errorf("%s does not document %q — fleet.landing has real resolved-config semantics (policy.go:1043-1208) and zero doc coverage", rel, needle)
		}
	}
}

// TestC1144_007_FleetLandingDocumentedInRuntimeReference — AC (Task 2). The
// operator-visible half: runtime-reference.md must say when to choose
// prefix-queue over per-lane, not merely that the key exists.
func TestC1144_007_FleetLandingDocumentedInRuntimeReference(t *testing.T) {
	const rel = "docs/operations/runtime-reference.md"
	doc := readDoc(t, rel)
	for _, needle := range []string{"fleet.landing", "prefix-queue"} {
		if !strings.Contains(doc, needle) {
			t.Errorf("%s does not carry the operator-visible %q note", rel, needle)
		}
	}
}

// TestC1144_008_FleetLandingADRExists — AC (Task 2). A standalone ADR under
// docs/architecture/adr/ must explain the prefix-queue single-writer composer.
// Discovered by scanning the ADR directory rather than pinning a number, so the
// predicate does not dictate the ADR's index.
func TestC1144_008_FleetLandingADRExists(t *testing.T) {
	adrDir := filepath.Join(acsassert.RepoRoot(t), "docs/architecture/adr")
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", adrDir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(adrDir, e.Name()))
		if err != nil {
			continue
		}
		body := string(b)
		if strings.Contains(body, "fleet.landing") && strings.Contains(body, "prefix-queue") {
			return
		}
	}
	t.Errorf("no ADR under docs/architecture/adr/ documents fleet.landing / prefix-queue")
}

// TestC1144_009_DocsFloorFlagsItsOwnBackfillInstance — the closing loop across
// both tasks, and the strongest anti-no-op signal in this suite. The classifier
// from Task 1, run over a diff shaped exactly like the historical
// `fleet.landing` landing (policy.go touched, zero docs), must flag it —
// proving the gate would have caught the very instance Task 2 is backfilling —
// and must pass the same landing done right. Driven through the real exported
// function by the contract test, never a source grep. (An in-package test, not
// a standalone probe program: internal/… is import-closed to synthetic
// `command-line-arguments` packages, so `go run` cannot reach it.)
func TestC1144_009_DocsFloorFlagsItsOwnBackfillInstance(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestArchitectureDocs_BackfillInstance_FleetLanding")
	if !ok {
		t.Errorf("the docs floor does not flag its own fleet.landing backfill instance:\n%s", out)
	}
}
