//go:build acs

package cycle2270

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePackage  = "./internal/core"
	buildPackage = "./internal/phases/build"
)

func TestC2270_001_APresentManifestReportsAncestorTrueOrFalseAgainstTheProjectHead(t *testing.T) {
	names := []string{
		"TestContinuationAncestor_SalvageOnProjectHeadReportsTrue",
		"TestContinuationAncestor_SalvageOffProjectHeadReportsFalse",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: corePackage,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2270_002_AnAbsentOrUnreadableManifestYieldsNothingAndRunsNoGit(t *testing.T) {
	names := []string{
		"TestContinuationAncestor_AbsentManifestYieldsNothingAndRunsNoGit",
		"TestContinuationAncestor_UnreadableManifestYieldsNothing",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: corePackage,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2270_003_AnUnresolvableOrMalformedShaReportsUnknownNeverFalse(t *testing.T) {
	names := []string{
		"TestContinuationAncestor_UnknownSHAReportsUnknownNotFalse",
		"TestContinuationAncestor_ExitCodeMapsToVerdict",
		"TestContinuationAncestor_MalformedSHAIsUnknownAndNeverReachesGit",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: corePackage,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2270_004_TheProbeRunsThroughTheInjectedGitRunnerWithTheExactArgv(t *testing.T) {
	names := []string{"TestContinuationAncestor_ProbeAsksGitExactlyMergeBaseIsAncestorAgainstHead"}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: corePackage,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2270_005_AdoptionServesTheLineAndAFreshCycleServesNone(t *testing.T) {
	names := []string{
		"TestContinuationAncestor_AdoptionServesTheLineFromTheProjectRootHead",
		"TestContinuationAncestor_FreshCycleCarriesNoLine",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: corePackage,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2270_006_TheBuildPromptRendersTheServedLineAndIsUnchangedWithout(t *testing.T) {
	names := []string{
		"TestComposePrompt_ContinuationAncestorLineRendersWhenServed",
		"TestComposePrompt_ContinuationAncestorAbsentOrEmptyIsByteIdentical",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: buildPackage,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}
