package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TDD RED (cycle-806, task ciparity-integration-tier).
//
// Config.CheckIntegrationTier and integrationTierCheckDefault do not yet exist
// → compile RED until Builder adds the integration-tier CI-parity gate. Root
// cause it closes: the `go` workflow's `-tags integration` job (TestFleetSoak)
// went red while the per-cycle audit stayed green, because ciparity ran
// go vet / acs-durable / apicover but NEVER the integration tier — the
// warnship_apicover_ci_gap disease, one tier up (per-cycle proof ⊊ repo CI).

// AC3.1 — an integration-tier gate reporting offenders must FAIL audit even
// when the EGPS suite is green and the report says PASS, exactly like the other
// CI-parity gates (mirrors TestRun_CIParityGate_Offenders_FAILsAudit). Forces
// the Config.CheckIntegrationTier field + its applyCIGate wiring.
func TestRun_IntegrationTierGate_Offenders_FAILsAudit(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0) // EGPS green → only the integration-tier gate can FAIL.
	phase := New(Config{
		Bridge:  &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts: fakePromptsFS("body"),
		CheckIntegrationTier: func(core.PhaseRequest) ([]string, error) {
			return []string{"--- FAIL: TestFleetSoak_AllFourInvariants"}, nil
		},
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL (integration-tier gate reported offenders)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "integration") {
		t.Errorf("want a diagnostic mentioning the integration tier; got %+v", resp.Diagnostics)
	}
}

// AC3.2 (edge / no-op) — the default gate is a no-op (nil,nil) when the
// worktree has no go module, mirroring goVetCheckDefault/cycleTouchedGo so the
// gate never fires against a synthetic or docs-only cycle. Forces
// integrationTierCheckDefault to exist with the same guard.
func TestIntegrationTierCheckDefault_NoOpWithoutGoModule(t *testing.T) {
	root := t.TempDir() // no go/go.mod present
	offenders, err := integrationTierCheckDefault(core.PhaseRequest{Cycle: 1, ProjectRoot: root, Worktree: root})
	if err != nil {
		t.Fatalf("no-op gate returned err: %v", err)
	}
	if len(offenders) != 0 {
		t.Errorf("integration-tier gate must no-op without a go module, got offenders %v", offenders)
	}
}

// AC3.3 (membership / anti-drift pin) — NewDefault must WIRE the real
// integration-tier gate (cycle-147 dormant-seam lesson), and that gate must
// actually build the test binary under `-tags integration`. Proof that does NOT
// couple to the exact -run pattern: a fixture cmd/evolve package with an
// integration-tagged test file that FAILS TO COMPILE only under that tag. Under
// `-tags integration` the whole test binary fails to build → non-zero exit →
// offenders, regardless of any -run filter (go compiles the binary before -run
// selection). Strip the tag from the gate command and the file is excluded, the
// package compiles clean, and this test fails — so it pins the tag membership.
func TestNewDefault_WiresIntegrationTierGate(t *testing.T) {
	if testing.Short() {
		t.Skip("skips real `go test -tags integration` subprocess under -short; full `go test` + CI still run it")
	}
	root := t.TempDir()
	cmdDir := filepath.Join(root, "go", "cmd", "evolve")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "go.mod"), []byte("module inttest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-test file so `package main` in ./cmd/evolve is a real, buildable package.
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Integration-tagged test that references an undefined symbol: it compiles
	// ONLY into the -tags integration test binary, and then fails the build.
	brokenTest := "//go:build integration\n\npackage main\n\nimport \"testing\"\n\n" +
		"func TestFleetSoak_IntegrationFixture(t *testing.T) { _ = thisSymbolDoesNotExistUnderIntegration }\n"
	if err := os.WriteFile(filepath.Join(cmdDir, "soak_integration_fixture_test.go"), []byte(brokenTest), 0o644); err != nil {
		t.Fatal(err)
	}
	// Build handoff naming a changed Go package → cycleTouchedGo true → the gate runs.
	buildRun := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(buildRun, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildRun, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_modified":["go/cmd/evolve/main.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	offenders, err := integrationTierCheckDefault(core.PhaseRequest{Cycle: 9, ProjectRoot: root, Worktree: root})
	if err != nil {
		t.Fatalf("integration-tier gate could not run: %v", err)
	}
	if len(offenders) == 0 {
		t.Errorf("integration-tier gate did not catch a failing //go:build integration package — is `-tags integration` actually in the gate command?")
	}
}
