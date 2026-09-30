//go:build acs

package cycle442

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	modelqueryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	bridgePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func runGoTest(t *testing.T, runFilter string, race bool, pkgs ...string) (stdout, stderr string, code int) {
	t.Helper()
	args := []string{"test", "-count=1"}
	if race {
		args = append(args, "-race")
	}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ = acsassert.SubprocessOutput("go", args...)
	return stdout, stderr, code
}

func TestC442_001_PromptDispatchedThroughSeam(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestPromptDispatcher_InterfaceContract|TestCLIClassifierClassify_DispatchesThroughPromptDispatcher", false, modelqueryPkg)
	if code != 0 {
		t.Errorf("C442_001: prompt-dispatched-through-seam tests exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC442_002_NilDispatcherErrorsNeverShellsOut(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestCLIClassifierClassify_NilDispatcherErrorsNeverShellsOut", false, modelqueryPkg)
	if code != 0 {
		t.Errorf("C442_002: nil-dispatcher-errors test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC442_003_NoRawExecLeftInClassifier(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestGuard_ClassifierHasNoDirectModelExec", false, modelqueryPkg)
	if code != 0 {
		t.Errorf("C442_003: no-raw-exec-in-classifier guard exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC442_004_DispatcherFailurePropagates(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestCLIClassifierClassify_DispatcherErrorPropagates", false, modelqueryPkg)
	if code != 0 {
		t.Errorf("C442_004: dispatcher-failure-propagates test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC442_005_ClassifySemanticsSurviveTheSeam(t *testing.T) {
	_, stderr, code := runGoTest(t,
		"TestCLIClassifierClassify_SkipsPromptEcho|TestCLIClassifierClassify_BadReply|TestCLIClassifierClassify_AllObjectsFailToMap|TestCLIClassifierGuards",
		false, modelqueryPkg)
	if code != 0 {
		t.Errorf("C442_005: classify-semantics-survive-the-seam tests exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC442_006_NoRegressionAcrossTouchedPackages(t *testing.T) {
	_, stderr, code := runGoTest(t, "", true, modelqueryPkg, bridgePkg)
	if code != 0 {
		t.Errorf("C442_006: modelquery+bridge -race suite exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC442_007_OllamaListReachesNoModelDocumented(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestOllamaListerReachesNoModel|TestOllamaListMetadataExceptionDocumented", false, modelqueryPkg)
	if code != 0 {
		t.Errorf("C442_007: ollama-list-metadata-exception tests exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC442_008_OllamaRegressionPathsStayGreen(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestOllamaListerError|TestOllamaListerUsesRunner|TestParseOllamaList", false, modelqueryPkg)
	if code != 0 {
		t.Errorf("C442_008: ollama-regression-paths-stay-green tests exit=%d\nstderr=%s", code, stderr)
	}
}
