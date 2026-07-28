//go:build acs

// Package cycle1149 materialises the cycle-1149 acceptance criteria for the one
// fleet-scoped task triage committed to THIS cycle:
//
//   - artifact-name-ssot-remaining-callsites → route the 7 remaining
//     "build-report.md" / "audit-report.md" string literals through
//     phasecontract.ArtifactFilename, the declared artifact-name SSOT.
//
// The deferred id (artifact-name-ssot-lint-guard) and the dropped one
// (manifest.go's "test-report.md") carry ZERO gating predicates for their own
// behavior (R9.3: predicates bind only to triage-committed work; a predicate
// gating deferred work starves the committed task — the cycle-280 failure
// mode). 005 is the exception that proves the rule: it pins the DROPPED half
// as a scope BOUNDARY the committed refactor must not cross, not as work.
//
// Predicate strategy. This is a behavior-preserving duplication-removal
// refactor: every literal resolves to exactly the string the registry already
// returns, so no runtime observation can distinguish "seven copies" from "one
// shared source" today. The predicates therefore split along the only axes
// that can actually observe the defect and its regressions:
//
//   - 001 is the CRUX and the only one that red-fails today: an absence check
//     over the production tree (the sanctioned form per go/acs/README.md
//     "Absence checks"), asserting the quoted literals survive in exactly ONE
//     declaration site — the registry.
//   - 002 is 001's ANTI-GAMING twin. The cheapest way to green an absence
//     check is to delete or rename the thing being counted, so 002 exercises
//     ArtifactFilename/ArtifactName directly and pins the runtime-truth values
//     plus both fallback edges (unregistered phase, NoArtifact ship).
//   - 003 and 004 are BEHAVIORAL regression guards over two of the seven
//     rewritten sites (core.RemovalClaimFailures, coherence.ReadCycleVerdicts),
//     each driven through its exported entry point with a positive case and a
//     wrong-filename NEGATIVE control that proves the positive is
//     filename-sensitive rather than trivially true. Pre-existing GREEN: they
//     stay green only if the substitution preserves the resolved value.
//   - 005 bounds the fix from the other side — the "test-report.md" half of
//     ship's manifest has no registry phase and must survive untouched.
//   - 006 pins the whole-tree compile, which is what proves the new
//     consensusdispatch → phasecontract import introduced no cycle.
//
// Roots: production source is read under acsassert.RepoRoot (the worktree —
// where Builder's change lands and is committed), never main.
package cycle1149

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// reportLiterals are the quoted forms of the two spine artifact filenames the
// registry owns. The QUOTED form is deliberate: prose comments that mention
// build-report.md are documentation, not a competing declaration, and must not
// trip the check.
var reportLiterals = []string{`"build-report.md"`, `"audit-report.md"`}

// registryDeclSite is the ONE file allowed to declare these literals — the
// phasecontract registry, which is the SSOT by construction.
const registryDeclSite = "internal/phasecontract/contract_registry.go"

