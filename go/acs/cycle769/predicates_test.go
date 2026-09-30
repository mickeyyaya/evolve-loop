//go:build acs

package cycle769

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	reaperPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/sessionreaper"
	preflightPkg = "github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
)

func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

func TestC769_001_SecondSweepSkipsReapedRuns(t *testing.T) {
	runGoTest(t, reaperPkg, "TestReapOrphans_SecondSweepSkipsReapedRuns")
}

func TestC769_002_PartialFailureNotTombstoned(t *testing.T) {
	runGoTest(t, reaperPkg, "TestReapOrphans_PartialFailureNotTombstoned")
}

func TestC769_003_PreflightOrphanReapDeadlineBounded(t *testing.T) {
	runGoTest(t, preflightPkg, "TestPreflight_OrphanReapIsDeadlineBounded")
}

func TestC769_004_VetTouchedPackages(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "vet",
		"github.com/mickeyyaya/evolve-loop/go/internal/sessionreaper",
		"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight",
		"github.com/mickeyyaya/evolve-loop/go/internal/swarm",
		"github.com/mickeyyaya/evolve-loop/go/cmd/evolve",
	)
	if code != 0 || err != nil {
		t.Fatalf("go vet exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
