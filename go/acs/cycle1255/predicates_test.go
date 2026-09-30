//go:build acs

package cycle1255

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	retroPkg        = "github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	bridgePkg       = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	changedpkgsPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	ampPhaseSpecRel = ".evolve/phases/test-amplification/phase.json"
)

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag the default suite skips). exit=%d\n"+
				"combined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

func TestC1255_001_RetroEmptyWorktreeFallsBackToScratch(t *testing.T) {
	assertDefaultSuiteTestsPass(t, retroPkg,
		"TestRetro_EmptyWorktree_FallsBackToScratchUnderWorkspace",
		"TestRetro_EmptyWorktree_NeverMainTreeOrProcessCwd",
		"TestRetro_RealWorktree_PassedThroughUnchanged",
		"TestRetro_EmptyWorktreeAndWorkspace_NoFabricatedPath",
	)
}

func TestC1255_002_FleetWorktreeGuardNotWidened(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestFleetModeRefusesEmptyWorktree",
		"TestRecipeFleetModeRefusesEmptyWorktree",
		"TestApplyScratchCwd_NoOpWhenWorktreeSet",
		"TestApplyScratchCwd_NoOpWhenNoWorkspace",
	)
}

func TestC1255_003_CoveringTestsDeriverBehaviour(t *testing.T) {
	assertDefaultSuiteTestsPass(t, changedpkgsPkg,
		"TestCoveringTests_DerivesTestFilesForChangedPackagesOnly",
		"TestCoveringTests_DedupesAcrossOverlappingPatterns",
		"TestCoveringTests_AcceptsNonRecursivePatternForm",
		"TestCoveringTests_FailsOpenOnUnusableInput",
	)
}

func TestC1255_004_CoveringTestsReachableFromProduction(t *testing.T) {
	assertDefaultSuiteTestsPass(t, changedpkgsPkg,
		"TestCoveringTests_ReachableFromProduction",
	)
}

// acs-predicate: config-check
func TestC1255_005_AmplificationPhaseDeclaresCoveringTestsInput(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, filepath.FromSlash(ampPhaseSpecRel))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", ampPhaseSpecRel, err)
	}
	var spec struct {
		Inputs struct {
			Files []string `json:"files"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("%s is not valid JSON: %v", ampPhaseSpecRel, err)
	}

	var hit string
	for _, f := range spec.Inputs.Files {
		if strings.Contains(f, "covering-tests") {
			hit = f
			break
		}
	}
	if hit == "" {
		t.Fatalf("%s inputs.files = %v — none names a covering-tests artifact, so the derived corpus never reaches the agent and the blind whole-repo search continues", ampPhaseSpecRel, spec.Inputs.Files)
	}
	if !strings.Contains(hit, "{cycle}") {
		t.Errorf("covering-tests input %q has no {cycle} placeholder — it would pin every cycle to one run's artifact", hit)
	}
	for _, want := range []string{"tdd-contract.md", "build-report.md"} {
		found := false
		for _, f := range spec.Inputs.Files {
			if strings.Contains(f, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s inputs.files = %v — the pre-existing %s input was dropped", ampPhaseSpecRel, spec.Inputs.Files, want)
		}
	}
}
