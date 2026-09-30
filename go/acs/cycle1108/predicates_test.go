//go:build acs

package cycle1108

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"

const manifestPkg = "github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1108_001_PorcelainDecodesQuotedPaths(t *testing.T) {
	ok, out := runGoTest(t, manifestPkg, "TestPorcelainChangedPaths_QuotePathUnescapesNonASCII")
	if !ok {
		t.Errorf("porcelain classification still yields git's C-quoted escape text instead of the "+
			"real path — a changed file whose name needs quoting matches no manifest entry and "+
			"stages as a no-op:\n%s", out)
	}
}

func TestC1108_002_AsciiClassificationUnchanged(t *testing.T) {
	ok, out := runGoTest(t, manifestPkg, "TestPorcelainChangedPaths_QuotePathAsciiUnchanged")
	if !ok {
		t.Errorf("the unquote change altered ASCII path classification — mangling the overwhelmingly "+
			"common case is a worse regression than the bug it fixes:\n%s", out)
	}
}

func TestC1108_003_GitReadsDisableQuotePath(t *testing.T) {
	ok, out := runGoTest(t, shipPkg, "TestStageExplicitPaths_QuotePathDisabledOnGitReads")
	if !ok {
		t.Errorf("ship still reads `git status --porcelain` / `git check-ignore` with git's default "+
			"core.quotePath=true (or passes the config arg after the subcommand, where git rejects "+
			"it), so every non-ASCII path arrives escaped:\n%s", out)
	}
}

func TestC1108_004_IgnoredProbeMatchesQuotedOutput(t *testing.T) {
	ok, out := runGoTest(t, shipPkg, "TestDropIgnoredPaths_QuotePathMatchesQuotedProbeOutput")
	if !ok {
		t.Errorf("the check-ignore filter still keys off git's quoted spelling, so an ignored "+
			"quote-bearing path survives into `git add` and reproduces the cycle-1101 rc=1 "+
			"refusal:\n%s", out)
	}
}

func TestC1108_005_IgnoredProbeNeverOverMatches(t *testing.T) {
	ok, out := runGoTest(t, shipPkg, "TestDropIgnoredPaths_QuotePathKeepsUnignoredPaths")
	if !ok {
		t.Errorf("the check-ignore filter now drops paths git never reported as ignored — an "+
			"under-staged ship commits less than it declares:\n%s", out)
	}
}

func TestC1108_006_ShipStagingContractStillGreen(t *testing.T) {
	if ok, out := runGoTest(t, shipPkg,
		"TestShipDirect_CycleClass_.*|TestShipDirect_AReportlessWorkspaceAdoptsNoPath|"+
			"TestShipDirect_NoWorkspacePath_StillStagesExplicitly|TestShipDirect_NonReleaseClasses_NeverAddAll|"+
			"TestShipDirect_CheckIgnoreProbeFailure_FailsOpen|TestShipFromWorktree_.*|"+
			"TestStageExplicitPaths_AlreadyStagedDeletion"); !ok {
		t.Errorf("the ship staging contract (explicit pathspec / repo-relative filter / ignored-path "+
			"drop / staged-deletion handling) regressed:\n%s", out)
	}
	if ok, out := runGoTest(t, manifestPkg, "TestStagePathspec_.*|TestIsRepoRelative|TestExtractReportPaths_.*"); !ok {
		t.Errorf("the ship path-selection contract (pathspec / repo-relative filter / report paths) regressed:\n%s", out)
	}
}
