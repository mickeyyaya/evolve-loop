//go:build acs

// Package cycle2 holds the (re-homed) ACS predicates for the Cycle Dossier feature
// (ADR-0055 slices D2–D4). Authored by TDD Engineer as the RED gate; Builder
// makes them GREEN by implementing dossier.Build/Render/Write, the failure-learning
// defects propagation, the evolve dossier CLI, and the docs patches.
package cycle6

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/dossier"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// ── D2: dossier-recorder-d2 ────────────────────────────────────────────────

// TestC6_001_DossierBuildReturnsPopulatedDossier verifies dossier.Build
// returns a non-nil Dossier with Cycle+Goal set from BuildOpts (D2-AC1).
func TestC6_001_DossierBuildReturnsPopulatedDossier(t *testing.T) {
	d, err := dossier.Build(1, dossier.BuildOpts{WorkspacePath: t.TempDir(), Goal: "reduce flags"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Cycle != 1 {
		t.Errorf("Cycle: got %d, want 1", d.Cycle)
	}
	if d.Goal != "reduce flags" {
		t.Errorf("Goal: got %q, want %q", d.Goal, "reduce flags")
	}
	if len(d.Phases) == 0 {
		t.Error("Build: expected >=1 PhaseRecord")
	}
	if d.FinalVerdict == "" {
		t.Error("Build: FinalVerdict must not be empty")
	}
}

// TestC6_002_DossierBuildErrorOnBadCycle verifies Build returns an error for
// cycle <= 0 (D2-AC2, negative test).
func TestC6_002_DossierBuildErrorOnBadCycle(t *testing.T) {
	cases := []struct{ cycle int }{{0}, {-1}}
	for _, tc := range cases {
		_, err := dossier.Build(tc.cycle, dossier.BuildOpts{Goal: "x"})
		if err == nil {
			t.Errorf("Build(%d, ...): want error for invalid cycle, got nil", tc.cycle)
		}
	}
}

// TestC6_003_DossierRenderJSONRoundTrip verifies RenderJSON produces valid
// JSON that round-trips back to the original Cycle+Goal (D2-AC3).
func TestC6_003_DossierRenderJSONRoundTrip(t *testing.T) {
	d := &dossier.Dossier{
		Cycle:        3,
		Goal:         "render test",
		FinalVerdict: dossier.VerdictPass,
		Phases:       []dossier.PhaseRecord{{Name: "build", Verdict: dossier.VerdictPass}},
	}
	raw, err := dossier.RenderJSON(d)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var round dossier.Dossier
	if err := json.Unmarshal(raw, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round.Cycle != d.Cycle {
		t.Errorf("JSON round-trip Cycle: got %d, want %d", round.Cycle, d.Cycle)
	}
	if round.Goal != d.Goal {
		t.Errorf("JSON round-trip Goal: got %q, want %q", round.Goal, d.Goal)
	}
	md, err := dossier.RenderMarkdown(d)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if len(md) == 0 {
		t.Error("RenderMarkdown: empty output — must produce non-empty markdown")
	}
}

// TestC6_004_DossierWriteCreatesTwoFiles verifies Write creates cycle-N.json
// and cycle-N.md in the target directory (D2-AC4).
func TestC6_004_DossierWriteCreatesTwoFiles(t *testing.T) {
	d := &dossier.Dossier{
		Cycle:        42,
		Goal:         "write test",
		FinalVerdict: dossier.VerdictPass,
		Phases:       []dossier.PhaseRecord{{Name: "build", Verdict: dossier.VerdictPass}},
	}
	dir := t.TempDir()
	if err := dossier.Write(d, dir, false); err != nil {
		t.Fatalf("Write: %v", err)
	}
	for _, name := range []string{"cycle-42.json", "cycle-42.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("Write: expected %s to exist: %v", name, err)
		}
	}
}

// TestC6_005_DefectsProducedAsCarryoverTodos verifies that each entry in
// FailedRecord.Defects becomes its own CarryoverTodo entry in State (D2-AC5).
// Negative test: one generic todo is NOT sufficient — each defect must be
// individually represented.
func TestC6_005_DefectsProducedAsCarryoverTodos(t *testing.T) {
	defects := []string{
		"unbounded fan-out in auditor verify path",
		"nil pointer in router when signals absent",
	}
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:          1,
		Verdict:        "FAIL",
		Classification: "test-defects",
		Defects:        defects,
		Summary:        "two distinct defects",
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	if len(state.CarryoverTodos) < len(defects) {
		t.Errorf("want >= %d CarryoverTodos (one per defect), got %d; a single generic todo is insufficient",
			len(defects), len(state.CarryoverTodos))
	}
	for i, defect := range defects {
		found := false
		for _, todo := range state.CarryoverTodos {
			if strings.Contains(todo.Action, defect) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("defect[%d] %q has no corresponding CarryoverTodo", i, defect)
		}
	}
}

// ── D3: dossier-acs-d3 ────────────────────────────────────────────────────

// TestC6_006_PolicyHasDossierCloseout verifies .evolve/policy.json contains
// the "dossier-closeout" gate entry required by D3 (D3-AC5).
// acs-predicate: config-check — inherently a policy-file presence assertion.
func TestC6_006_PolicyHasDossierCloseout(t *testing.T) {
	root := acsassert.RepoRoot(t)
	policyPath := filepath.Join(root, ".evolve", "policy.json")
	if !acsassert.FileExists(t, policyPath) {
		t.Fatalf("RED: .evolve/policy.json missing")
	}
	if !acsassert.FileContains(t, policyPath, "dossier") {
		t.Errorf("RED: .evolve/policy.json has no 'dossier' entry — gate not enrolled")
	}
}

// TestC6_007_DossierCLISubcommandRegistered verifies the evolve binary
// exposes a "dossier" subcommand (D3-AC4 behavioral check — invokes the
// real binary and asserts on its help output).
func TestC6_007_DossierCLISubcommandRegistered(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bin := filepath.Join(root, "go", "bin", "evolve")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("evolve binary not found at %s — rebuild with 'cd go && make build': %v", bin, err)
	}
	out, _, _, _ := acsassert.SubprocessOutput(bin, "help")
	if !strings.Contains(out, "dossier") {
		t.Errorf("evolve help output does not mention 'dossier' — cmd_dossier.go not registered or binary stale; rebuild and retry")
	}
}

// ── D4: dossier-docs-d4 ──────────────────────────────────────────────────

// TestC6_008_ADRDossierFileExistsAndHasSections verifies the ADR file exists,
// is > 1000 bytes, and contains all four required ADR sections (D4-AC1).
// acs-predicate: config-check — inherently a documentation presence assertion.
func TestC6_008_ADRDossierFileExistsAndHasSections(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0055-cycle-dossier.md")
	if !acsassert.FileExists(t, adrPath) {
		t.Fatalf("RED: %s missing — ADR not written", adrPath)
	}
	info, err := os.Stat(adrPath)
	if err != nil {
		t.Fatalf("stat ADR: %v", err)
	}
	if info.Size() < 1000 {
		t.Errorf("ADR file is %d bytes, want >= 1000", info.Size())
	}
	for _, section := range []string{"## Status", "## Context", "## Decision", "## Consequences"} {
		if !acsassert.FileContains(t, adrPath, section) {
			t.Errorf("ADR missing required section %q", section)
		}
	}
}

// TestC6_009_ScoutMdHasKBRecallStep verifies agents/evolve-scout.md contains
// a knowledge-base/cycles recall step (D4-AC2).
// acs-predicate: config-check — doc-content presence assertion.
func TestC6_009_ScoutMdHasKBRecallStep(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scoutPath := filepath.Join(root, "agents", "evolve-scout.md")
	if !acsassert.FileExists(t, scoutPath) {
		t.Fatalf("agents/evolve-scout.md missing")
	}
	if !acsassert.FileContains(t, scoutPath, "knowledge-base/cycles") {
		t.Errorf("RED: agents/evolve-scout.md has no 'knowledge-base/cycles' recall step — D4 patch not applied")
	}
}

// TestC6_010_RuntimeReferenceHasDossierVerify verifies runtime-reference.md
// documents the evolve dossier verify command (D4-AC3).
// acs-predicate: config-check — doc-content presence assertion.
func TestC6_010_RuntimeReferenceHasDossierVerify(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rtPath := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	if !acsassert.FileExists(t, rtPath) {
		t.Fatalf("docs/operations/runtime-reference.md missing")
	}
	if !acsassert.FileContains(t, rtPath, "dossier verify") {
		t.Errorf("RED: runtime-reference.md has no 'dossier verify' entry — D4 doc patch not applied")
	}
}

// TestC6_011_ScoutMdRetainsFiveSectionHeaders verifies agents/evolve-scout.md
// retains >= 5 '## ' section headers after the D4 patch (regression guard,
// D4-AC4 negative test — a destructive patch that removes headers fails here).
func TestC6_011_ScoutMdRetainsFiveSectionHeaders(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scoutPath := filepath.Join(root, "agents", "evolve-scout.md")
	raw, err := os.ReadFile(scoutPath)
	if err != nil {
		t.Fatalf("read agents/evolve-scout.md: %v", err)
	}
	count := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "## ") {
			count++
		}
	}
	if count < 5 {
		t.Errorf("agents/evolve-scout.md has %d '## ' section headers, want >= 5 — D4 patch must preserve file structure", count)
	}
}
