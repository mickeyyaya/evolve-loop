//go:build acs

package cycle672

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	phasestreamPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	runnerPkg      = "github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	bridgePkg      = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	classifyPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC672_001_ProduceEchoVetoWired(t *testing.T) {
	ok, out := runGoTest(t, phasestreamPkg, "TestC672_001_ProduceInjectedPromptEchoVeto")
	if !ok {
		t.Errorf("Produce does not thread the injected prompt into the live Classifier (ProduceConfig.InjectedPrompt wiring missing):\n%s", out)
	}
}

func TestC672_002_RunnerThreadsPrompt(t *testing.T) {
	ok, out := runGoTest(t, runnerPkg, "TestC672_002_RunnerThreadsComposedPromptToEventsProducer")
	if !ok {
		t.Errorf("BaseRunner's events producer drops the composed prompt — echoed prompt text still classifies infra_failure in <phase>-events.ndjson:\n%s", out)
	}
}

func TestC672_003_TickEchoVetoWired(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg,
		"TestC672_003_TickEchoedExhaustionDoesNotEscalate|TestC672_005_ResponderConstructionSitesCarryInjectedPrompt")
	if !ok {
		t.Errorf("autoResponder.tick still scans the RAW pane for exhaustion (stripPromptEchoLines unwired) or a construction site drops the prompt:\n%s", out)
	}
}

func TestC672_004_GenuineSignalsSurvive(t *testing.T) {
	if ok, out := runGoTest(t, bridgePkg,
		"TestC672_004_TickGenuineExhaustionStillEscalates|TestC654_004_EchoedExhaustionStrippedGenuineSurvives"); !ok {
		t.Errorf("genuine CLI exhaustion no longer escalates — the echo-veto wiring over-corrected (bridge):\n%s", out)
	}
	if ok, out := runGoTest(t, phasestreamPkg, "TestClassifier_SetInjectedPrompt"); !ok {
		t.Errorf("cycle-654 normalizer regression arm broke (phasestream):\n%s", out)
	}
	if ok, out := runGoTest(t, classifyPkg, "TestC654_002_GenuineInfraStillVetoes"); !ok {
		t.Errorf("genuine runtime infra no longer classifies infrastructure (cycleclassify regression):\n%s", out)
	}
}
