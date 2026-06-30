//go:build acs

// Package cycle427 materialises the cycle-427 acceptance criteria for three tasks:
// T1 (parallel-evaluate-policy-injection), T2 (bridge-driver-liveness-routing-test),
// T3 (codex-stalled-liveness-regression).
//
// Goal: close residual gaps left after cycles 423–426 built and wired the
// LivenessDetector abstraction:
//   - T1: ParallelEvaluate lever blocked by a config-not-code violation
//     (config.go:248 has a hardcoded StageOff literal, no policy block exists).
//   - T2: bridge-level detectorFor seam has no test pinning it against regression.
//   - T3: codex stalled→Idle behavior (weak-signal closure) is undocumented by test.
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	parallel-evaluate-policy-injection (T1 — Medium, P0):
//	  AC1  absent block → default Stage="off", Concurrency=3          (positive)  → C427_001 RED (policy compile fail)
//	  AC2  stage "shadow" override persists                            (positive)  → C427_002 RED (policy compile fail)
//	  AC3  stage "enforce" override persists                           (positive)  → C427_003 RED (policy compile fail)
//	  AC4  unknown stage falls back to "off" (fail-safe, anti-enable) (negative)  → C427_004 RED (policy compile fail)
//	  AC5  zero/negative concurrency defaults to 3                    (edge)      → C427_005 RED (policy compile fail)
//	  AC6  defaults().RolloutStages.ParallelEvaluate == StageOff      (safety)    → C427_006 pre-GREEN
//
//	bridge-driver-liveness-routing-test (T2 — Small, P1):
//	  AC7  claude-tmux → ClaudeDetector                               (positive)  → C427_007 pre-GREEN
//	  AC8  codex-tmux → DefaultDetector                               (positive)  → C427_008 pre-GREEN
//	  AC9  agy-tmux → AgyDetector                                     (positive)  → C427_009 pre-GREEN
//	  AC10 ollama-tmux → OllamaDetector                               (positive)  → C427_010 pre-GREEN
//	  AC11 unknown-tmux → DefaultDetector, never nil                  (negative)  → C427_011 pre-GREEN
//	  AC12 stopreview.go has zero CLI literals                        (grep)      → C427_012 pre-GREEN
//
//	codex-stalled-liveness-regression (T3 — Small, P2):
//	  AC13 thinking→answer growth → LivenessConverging                (positive)  → C427_013 pre-GREEN
//	  AC14 prime call never LivenessHung                              (edge)      → C427_014 pre-GREEN
//	  AC15 stalled ≥3 intervals → LivenessIdle, never LivenessHung   (negative)  → C427_015 pre-GREEN
//	  AC16 confidence ∈ [0,1] across all frames                      (semantic)  → C427_016 pre-GREEN
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C427_004 (unknown stage must NOT enable), C427_011 (unknown driver never nil/coarse),
//	           C427_015 (stalled codex must NEVER produce Hung)
//	Edge/OOD:  C427_005 (zero/negative concurrency), C427_014 (priming guard)
//	Semantic:  C427_006 (safety-pin: StageOff default, blind-flip guard),
//	           C427_016 (confidence range invariant)
//
// 1:1 enforcement:
//
//	T1: predicate=6 (C427_001–006)  → total=6 ✓
//	T2: predicate=6 (C427_007–012) → total=6 ✓
//	T3: predicate=4 (C427_013–016) → total=4 ✓
//
// RED strategy:
//
//	T1 predicates invoke `go test ./internal/policy/ -run TestParallelEvaluateConfig_*`
//	via subprocess. The policy test file (parallel_evaluate_config_test.go) references
//	policy.ParallelEvaluatePolicy which doesn't exist → compile error in subprocess →
//	predicate fails. This keeps the ACS package itself compile-clean so T2/T3 predicates
//	run independently.
//	T2 and T3 predicates are pre-existing GREEN (production code already correct).
package cycle427

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// runPolicyTest runs `go test -count=1 -run <testName> <pkg>` as a subprocess
// from within the Go module (package import path). Returns (stdout, stderr, exit).
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
	// ACS lives at go/acs/cycle427/; testdata is at go/internal/bridge/panestream/testdata/
	dir := filepath.Dir(file)
	b, err := os.ReadFile(filepath.Join(dir, "..", "..", "internal", "bridge", "panestream", "testdata", "codex", name))
	if err != nil {
		t.Fatalf("codexTestdataFrame(%q): %v", name, err)
	}
	return string(b)
}

