package docsfloor_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/docsfloor"
)

// TestEvaluateDecisionTable pins the full decision surface: the crux WARN, the
// documented PASS, and every SKIP axis (stage off, unlabeled, empty diff).
func TestEvaluateDecisionTable(t *testing.T) {
	cases := []struct {
		name string
		cfg  docsfloor.Config
		in   docsfloor.Input
		want string
	}{
		{
			name: "architecture change with no docs warns",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in:   docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{"go/internal/core/cyclerun.go"}},
			want: docsfloor.StatusWarn,
		},
		{
			name: "empty stage defaults to evaluating",
			cfg:  docsfloor.Config{},
			in:   docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{"go/internal/policy/policy.go"}},
			want: docsfloor.StatusWarn,
		},
		{
			name: "documented architecture change passes",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in: docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{
				"go/internal/core/cyclerun.go", "docs/architecture/adr/0077-docs-floor.md"}},
			want: docsfloor.StatusPass,
		},
		{
			name: "leading ./ still counts as a docs touch",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in: docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{
				"./docs/operations/operating-policy.md"}},
			want: docsfloor.StatusPass,
		},
		{
			name: "non-architecture change is skipped",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in:   docsfloor.Input{ArchitectureLabeled: false, ChangedFiles: []string{"go/internal/gc/discover.go"}},
			want: docsfloor.StatusSkip,
		},
		{
			name: "stage off never fires",
			cfg:  docsfloor.Config{Stage: "OFF"},
			in:   docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{"go/internal/core/cyclerun.go"}},
			want: docsfloor.StatusSkip,
		},
		{
			name: "shadow stage still evaluates",
			cfg:  docsfloor.Config{Stage: "shadow"},
			in:   docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{"go/internal/core/cyclerun.go"}},
			want: docsfloor.StatusWarn,
		},
		{
			name: "empty change set is not judged",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in:   docsfloor.Input{ArchitectureLabeled: true},
			want: docsfloor.StatusSkip,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := docsfloor.Evaluate(tc.cfg, tc.in)
			if got.Status != tc.want {
				t.Errorf("status = %q, want %q (reason %q)", got.Status, tc.want, got.Reason)
			}
			if strings.TrimSpace(got.Reason) == "" {
				t.Errorf("verdict %q carries an empty Reason", got.Status)
			}
		})
	}
}

// TestEvaluateWarnReasonNamesTheRule keeps the warning actionable: it must say
// what is missing and cite the policy clause, not just fire.
func TestEvaluateWarnReasonNamesTheRule(t *testing.T) {
	v := docsfloor.Evaluate(
		docsfloor.Config{Stage: "enforce"},
		docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{"go/internal/phasecontract/contract_registry.go"}},
	)
	if v.Status != docsfloor.StatusWarn {
		t.Fatalf("status = %q, want %q", v.Status, docsfloor.StatusWarn)
	}
	for _, want := range []string{"docs/", "operating-policy"} {
		if !strings.Contains(v.Reason, want) {
			t.Errorf("WARN reason %q does not mention %q", v.Reason, want)
		}
	}
}

// TestLabelArchitecture covers both axes of the label: every declared surface
// labels, and an ordinary leaf-package change does not (otherwise the floor
// would warn on every build and become noise).
func TestLabelArchitecture(t *testing.T) {
	labeled := []string{
		"go/internal/core/orchestrator.go",
		"go/internal/policy/policy.go",
		"go/internal/config/config.go",
		"go/internal/phasecontract/contract_registry.go",
		"go/internal/router/digest.go",
		"docs/architecture/phase-registry.json",
	}
	for _, f := range labeled {
		if !docsfloor.LabelArchitecture([]string{f}) {
			t.Errorf("LabelArchitecture(%q) = false, want true — architecture surface not labeled", f)
		}
	}
	unlabeled := []string{
		"go/internal/gc/discover.go",
		"go/internal/dossier/build.go",
		"README.md",
		"skills/loop/SKILL.md",
	}
	if docsfloor.LabelArchitecture(unlabeled) {
		t.Errorf("LabelArchitecture(%v) = true, want false — non-architecture change over-labeled", unlabeled)
	}
	if docsfloor.LabelArchitecture(nil) {
		t.Error("LabelArchitecture(nil) = true, want false")
	}
	// A mixed set is labeled by its single architecture member.
	if !docsfloor.LabelArchitecture(append(append([]string{}, unlabeled...), "./go/internal/core/cyclerun.go")) {
		t.Error("mixed change set containing an architecture surface was not labeled")
	}
}

