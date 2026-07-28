package deliverable

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Cycle-1144 RED contract for the docs floor on architecture-class changes
// (inbox item cycle-docs-floor-architecture-changes).
//
// The gap: a cycle can add a policy vocabulary key, a new internal package, or a
// new phase spec and ship with ZERO documentation delta — `fleet.landing`
// (policy.go:1043-1208) is the live proof: real resolved-config semantics,
// no entry in docs/architecture/control-flags.md or
// docs/operations/runtime-reference.md. `deliverable.Verify` is the ADR-0034
// seam where the `evolve phase verify` self-check and the host-side reviewer
// gate run byte-identical logic, so the floor belongs here — enforced once,
// for both callers.
//
// Contract under test (to be implemented by Builder):
//
//	const CodeMissingArchitectureDocs = "missing_architecture_docs"
//	func ArchitectureDocsViolations(changed []string) []Violation
//	func VerifyBuildWithChangedPaths(roots phasecontract.Roots, changed []string) (Result, error)
//
// ArchitectureDocsViolations is a PURE classifier over the build diff's changed
// path set (deterministic — no LLM judgement, no git shell-out), returning one
// CodeMissingArchitectureDocs violation iff the set is architecture-class AND
// carries no docs delta. VerifyBuildWithChangedPaths is the wiring: the same
// well-formedness result Verify("build", …) produces, with the docs-floor
// violations appended and OK recomputed. Fail-open is the default — a diff that
// is not architecture-class never yields the violation, so pure bugfix, test
// and docs cycles are byte-identical to today.

// archFloorBuildReport is the minimum build-report.md that passes every
// existing build contract check, so any !OK in the wiring tests below is
// attributable to the docs floor alone.
const archFloorBuildReport = "# Build Report\n\n## Changes\n- go/internal/policy/policy.go\n\nVerdict: PASS\n"

// adrPath is a changed-path set element that satisfies the floor.
const adrPath = "docs/architecture/adr/0077-fleet-landing-prefix-queue.md"

// --- AC1: the violation code exists and is stable -------------------------

func TestArchitectureDocs_CodeConstantIsStable(t *testing.T) {
	// The code is consumed by the CLI, the reviewer gate and the auditor
	// checklist — it is closed vocabulary, spelled snake_case like its
	// siblings in this package.
	if CodeMissingArchitectureDocs != "missing_architecture_docs" {
		t.Errorf("CodeMissingArchitectureDocs = %q, want %q", CodeMissingArchitectureDocs, "missing_architecture_docs")
	}
}

// --- AC2: architecture-class diff with no docs delta is a violation -------

func TestArchitectureDocsViolations_ArchitectureClassWithoutDocs(t *testing.T) {
	cases := []struct {
		name    string
		changed []string
	}{
		{
			// The backfill instance itself: a policy vocabulary key added with
			// no doc anywhere. This is exactly the `fleet.landing` shape.
			name:    "policy vocabulary key",
			changed: []string{"go/internal/policy/policy.go", "go/internal/policy/policy_test.go"},
		},
		{
			name:    "new internal package",
			changed: []string{"go/internal/docsfloor/docsfloor.go", "go/internal/docsfloor/docsfloor_test.go"},
		},
		{
			name:    "new phase spec",
			changed: []string{"phases/specs/docs-floor.json"},
		},
		{
			name:    "trust-kernel surface",
			changed: []string{"go/internal/core/routing_dispatch.go"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vs := ArchitectureDocsViolations(tc.changed)
			if len(vs) == 0 {
				t.Fatalf("architecture-class diff %v with no docs delta: want a violation, got none", tc.changed)
			}
			if vs[0].Code != CodeMissingArchitectureDocs {
				t.Errorf("code = %q, want %q", vs[0].Code, CodeMissingArchitectureDocs)
			}
			// The message is the correction directive re-dispatched verbatim to
			// the builder, so it must name where the docs belong — a bare
			// "missing docs" is not actionable.
			if !strings.Contains(vs[0].Message, "docs/architecture") {
				t.Errorf("message must name the docs root to be actionable; got %q", vs[0].Message)
			}
		})
	}
}

// --- AC3: the same diff WITH a docs delta passes --------------------------

func TestArchitectureDocsViolations_DocsDeltaSatisfiesFloor(t *testing.T) {
	cases := []struct {
		name    string
		changed []string
	}{
		{
			name:    "new ADR",
			changed: []string{"go/internal/policy/policy.go", adrPath},
		},
		{
			name:    "control-flags entry",
			changed: []string{"go/internal/policy/policy.go", "docs/architecture/control-flags.md"},
		},
		{
			name:    "operator runtime reference",
			changed: []string{"go/internal/policy/policy.go", "docs/operations/runtime-reference.md"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if vs := ArchitectureDocsViolations(tc.changed); len(vs) != 0 {
				t.Errorf("architecture-class diff WITH docs delta %v: want no violation, got %+v", tc.changed, vs)
			}
		})
	}
}

// --- AC4: fail-open for everything that is not architecture-class ---------
//
// This is the regression guard for every ordinary cycle. A bugfix, a test-only
// change, a docs-only change and an empty diff must all stay byte-identical to
// today's behaviour.

