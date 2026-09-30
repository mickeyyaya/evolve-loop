//go:build acs

package cycle748

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	ciwatchPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	preflightPkg = "github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
	dossierPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	policyPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
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

func TestC748_001_FailedCIRunFilesCriticalInboxItem(t *testing.T) {
	runGoTest(t, ciwatchPkg, "TestCIWatch_FailedRunFilesCriticalInboxItem")
}

func TestC748_002_GreenCIRunFilesNoInboxItem(t *testing.T) {
	runGoTest(t, ciwatchPkg, "TestCIWatch_GreenRunFilesNoInboxItem")
}

func TestC748_003_ReleaseRefusesTagOnRedCI(t *testing.T) {
	runGoTest(t, preflightPkg, "TestRun_RefusesTagOnRedReleaseCommitCI")
}

func TestC748_004_ReleaseCIOverrideLogsLoudly(t *testing.T) {
	runGoTest(t, preflightPkg, "TestRun_CIOverrideAllowsRedCIAndLogsLoudly")
}

func TestC748_005_CIVerdictAppearsInCycleDossier(t *testing.T) {
	runGoTest(t, dossierPkg, "TestBuild_IngestsCIWatchVerdict")
}

func TestC748_006_CIWatchKnobsResolveFromPolicyJSON(t *testing.T) {
	runGoTest(t, policyPkg, "TestCIWatchPolicy_KnobsFromPolicyJSON")
}

// acs-predicate: config-check
func TestC748_007_NoNewCIWatchEnvFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	table := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileNotContains(t, table, "CI_WATCH") {
		t.Errorf("flagregistry registry_table.go gained a CI_WATCH env flag — AC4 requires policy.json knobs only (zero new env flags)")
	}
}
