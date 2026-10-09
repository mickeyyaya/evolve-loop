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

func TestRun_IntegrationTierGate_Offenders_FAILsAudit(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
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

func TestIntegrationTierCheckDefault_NoOpWithoutGoModule(t *testing.T) {
	root := t.TempDir()
	offenders, err := integrationTierCheckDefault(core.PhaseRequest{Cycle: 1, ProjectRoot: root, Worktree: root})
	if err != nil {
		t.Fatalf("no-op gate returned err: %v", err)
	}
	if len(offenders) != 0 {
		t.Errorf("integration-tier gate must no-op without a go module, got offenders %v", offenders)
	}
}

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
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brokenTest := "//go:build integration\n\npackage main\n\nimport \"testing\"\n\n" +
		"func TestFleetSoak_IntegrationFixture(t *testing.T) { _ = thisSymbolDoesNotExistUnderIntegration }\n"
	if err := os.WriteFile(filepath.Join(cmdDir, "soak_integration_fixture_test.go"), []byte(brokenTest), 0o644); err != nil {
		t.Fatal(err)
	}
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