func TestArchitectureDocsViolations_NonArchitectureDiffsFailOpen(t *testing.T) {
	cases := []struct {
		name    string
		changed []string
	}{
		{name: "empty diff", changed: nil},
		{name: "test-only", changed: []string{"go/internal/deliverable/deliverable_test.go"}},
		{name: "policy test-only", changed: []string{"go/internal/policy/policy_test.go"}},
		{name: "docs-only", changed: []string{"docs/operations/release-notes/index.md"}},
		{name: "workspace artifacts", changed: []string{".evolve/runs/cycle-1144/build-report.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if vs := ArchitectureDocsViolations(tc.changed); len(vs) != 0 {
				t.Errorf("non-architecture diff %v must fail open; got %+v", tc.changed, vs)
			}
		})
	}
}

// --- AC5: WIRING PROOF — the classifier reaches a Verify result -----------
//
// A classifier nobody calls is a no-op. This is the gate-wiring proof: the
// verdict must surface through the ADR-0034 verify seam, on a build-report
// that is otherwise perfectly well-formed.

func TestVerify_ArchitectureClassRequiresDocsDelta(t *testing.T) {
	t.Run("architecture-class without docs fails CLOSED", func(t *testing.T) {
		ws := t.TempDir()
		writeFile(t, ws, "build-report.md", archFloorBuildReport)

		res, err := VerifyBuildWithChangedPaths(
			phasecontract.Roots{Workspace: ws},
			[]string{"go/internal/policy/policy.go"},
		)
		if err != nil {
			t.Fatalf("a confirmed docs-floor violation is not ambiguity; want err=nil, got %v", err)
		}
		if res.OK {
			t.Fatal("want !OK (fail CLOSED) for an architecture-class diff with no docs delta")
		}
		if !hasCode(res, CodeMissingArchitectureDocs) {
			t.Errorf("want a %s violation, got %+v", CodeMissingArchitectureDocs, res.Violations)
		}
		if res.Phase != "build" {
			t.Errorf("Phase = %q, want \"build\"", res.Phase)
		}
	})

	t.Run("architecture-class with docs passes", func(t *testing.T) {
		ws := t.TempDir()
		writeFile(t, ws, "build-report.md", archFloorBuildReport)

		res, err := VerifyBuildWithChangedPaths(
			phasecontract.Roots{Workspace: ws},
			[]string{"go/internal/policy/policy.go", adrPath},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.OK {
			t.Errorf("want OK when the docs delta is present, got %+v", res.Violations)
		}
	})

	t.Run("non-architecture diff is unaffected", func(t *testing.T) {
		ws := t.TempDir()
		writeFile(t, ws, "build-report.md", archFloorBuildReport)

		res, err := VerifyBuildWithChangedPaths(
			phasecontract.Roots{Workspace: ws},
			[]string{"go/internal/deliverable/deliverable_test.go"},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.OK {
			t.Errorf("non-architecture diff must be unaffected by the floor, got %+v", res.Violations)
		}
	})

	t.Run("well-formedness violations still surface alongside the floor", func(t *testing.T) {
		// The floor is ADDITIVE: it must not swallow or replace the existing
		// contract checks. A missing artifact plus an architecture-class diff
		// must report the missing artifact.
		ws := t.TempDir()
		res, err := VerifyBuildWithChangedPaths(
			phasecontract.Roots{Workspace: ws},
			[]string{"go/internal/policy/policy.go"},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.OK {
			t.Fatal("want !OK for a missing build-report")
		}
		if !hasCode(res, CodeMissingArtifact) {
			t.Errorf("want the pre-existing %s violation preserved, got %+v", CodeMissingArtifact, res.Violations)
		}
	})
}

// --- AC7: the floor flags its own backfill instance ----------------------

func TestArchitectureDocs_BackfillInstance_FleetLanding(t *testing.T) {
	// The closing loop, and the strongest anti-no-op signal for the pair of
	// tasks in this cycle: the classifier run over a diff shaped exactly like
	// the historical `fleet.landing` landing (policy vocabulary added, zero
	// docs) must flag it — proving the gate would have caught the very
	// instance Task 2 is backfilling. Done right (Task 2's own shape, docs
	// riding along) the same diff must pass.
	undocumented := ArchitectureDocsViolations([]string{"go/internal/policy/policy.go"})
	if len(undocumented) == 0 || undocumented[0].Code != CodeMissingArchitectureDocs {
		t.Errorf("the undocumented fleet.landing landing was NOT flagged: %+v", undocumented)
	}
	documented := ArchitectureDocsViolations([]string{
		"go/internal/policy/policy.go",
		"docs/architecture/control-flags.md",
		"docs/operations/runtime-reference.md",
	})
	if len(documented) != 0 {
		t.Errorf("the documented fleet.landing landing was wrongly flagged: %+v", documented)
	}
}

// --- AC6: the legacy entry point is untouched ----------------------------

func TestVerify_BuildWithoutDiff_ByteIdentical(t *testing.T) {
	// Verify("build", …) has no diff to inspect, so it must never emit the
	// docs-floor violation — callers with no diff source keep today's exact
	// behaviour (the fail-open half of the ADR-0034 contract).
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", archFloorBuildReport)

	res, err := Verify("build", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Errorf("Verify with no diff must stay OK, got %+v", res.Violations)
	}
	if hasCode(res, CodeMissingArchitectureDocs) {
		t.Error("Verify with no diff must not emit the docs-floor violation")
	}
}