// TestC1149_001_ReportFilenameLiteralsDeclaredOnlyInRegistry is the crux
// predicate for artifact-name-ssot-remaining-callsites: after the substitution
// no production (non-test) Go file under go/internal may declare the build or
// audit artifact filename as its own string literal. Every consumer must
// resolve it from phasecontract.ArtifactFilename.
//
// RED today: phases/build, phases/audit, core/phase_bindings,
// core/build_removal_check, coherence, consensusdispatch and phases/ship each
// carry an independent copy.
func TestC1149_001_ReportFilenameLiteralsDeclaredOnlyInRegistry(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	internalDir := filepath.Join(goDir, "internal")

	var offenders []string
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(goDir, path)
		if relErr != nil {
			return relErr
		}
		slashRel := filepath.ToSlash(rel)
		if slashRel == registryDeclSite {
			return nil // the SSOT declaration itself
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, lit := range reportLiterals {
			if strings.Contains(string(body), lit) {
				offenders = append(offenders, slashRel+" ("+lit+")")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", internalDir, err)
	}

	if len(offenders) > 0 {
		t.Errorf("the spine artifact filenames are independently declared at %d production site(s) outside the phasecontract SSOT: %s — route each through phasecontract.ArtifactFilename(<phase>)",
			len(offenders), strings.Join(offenders, ", "))
	}
}

// TestC1149_002_ArtifactFilenameIsTheRuntimeTruthSSOT is the anti-gaming twin
// of 001: it exercises the SSOT functions themselves so the absence check
// cannot be greened by deleting or renaming the surviving declaration. It also
// pins both fallback edges the seven call sites depend on — an unregistered
// phase and the NoArtifact ship phase — because ArtifactFilename (not
// ArtifactName) is the correct call only if those keep returning a usable name.
func TestC1149_002_ArtifactFilenameIsTheRuntimeTruthSSOT(t *testing.T) {
	for phase, want := range map[string]string{"build": "build-report.md", "audit": "audit-report.md"} {
		if got := phasecontract.ArtifactFilename(phase); got != want {
			t.Errorf("phasecontract.ArtifactFilename(%q) = %q, want %q (the filename the %s phase actually writes)", phase, got, want, phase)
		}
		if got := phasecontract.ArtifactName(phase); got != want {
			t.Errorf("phasecontract.ArtifactName(%q) = %q, want %q — the registry entry the callers resolve against was renamed or dropped", phase, got, want)
		}
	}

	// NoArtifact edge: ship has a registry entry but no deliverable, so
	// ArtifactName is empty while ArtifactFilename still yields the convention.
	if got := phasecontract.ArtifactName("ship"); got != "" {
		t.Errorf("phasecontract.ArtifactName(\"ship\") = %q, want \"\" — ship is NoArtifact and the empty return carries that distinction", got)
	}
	if got, want := phasecontract.ArtifactFilename("ship"), "ship-report.md"; got != want {
		t.Errorf("phasecontract.ArtifactFilename(\"ship\") = %q, want %q — the conventional fallback the call sites rely on", got, want)
	}

	// Unregistered edge: an inserted/user phase falls back to the convention.
	const unregistered = "c1149-not-a-registered-phase"
	if got, want := phasecontract.ArtifactFilename(unregistered), unregistered+"-report.md"; got != want {
		t.Errorf("phasecontract.ArtifactFilename(%q) = %q, want %q — the <phase>-report.md fallback is gone", unregistered, got, want)
	}
}

// TestC1149_003_RemovalClaimGateReadsRegistryNamedBuildReport is the behavioral
// guard over core/build_removal_check.go:71, one of the seven rewritten sites.
// It drives the exported floor check end-to-end (real report bytes, real
// worktree, real os.Stat) in three directions:
//
//	positive — report at phasecontract.ArtifactFilename("build") claiming a path
//	           that STILL exists → one failure line naming that path.
//	fallback — the same report under deliverables/ (the correction-ladder copy)
//	           → the same failure, so the substitution must keep BOTH paths.
//	negative — the identical bytes under any other filename → no failures at
//	           all, which is what proves the positive case is filename-sensitive
//	           rather than trivially true.
func TestC1149_003_RemovalClaimGateReadsRegistryNamedBuildReport(t *testing.T) {
	const claimed = "c1149-still-here.txt"
	report := "# Build Report\n\n```json\n{\"removedPaths\": [\"" + claimed + "\"]}\n```\n"

	check := func(t *testing.T, rel string) []string {
		t.Helper()
		workspace := t.TempDir()
		worktree := t.TempDir()
		// The claimed-removed file is still present: an honest gate must object.
		if err := os.WriteFile(filepath.Join(worktree, claimed), []byte("still here\n"), 0o644); err != nil {
			t.Fatalf("seeding worktree file: %v", err)
		}
		dest := filepath.Join(workspace, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, []byte(report), 0o644); err != nil {
			t.Fatalf("writing %s: %v", dest, err)
		}
		return core.RemovalClaimFailures(context.Background(), core.ReviewInput{
			Phase:     string(core.PhaseBuild),
			Workspace: workspace,
			Worktree:  worktree,
		})
	}

	canonical := phasecontract.ArtifactFilename("build")
	for _, rel := range []string{canonical, filepath.Join("deliverables", canonical)} {
		failures := check(t, rel)
		if len(failures) != 1 {
			t.Errorf("core.RemovalClaimFailures with the build report at %q returned %d failure(s), want 1 — the gate did not read the registry-named artifact: %v", rel, len(failures), failures)
			continue
		}
		if !strings.Contains(failures[0], claimed) {
			t.Errorf("failure line for %q does not name the falsely-claimed path %q: %q", rel, claimed, failures[0])
		}
	}

	// Negative / sensitivity control: any other filename must NOT be picked up.
	if failures := check(t, "build-report-c1149-not-the-contract.md"); len(failures) != 0 {
		t.Errorf("core.RemovalClaimFailures reported %v for a report that is NOT at the contracted artifact name — the gate is matching something other than the registry filename", failures)
	}
}

// TestC1149_004_CoherenceReadsRegistryNamedAuditReport is the behavioral guard
// over coherence.go:127. ReadCycleVerdicts is the reader the verdict-coherence
// halt decision depends on, so a filename drift there silently turns every
// cycle into "audit never ran". Positive case uses the canonical sentinel
// renderer (producer and parser in lockstep); the wrong-name negative is the
// control that proves filename sensitivity.
func TestC1149_004_CoherenceReadsRegistryNamedAuditReport(t *testing.T) {
	body := "# Audit Report\n\n" + phasecontract.RenderVerdictSentinel("audit", "PASS") + "\n"

	read := func(t *testing.T, filename string) (string, bool) {
		t.Helper()
		workspace := t.TempDir()
		if err := os.WriteFile(filepath.Join(workspace, filename), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", filename, err)
		}
		audit, _, auditRan := coherence.ReadCycleVerdicts(workspace)
		return audit, auditRan
	}

	canonical := phasecontract.ArtifactFilename("audit")
	audit, auditRan := read(t, canonical)
	if !auditRan {
		t.Errorf("coherence.ReadCycleVerdicts reported auditRan=false with the audit report at %q (the registry ArtifactFilename) — the reader did not open the registry-named artifact", canonical)
	}
	if audit != "PASS" {
		t.Errorf("coherence.ReadCycleVerdicts read audit verdict %q from %q, want \"PASS\"", audit, canonical)
	}

	// Negative / sensitivity control.
	if _, otherRan := read(t, "audit-report-c1149-not-the-contract.md"); otherRan {
		t.Errorf("coherence.ReadCycleVerdicts reported auditRan=true for a report that is NOT at the contracted artifact name — the reader is matching something other than the registry filename")
	}
}

// TestC1149_005_ShipManifestCoversTheTDDReport bounds the fix from the other
// side: the SSOT sweep must not SHRINK ship's manifestReportFiles by dropping
// an entry it could not resolve.
//
// SUPERSEDED PREMISE (corrected in cycle-1152). This predicate originally
// asserted that the "test-report.md" entry must survive as a LITERAL, on the
// stated ground that "'test' has no phasecontract entry". That premise rested
// on a wrong registry key: the phase is named "tdd", not "test", and
// contract_registry.go:132 registers it with ArtifactName "test-report.md".
// ArtifactName("test") returns "" because "test" is not a phase at all — not
// because the name lacks an SSOT. The entry is therefore fully SSOT-able and
// cycle-1152 migrates it.
//
// What actually needs bounding is COVERAGE, not the literal: whatever ship's
// manifest names for the TDD phase must equal the registry's name for it.
func TestC1149_005_ShipManifestCoversTheTDDReport(t *testing.T) {
	tddReport := phasecontract.ArtifactName(string(core.PhaseTDD))
	if tddReport == "" {
		t.Fatalf("premise broken: phasecontract.ArtifactName(%q) = \"\" — the TDD phase lost its registry contract, so ship's manifest has nothing to resolve against", string(core.PhaseTDD))
	}

	manifest := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "phases", "ship", "manifest.go")
	src, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("read %s: %v", manifest, err)
	}
	// Coverage survives either as the literal (pre-cycle-1152) or, once
	// migrated, as a phasecontract call — but it must not simply vanish.
	if !strings.Contains(string(src), `"`+tddReport+`"`) &&
		!strings.Contains(string(src), "phasecontract.ArtifactName(") &&
		!strings.Contains(string(src), "phasecontract.ArtifactFilename(") {
		t.Errorf("%s names neither %q nor a phasecontract resolver — the TDD half of "+
			"manifestReportFiles was dropped by the SSOT sweep, shrinking ship's manifest coverage",
			manifest, tddReport)
	}
}

// TestC1149_006_ProductionTreeCompilesAfterSubstitution pins the whole-module
// compile. This is the predicate that actually proves the new
// consensusdispatch → phasecontract edge introduced no import cycle: an import
// cycle is a compile error, and a per-package test run can hide it.
func TestC1149_006_ProductionTreeCompilesAfterSubstitution(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "build", "-C", goDir, "./...")
	if err != nil || code != 0 {
		t.Errorf("`go build ./...` in %s failed (code=%d err=%v):\n%s", goDir, code, err, stderr)
	}
}
