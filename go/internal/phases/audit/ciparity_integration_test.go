package audit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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
// worktree has no go module, mirroring goVetCheckDefault/changedScopeForGate so the
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

// AC3.3 (membership / anti-drift pin) — NewDefault must wire the real
// integration-tier gate, and that gate must actually build the test binary
// under `-tags integration`. Proof that does NOT
// couple to the exact -run pattern: a fixture cmd/tool package (not env-exclusive — membership is the record table integrationTierEnvExclusive, the single authority) with an
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
	cmdDir := filepath.Join(root, "go", "cmd", "tool")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "go.mod"), []byte("module inttest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-test file so `package main` in ./cmd/tool is a real, buildable package.
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
	// Build handoff naming a changed Go package → changedScopeForGate run=true → the gate runs.
	buildRun := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(buildRun, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildRun, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_modified":["go/cmd/tool/main.go"]}]}`), 0o644); err != nil {
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

// writeRaceFixtureWorktree builds a minimal cycle worktree whose only
// integration-tagged test contains a GENUINE data race: two hundred goroutines
// doing an unsynchronized read-modify-write on a shared int. Such a race is
// invisible to a plain `go test -tags integration` run (an int race never
// panics — it silently loses updates) and is reliably reported only under
// `-race`. That asymmetry is the whole point: the fixture proves the gate change
// by BEHAVIOR (does it catch a real race?), not by grepping the args for the
// string "-race" (which a no-op could satisfy). Mirrors the compile-fail fixture
// setup in TestNewDefault_WiresIntegrationTierGate, swapping the broken-compile
// file for a race file.
func writeRaceFixtureWorktree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cmdDir := filepath.Join(root, "go", "cmd", "tool")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "go.mod"), []byte("module inttest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A real, unsynchronized data race that compiles+passes under plain
	// `-tags integration` and fails ONLY under `-race -tags integration`.
	raceTest := "//go:build integration\n\npackage main\n\n" +
		"import (\n\t\"sync\"\n\t\"testing\"\n)\n\n" +
		"func TestRaceFixture_UnsynchronizedCounter(t *testing.T) {\n" +
		"\tcounter := 0\n" +
		"\tvar wg sync.WaitGroup\n" +
		"\tfor i := 0; i < 200; i++ {\n" +
		"\t\twg.Add(1)\n" +
		"\t\tgo func() {\n" +
		"\t\t\tdefer wg.Done()\n" +
		"\t\t\tcounter++ // unsynchronized read-modify-write → data race (only -race catches it)\n" +
		"\t\t}()\n" +
		"\t}\n" +
		"\twg.Wait()\n" +
		"\tif counter < 0 {\n\t\tt.Fatal(\"unreachable\")\n\t}\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(cmdDir, "race_integration_fixture_test.go"), []byte(raceTest), 0o644); err != nil {
		t.Fatal(err)
	}
	// Build handoff naming a changed Go package → changedScopeForGate run=true → gate runs.
	buildRun := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(buildRun, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildRun, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_modified":["go/cmd/tool/main.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// AC1 (behavioral) — the integration-tier gate must catch a real data race,
// which is only possible when its `go test` command carries `-race`. This
// proves the flag by effect, never by string presence.
func TestIntegrationTierGate_Race(t *testing.T) {
	if testing.Short() {
		t.Skip("skips real `go test -race -tags integration` subprocess under -short; full `go test` + CI still run it")
	}
	root := writeRaceFixtureWorktree(t)
	offenders, err := integrationTierCheckDefault(core.PhaseRequest{Cycle: 9, ProjectRoot: root, Worktree: root})
	if err != nil {
		t.Fatalf("integration-tier gate could not run: %v", err)
	}
	if len(offenders) == 0 {
		t.Fatalf("integration-tier gate did not catch a genuine data race — is `-race` actually in the gate command (ciparity.go:205)? The fixture races on an int counter, invisible without -race.")
	}
	if joined := strings.Join(offenders, "\n"); !strings.Contains(joined, "FAIL") {
		t.Errorf("gate reported offenders but none read as a test FAIL (expected the race-detector FAIL line): %v", offenders)
	}
}

// AC2 (negative / anti-gaming) — proves the fixture's failure is a genuine
// race, not a compile or logic error a mere flag-string flip would also
// surface: the same fixture, run under plain `-tags integration` (no -race),
// must PASS, pinning that the detection in TestIntegrationTierGate_Race can
// only come from `-race`.
func TestIntegrationTierGate_RaceFixtureIsRaceOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skips real `go test -tags integration` subprocess under -short; full `go test` + CI still run it")
	}
	root := writeRaceFixtureWorktree(t)
	cmd := exec.Command("go", "test", "-count=1", "-tags", "integration", "./...")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("race fixture must PASS under plain `-tags integration` (no -race), proving it is a race-only failure; got err %v\n%s", err, out)
	}
}
