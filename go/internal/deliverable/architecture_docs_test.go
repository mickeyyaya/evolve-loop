package deliverable

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// archFloorBuildReport passes every other build contract check, so any !OK below is the docs floor's.
const archFloorBuildReport = "# Build Report\n\n## Changes\n- go/internal/policy/policy.go\n\nVerdict: PASS\n"

// adrPath is a changed-path set element that satisfies the floor.
const adrPath = "docs/architecture/adr/0077-fleet-landing-prefix-queue.md"

func TestArchitectureDocs_CodeConstantIsStable(t *testing.T) {
	if CodeMissingArchitectureDocs != "missing_architecture_docs" {
		t.Errorf("CodeMissingArchitectureDocs = %q, want %q", CodeMissingArchitectureDocs, "missing_architecture_docs")
	}
}

func TestArchitectureDocsViolations_ArchitectureClassWithoutDocs(t *testing.T) {
	cases := []struct {
		name    string
		changed []string
	}{
		{
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
			if !strings.Contains(vs[0].Message, "docs/architecture") {
				t.Errorf("message must name the docs root to be actionable; got %q", vs[0].Message)
			}
		})
	}
}

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

func TestArchitectureDocs_BackfillInstance_FleetLanding(t *testing.T) {
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

func TestVerify_BuildWithoutDiff_ByteIdentical(t *testing.T) {
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