// ── T1: parallel-evaluate-policy-injection ───────────────────────────────────

const policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"

// TestC427_001_ParallelEvaluatePolicyAbsentDefaults (positive, RED):
// absent parallel_evaluate block → Stage="off", Concurrency=3.
func TestC427_001_ParallelEvaluatePolicyAbsentDefaults(t *testing.T) {
	_, stderr, code := runPolicyTest(t, policyPkg, "TestParallelEvaluateConfig_AbsentDefaults")
	if code != 0 {
		t.Errorf("C427_001: policy absent-defaults test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC427_002_ParallelEvaluatePolicyStageShadow (positive, RED):
// stage="shadow" override must persist through ParallelEvaluateConfig().
func TestC427_002_ParallelEvaluatePolicyStageShadow(t *testing.T) {
	_, stderr, code := runPolicyTest(t, policyPkg, "TestParallelEvaluateConfig_StageOverrideShadow")
	if code != 0 {
		t.Errorf("C427_002: policy stage-shadow test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC427_003_ParallelEvaluatePolicyStageEnforce (positive, RED):
// stage="enforce" override must persist through ParallelEvaluateConfig().
func TestC427_003_ParallelEvaluatePolicyStageEnforce(t *testing.T) {
	_, stderr, code := runPolicyTest(t, policyPkg, "TestParallelEvaluateConfig_StageOverrideEnforce")
	if code != 0 {
		t.Errorf("C427_003: policy stage-enforce test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC427_004_ParallelEvaluatePolicyUnknownStageFallsToOff (negative, RED):
// unknown stage string must map to "off", never to "enforce" or any active stage.
// This is the anti-accidental-enable guard (a typo in policy.json must not arm
// the parallel dispatcher).
func TestC427_004_ParallelEvaluatePolicyUnknownStageFallsToOff(t *testing.T) {
	_, stderr, code := runPolicyTest(t, policyPkg, "TestParallelEvaluateConfig_UnknownStageFallsToOff")
	if code != 0 {
		t.Errorf("C427_004: policy unknown-stage-fallsoff test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC427_005_ParallelEvaluatePolicyZeroConcurrencyDefault (edge, RED):
// zero and negative concurrency must default to 3, not produce a zero-concurrency
// dispatcher (which would deadlock or be useless).
func TestC427_005_ParallelEvaluatePolicyZeroConcurrencyDefault(t *testing.T) {
	_, stderr, code := runPolicyTest(t, policyPkg, "TestParallelEvaluateConfig_ZeroConcurrencyDefaultsTo3|TestParallelEvaluateConfig_NegativeConcurrencyDefaultsTo3")
	if code != 0 {
		t.Errorf("C427_005: policy zero/negative-concurrency test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC427_006_ParallelEvaluateDefaultIsStageOff (safety pin, pre-GREEN):
// the compile-time default in defaults() must be StageOff — the parallel
// dispatcher is dormant until explicitly enabled via policy.json shadow soak.
// This is the safety oracle against a blind enforce flip (per goal spec).
func TestC427_006_ParallelEvaluateDefaultIsStageOff(t *testing.T) {
	const configPkg = "github.com/mickeyyaya/evolve-loop/go/internal/config"
	_, stderr, code := runPolicyTest(t, configPkg, "TestDefaults_ParallelEvaluate_Off")
	if code != 0 {
		t.Errorf("C427_006: config default-StageOff test exit=%d\nstderr=%s", code, stderr)
	}
}

// ── T2: bridge-driver-liveness-routing-test ──────────────────────────────────

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

func runBridgeTest(t *testing.T, runFilter string) (string, string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runFilter, bridgePkg,
	)
	return stdout, stderr, code
}

// TestC427_007_DriverLivenessRouting_Claude (positive, pre-GREEN):
// claude-tmux must resolve to ClaudeDetector for the ↓-token-counter layer.
func TestC427_007_DriverLivenessRouting_Claude(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_ClaudeTmux")
	if code != 0 {
		t.Errorf("C427_007: claude-tmux routing test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC427_008_DriverLivenessRouting_Codex (positive, pre-GREEN):
// codex-tmux must resolve to DefaultDetector (growth-velocity only, no busy affordance).
func TestC427_008_DriverLivenessRouting_Codex(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_CodexTmux")
	if code != 0 {
		t.Errorf("C427_008: codex-tmux routing test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC427_009_DriverLivenessRouting_Agy (positive, pre-GREEN):
// agy-tmux must resolve to AgyDetector for the ⣯-spinner layer.
func TestC427_009_DriverLivenessRouting_Agy(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_AgyTmux")
	if code != 0 {
		t.Errorf("C427_009: agy-tmux routing test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC427_010_DriverLivenessRouting_Ollama (positive, pre-GREEN):
// ollama-tmux must resolve to OllamaDetector for the Thinking-header layer.
func TestC427_010_DriverLivenessRouting_Ollama(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_OllamaTmux")
	if code != 0 {
		t.Errorf("C427_010: ollama-tmux routing test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC427_011_DriverLivenessRouting_UnknownNeverNil (negative, pre-GREEN):
// unknown driver must return DefaultDetector, never nil (nil would panic in reviewer).
func TestC427_011_DriverLivenessRouting_UnknownNeverNil(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_UnknownTmux")
	if code != 0 {
		t.Errorf("C427_011: unknown-tmux routing test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC427_012_StopReviewHasNoCLILiterals (grep, pre-GREEN):
// stopreview.go must contain zero CLI-name literals. All per-CLI branching must
// live in panestream.DetectorFor (ADR-0047 single-source-with-projection).
func TestC427_012_StopReviewHasNoCLILiterals(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestDriverLivenessRouting_StopReviewHasNoCLILiterals")
	if code != 0 {
		t.Errorf("C427_012: stopreview-no-cli-literals test exit=%d\nstderr=%s", code, stderr)
	}
}

// ── T3: codex-stalled-liveness-regression ────────────────────────────────────
// T3 predicates directly exercise panestream.DefaultDetector with codex testdata.

// TestC427_013_CodexThinkingToAnswerConverging (positive, pre-GREEN):
// codex thinking→answer frame transition yields LivenessConverging.
func TestC427_013_CodexThinkingToAnswerConverging(t *testing.T) {
	p := panestream.Profiles["codex"]
	det := panestream.NewDefaultDetector(3)
	think := codexTestdataFrame(t, "thinking.txt")
	answer := codexTestdataFrame(t, "answer.txt")
	det.Assess(think, p) // prime
	state, _ := det.Assess(answer, p)
	if state != panestream.LivenessConverging {
		t.Errorf("C427_013: thinking→answer got %v, want LivenessConverging", state)
	}
}

// TestC427_014_CodexPrimingNotHung (edge, pre-GREEN):
// prime call on any codex frame must never return LivenessHung, regardless of
// stallThreshold (stallThreshold=1 forces earliest-possible Hung).
func TestC427_014_CodexPrimingNotHung(t *testing.T) {
	p := panestream.Profiles["codex"]
	answer := codexTestdataFrame(t, "answer.txt")
	det := panestream.NewDefaultDetector(1) // earlies-possible Hung
	state, _ := det.Assess(answer, p)
	if state == panestream.LivenessHung {
		t.Errorf("C427_014: prime call must NOT be LivenessHung (got %v)", state)
	}
}

// TestC427_015_CodexStalledIdleNotHung is the load-bearing negative test (pre-GREEN):
// repeated identical codex answer frames (stalled, no busy affordance) across
// ≥3 intervals must yield LivenessIdle and NEVER LivenessHung. Hung requires
// busy=true accumulating stalls, but PaneBusy is always false on codex frames —
// a structural invariant of the documented weak-signal closure.
func TestC427_015_CodexStalledIdleNotHung(t *testing.T) {
	p := panestream.Profiles["codex"]
	answer := codexTestdataFrame(t, "answer.txt")
	if panestream.PaneBusy(answer, p) {
		t.Fatal("C427_015 precondition: codex answer frame must not be busy")
	}
	det := panestream.NewDefaultDetector(3) // hungAfter=3, earliest Hung with 3 stalls
	det.Assess(answer, p)                   // prime
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

// TestC427_016_CodexConfidenceInRange (semantic, pre-GREEN):
// confidence must be in [0,1] for all codex testdata frames, both on prime and
// subsequent assess calls.
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
