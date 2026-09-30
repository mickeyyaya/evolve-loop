//go:build acs

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

var reportLiterals = []string{`"build-report.md"`, `"audit-report.md"`}

const registryDeclSite = "internal/phasecontract/contract_registry.go"

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
			return nil
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

func TestC1149_002_ArtifactFilenameIsTheRuntimeTruthSSOT(t *testing.T) {
	for phase, want := range map[string]string{"build": "build-report.md", "audit": "audit-report.md"} {
		if got := phasecontract.ArtifactFilename(phase); got != want {
			t.Errorf("phasecontract.ArtifactFilename(%q) = %q, want %q (the filename the %s phase actually writes)", phase, got, want, phase)
		}
		if got := phasecontract.ArtifactName(phase); got != want {
			t.Errorf("phasecontract.ArtifactName(%q) = %q, want %q — the registry entry the callers resolve against was renamed or dropped", phase, got, want)
		}
	}

	if got := phasecontract.ArtifactName("ship"); got != "" {
		t.Errorf("phasecontract.ArtifactName(\"ship\") = %q, want \"\" — ship is NoArtifact and the empty return carries that distinction", got)
	}
	if got, want := phasecontract.ArtifactFilename("ship"), "ship-report.md"; got != want {
		t.Errorf("phasecontract.ArtifactFilename(\"ship\") = %q, want %q — the conventional fallback the call sites rely on", got, want)
	}

	const unregistered = "c1149-not-a-registered-phase"
	if got, want := phasecontract.ArtifactFilename(unregistered), unregistered+"-report.md"; got != want {
		t.Errorf("phasecontract.ArtifactFilename(%q) = %q, want %q — the <phase>-report.md fallback is gone", unregistered, got, want)
	}
}

func TestC1149_003_RemovalClaimGateReadsRegistryNamedBuildReport(t *testing.T) {
	const claimed = "c1149-still-here.txt"
	report := "# Build Report\n\n```json\n{\"removedPaths\": [\"" + claimed + "\"]}\n```\n"

	check := func(t *testing.T, rel string) []string {
		t.Helper()
		workspace := t.TempDir()
		worktree := t.TempDir()
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

	if failures := check(t, "build-report-c1149-not-the-contract.md"); len(failures) != 0 {
		t.Errorf("core.RemovalClaimFailures reported %v for a report that is NOT at the contracted artifact name — the gate is matching something other than the registry filename", failures)
	}
}

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

	if _, otherRan := read(t, "audit-report-c1149-not-the-contract.md"); otherRan {
		t.Errorf("coherence.ReadCycleVerdicts reported auditRan=true for a report that is NOT at the contracted artifact name — the reader is matching something other than the registry filename")
	}
}

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
	if !strings.Contains(string(src), `"`+tddReport+`"`) &&
		!strings.Contains(string(src), "phasecontract.ArtifactName(") &&
		!strings.Contains(string(src), "phasecontract.ArtifactFilename(") {
		t.Errorf("%s names neither %q nor a phasecontract resolver — the TDD half of "+
			"manifestReportFiles was dropped by the SSOT sweep, shrinking ship's manifest coverage",
			manifest, tddReport)
	}
}

func TestC1149_006_ProductionTreeCompilesAfterSubstitution(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "build", "-C", goDir, "./...")
	if err != nil || code != 0 {
		t.Errorf("`go build ./...` in %s failed (code=%d err=%v):\n%s", goDir, code, err, stderr)
	}
}
