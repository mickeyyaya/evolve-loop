//go:build acs

package cycle427

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runPolicyTest(t *testing.T, pkg, runFilter string) (string, string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runFilter, pkg,
	)
	return stdout, stderr, code
}

func codexTestdataFrame(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	b, err := os.ReadFile(filepath.Join(dir, "..", "..", "internal", "bridge", "panestream", "testdata", "codex", name))
	if err != nil {
		t.Fatalf("codexTestdataFrame(%q): %v", name, err)
	}
	return string(b)
}

const policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"

func TestC427_001_ParallelEvaluatePolicyAbsentDefaults(t *testing.T) {
	_, stderr, code := runPolicyTest(t, policyPkg, "TestParallelEvaluateConfig_AbsentDefaults")
	if code != 0 {
		t.Errorf("C427_001: policy absent-defaults test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_002_ParallelEvaluatePolicyStageShadow(t *testing.T) {
	_, stderr, code := runPolicyTest(t, policyPkg, "TestParallelEvaluateConfig_StageOverrideShadow")
	if code != 0 {
		t.Errorf("C427_002: policy stage-shadow test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_003_ParallelEvaluatePolicyStageEnforce(t *testing.T) {
	_, stderr, code := runPolicyTest(t, policyPkg, "TestParallelEvaluateConfig_StageOverrideEnforce")
	if code != 0 {
		t.Errorf("C427_003: policy stage-enforce test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_004_ParallelEvaluatePolicyUnknownStageFallsToOff(t *testing.T) {
	_, stderr, code := runPolicyTest(t, policyPkg, "TestParallelEvaluateConfig_UnknownStageFallsToOff")
	if code != 0 {
		t.Errorf("C427_004: policy unknown-stage-fallsoff test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_005_ParallelEvaluatePolicyZeroConcurrencyDefault(t *testing.T) {
	_, stderr, code := runPolicyTest(t, policyPkg, "TestParallelEvaluateConfig_ZeroConcurrencyDefaultsTo3|TestParallelEvaluateConfig_NegativeConcurrencyDefaultsTo3")
	if code != 0 {
		t.Errorf("C427_005: policy zero/negative-concurrency test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_006_ParallelEvaluateDefaultIsStageOff(t *testing.T) {
	const configPkg = "github.com/mickeyyaya/evolve-loop/go/internal/config"
	_, stderr, code := runPolicyTest(t, configPkg, "TestDefaults_ParallelEvaluate_Off")
	if code != 0 {
		t.Errorf("C427_006: config default-StageOff test exit=%d\nstderr=%s", code, stderr)
	}
}

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

func runBridgeTest(t *testing.T, runFilter string) (string, string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runFilter, bridgePkg,
	)
	return stdout, stderr, code
}

func TestC427_007_DriverLivenessRouting_Claude(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_ClaudeTmux")
	if code != 0 {
		t.Errorf("C427_007: claude-tmux routing test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_008_DriverLivenessRouting_Codex(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_CodexTmux")
	if code != 0 {
		t.Errorf("C427_008: codex-tmux routing test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_009_DriverLivenessRouting_Agy(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_AgyTmux")
	if code != 0 {
		t.Errorf("C427_009: agy-tmux routing test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_010_DriverLivenessRouting_Ollama(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_OllamaTmux")
	if code != 0 {
		t.Errorf("C427_010: ollama-tmux routing test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_011_DriverLivenessRouting_UnknownNeverNil(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_UnknownTmux")
	if code != 0 {
		t.Errorf("C427_011: unknown-tmux routing test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_012_StopReviewHasNoCLILiterals(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_StopReviewHasNoCLILiterals")
	if code != 0 {
		t.Errorf("C427_012: stopreview-no-cli-literals test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC427_013_CodexThinkingToAnswerConverging(t *testing.T) {
	p := panestream.Profiles["codex"]
	det := panestream.NewDefaultDetector(3)
	think := codexTestdataFrame(t, "thinking.txt")
	answer := codexTestdataFrame(t, "answer.txt")
	det.Assess(think, p)
	state, _ := det.Assess(answer, p)
	if state != panestream.LivenessConverging {
		t.Errorf("C427_013: thinking→answer got %v, want LivenessConverging", state)
	}
}

func TestC427_014_CodexPrimingNotHung(t *testing.T) {
	p := panestream.Profiles["codex"]
	answer := codexTestdataFrame(t, "answer.txt")
	det := panestream.NewDefaultDetector(1)
	state, _ := det.Assess(answer, p)
	if state == panestream.LivenessHung {
		t.Errorf("C427_014: prime call must NOT be LivenessHung (got %v)", state)
	}
}

func TestC427_015_CodexStalledIdleNotHung(t *testing.T) {
	p := panestream.Profiles["codex"]
	answer := codexTestdataFrame(t, "answer.txt")
	if panestream.PaneBusy(answer, p) {
		t.Fatal("C427_015 precondition: codex answer frame must not be busy")
	}
	det := panestream.NewDefaultDetector(3)
	det.Assess(answer, p)
	for i := 1; i <= 5; i++ {
		state, _ := det.Assess(answer, p)
		if state == panestream.LivenessHung {
			t.Errorf("C427_015: stall interval %d: got LivenessHung — impossible without busy affordance", i)
		}
		if state != panestream.LivenessIdle {
			t.Errorf("C427_015: stall interval %d: got %v, want LivenessIdle", i, state)
		}
	}
}

func TestC427_016_CodexConfidenceInRange(t *testing.T) {
	p := panestream.Profiles["codex"]
	frames := []string{"thinking.txt", "answer.txt", "final.txt"}
	for _, name := range frames {
		t.Run(name, func(t *testing.T) {
			det := panestream.NewDefaultDetector(3)
			content := codexTestdataFrame(t, name)
			_, c1 := det.Assess(content, p)
			_, c2 := det.Assess(content, p)
			for _, c := range []float64{c1, c2} {
				if c < 0 || c > 1 {
					t.Errorf("C427_016 [%s]: confidence %v out of [0,1]", name, c)
				}
			}
		})
	}
}
