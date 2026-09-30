//go:build acs

package cycle654

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cycleclassifyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	phasestreamPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	bridgePkg        = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
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

func TestC654_001_SourceOfTruthVetoGate(t *testing.T) {
	ok, out := runGoTest(t, cycleclassifyPkg, "TestC654_001_PassDeliverableExit0EchoNotInfraVeto")
	if !ok {
		t.Errorf("cycleclassify vetoes a PASS/exit-0 phase on a prompt-echo infra_failure (source-of-truth gate missing):\n%s", out)
	}
}

func TestC654_002_GenuineInfraStillVetoes(t *testing.T) {
	ok, out := runGoTest(t, cycleclassifyPkg, "TestC654_002_GenuineInfraStillVetoes")
	if !ok {
		t.Errorf("genuine runtime infra no longer classifies infrastructure — the echo-veto fix over-corrected:\n%s", out)
	}
}

func TestC654_003_NormalizerPromptEchoGate(t *testing.T) {
	ok, out := runGoTest(t, phasestreamPkg, "TestClassifier_SetInjectedPrompt")
	if !ok {
		t.Errorf("normalizer still emits infra_failure for echoed prompt text (SetInjectedPrompt gate missing):\n%s", out)
	}
}

func TestC654_004_EscalationPromptEchoGate(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestC654_004_EchoedExhaustionStrippedGenuineSurvives")
	if !ok {
		t.Errorf("escalation matcher still fires on echoed prompt exhaustion text (stripPromptEchoLines missing):\n%s", out)
	}
}