// TestVerdictIsAValue guards the gate's shape: Verdict is a plain value type
// callers can construct and compare, so a caller can assert against an expected
// verdict rather than reaching into the package.
func TestVerdictIsAValue(t *testing.T) {
	want := docsfloor.Verdict{Status: docsfloor.StatusSkip, Reason: "docs floor stage is off"}
	got := docsfloor.Evaluate(docsfloor.Config{Stage: "off"}, docsfloor.Input{ArchitectureLabeled: true})
	if got != want {
		t.Errorf("Evaluate = %+v, want %+v", got, want)
	}
}

// TestIsArchitectureClass covers the blocking-grade classifier's three
// divergences from LabelArchitecture — test-file exclusion, the new-package
// namesake proxy, and phase specs — plus the fail-open default that keeps
// ordinary cycles untouched.
func TestIsArchitectureClass(t *testing.T) {
	class := [][]string{
		{"go/internal/policy/policy.go", "go/internal/policy/policy_test.go"},
		{"go/internal/config/config.go"},
		{"go/internal/docsfloor/docsfloor.go"}, // new package via its namesake file
		{"go/internal/fleet/prefixqueue.go"},   // trust-kernel surface
		{"go/internal/phases/ship/landprefixes.go"},
		{"phases/specs/docs-floor.json"},
	}
	for _, c := range class {
		if !docsfloor.IsArchitectureClass(c) {
			t.Errorf("IsArchitectureClass(%v) = false, want true", c)
		}
	}
	open := [][]string{
		nil,
		{"go/internal/policy/policy_test.go"}, // test-only: documents nothing
		{"go/internal/deliverable/reviewer_test.go"},
		{"go/internal/dossier/build.go"}, // leaf package, not its namesake
		{"go/internal/gc/discover.go"},
		{"docs/operations/release-notes/index.md"},
		{".evolve/runs/cycle-1144/build-report.md"},
	}
	for _, c := range open {
		if docsfloor.IsArchitectureClass(c) {
			t.Errorf("IsArchitectureClass(%v) = true, want false — must fail open", c)
		}
	}
}

// TestHasDocsDelta pins what satisfies the floor: the architecture docs tree
// (DocsRoots[0]) or the operator runtime reference (DocsRoots[1]) — and nothing
// else, so an incidental doc edit elsewhere cannot buy off an ADR.
func TestHasDocsDelta(t *testing.T) {
	if len(docsfloor.DocsRoots) != 2 {
		t.Fatalf("DocsRoots = %v, want the architecture tree + runtime reference", docsfloor.DocsRoots)
	}
	satisfying := []string{
		"docs/architecture/adr/0078-fleet-landing-prefix-queue.md",
		"docs/architecture/control-flags.md",
		"docs/operations/runtime-reference.md",
	}
	for _, f := range satisfying {
		if !docsfloor.HasDocsDelta([]string{"go/internal/policy/policy.go", f}) {
			t.Errorf("HasDocsDelta with %q = false, want true", f)
		}
	}
	insufficient := []string{"README.md", "docs/operations/release-notes/index.md", "skills/loop/SKILL.md"}
	if docsfloor.HasDocsDelta(insufficient) {
		t.Errorf("HasDocsDelta(%v) = true, want false", insufficient)
	}
	if docsfloor.HasDocsDelta(nil) {
		t.Error("HasDocsDelta(nil) = true, want false")
	}
}
