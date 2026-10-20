//go:build acs

package cycle1999

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build evolve: %v\n%s", err, out)
	}
	return bin
}

func TestC1999_001_RoutingPlanIsAKnownSubcommandThatRefusesAnUnknownCycleWithExitOne(t *testing.T) {
	bin := buildEvolve(t)
	project := acsassert.SetupTempProject(t)

	stdout, stderr, code, _ := acsassert.SubprocessOutput(bin, "routing", "plan", "--cycle", "9999", "--project-root", project)

	if strings.Contains(stderr, "unknown subcommand") {
		t.Fatalf("RED: routing plan is not a subcommand yet: %q", stderr)
	}
	if code != 1 {
		t.Errorf("routing plan --cycle 9999 exit = %d, want 1 (unknown cycle)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "9999") {
		t.Errorf("stderr must name the unknown cycle: %q", stderr)
	}
}

func TestC1999_002_RoutingPlanOfAnUnknownCycleCreatesNoRunDirectory(t *testing.T) {
	bin := buildEvolve(t)
	project := acsassert.SetupTempProject(t)

	acsassert.SubprocessOutput(bin, "routing", "plan", "--cycle", "9999", "--project-root", project)

	if _, err := os.Stat(filepath.Join(project, ".evolve", "runs", "cycle-9999")); !os.IsNotExist(err) {
		t.Errorf("routing plan created or touched runs/cycle-9999 (stat err = %v)", err)
	}
	entries, err := os.ReadDir(filepath.Join(project, ".evolve", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("routing plan left %d entries under .evolve/runs, want none", len(entries))
	}
}

func TestC1999_003_RoutingPlanRequiresACycleAndTheUnknownSubcommandHintNamesPlan(t *testing.T) {
	bin := buildEvolve(t)
	project := acsassert.SetupTempProject(t)

	_, stderr, code, _ := acsassert.SubprocessOutput(bin, "routing", "plan", "--project-root", project)
	if strings.Contains(stderr, "unknown subcommand") {
		t.Fatalf("RED: routing plan is not a subcommand yet: %q", stderr)
	}
	if code != 10 || !strings.Contains(stderr, "--cycle") {
		t.Errorf("routing plan without --cycle: exit = %d, stderr = %q, want exit 10 naming --cycle", code, stderr)
	}

	_, hint, hintCode, _ := acsassert.SubprocessOutput(bin, "routing", "bogus")
	if hintCode != 10 || !strings.Contains(hint, "plan") {
		t.Errorf("routing bogus: exit = %d, stderr = %q, want exit 10 listing plan among the subcommands", hintCode, hint)
	}
}
