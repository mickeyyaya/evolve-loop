//go:build acs

// Package cycle424 materializes the cycle-424 acceptance criteria for two tasks:
// a driver-scoped boot-timeout bench mechanism (T1) and an ollama liveness strategy
// composed over DefaultDetector (T2).
//
// Goal: reduce cycle latency by eliminating repeated dead-boot cycles (T1) and
// demonstrating the liveness abstraction is genuinely extensible (T2).
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	repl-boot-timeout-driver-bench (T1 — Medium):
//	  AC1  BootTimeoutPattern constant and Benchable("repl_boot_timeout")=true  → C424_001
//	  AC2  IsBootTimeoutExitCode(80)=true; IsBootTimeoutExitCode(81)=false       → C424_002
//	  AC3  ≥threshold consecutive RecordBootStrike benches the driver (positive)  → C424_003
//	  AC4  Single RecordBootStrike does NOT bench (negative — threshold guard)    → C424_004
//	  AC5  Driver-scoped: "codex-tmux" bench does NOT bench "codex" (anti-gaming) → C424_005
//
//	ollama-liveness-strategy (T2 — Small):
//	  AC6  OllamaDetector → Converging+higher-conf on "Thinking…" frame w/ no content delta → C424_006
//	  AC7  No "Thinking…" header → OllamaDetector byte-identical to DefaultDetector         → C424_007
//	  AC8  Malformed/empty frames → no panic (edge/OOD)                                     → C424_008
//	  AC9  DetectorFor("ollama") routes to OllamaDetector (conf uplift vs Default)          → C424_009
//	  AC10 stopreview.go has no CLI-name literals (config-check waiver; pre-existing GREEN)  → C424_010
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C424_004 (1 strike < threshold → no bench), C424_007 (no-Thinking fallback)
//	Edge/OOD:  C424_002 (non-80 not a boot exit), C424_005 (driver-scoped isolation), C424_008 (no panic)
//	Semantic:  10 distinct dimensions: pattern-exists / exit-code-mapping /
//	           threshold-crossed / single-strike / driver-scope-isolation /
//	           thinking-converging / thinking-absent-fallback / edge-no-panic /
//	           detector-routing / no-cli-branch.
//
// 1:1 enforcement:
//
//	T1: predicate=5 (C424_001–C424_005), manual=0, unverifiable=0 → total AC=5 ✓
//	T2: predicate=5 (C424_006–C424_010), manual=0, unverifiable=0 → total AC=5 ✓
package cycle424

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// readTestdataFrame loads a real capture-pane snapshot from the panestream testdata
// directory under the repo root.
func readTestdataFrame(t *testing.T, root, relPath string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "go", "internal", "bridge", "panestream", "testdata", relPath))
	if err != nil {
		t.Fatalf("readTestdataFrame(%q): %v", relPath, err)
	}
	return string(b)
}

// ===================== T1 — repl-boot-timeout-driver-bench =====================

// TestC424_001_BootTimeoutPatternBenchable asserts BootTimeoutPattern constant exists
// and Benchable returns true for it. Compile check confirms the constant is exported;
// behavioral check confirms the closed-set gate recognises the boot-timeout class.
// RED: clihealth.BootTimeoutPattern / clihealth.Benchable absent or wrong → compile/fail.
func TestC424_001_BootTimeoutPatternBenchable(t *testing.T) {
	pat := clihealth.BootTimeoutPattern
	if pat == "" {
		t.Fatal("BootTimeoutPattern must be a non-empty string constant")
	}
	if !clihealth.Benchable(pat) {
		t.Errorf("Benchable(%q) = false, want true — boot-timeout pattern must be benchable", pat)
	}
	// Sanity: the existing rate_limit pattern must still be benchable (no regression).
	if !clihealth.Benchable("rate_limit") {
		t.Error("Benchable(\"rate_limit\") regressed to false")
	}
}

// TestC424_002_IsBootTimeoutExitCodeMapping asserts IsBootTimeoutExitCode is a pure function
// that returns true only for exit-80 (ExitREPLBootTimeout). Positive case: exit 80.
// Negative/edge cases: exits 81 (artifact timeout), 85 (unknown prompt), 0 (success), 1 (generic fail).
// RED: clihealth.IsBootTimeoutExitCode absent → compile error.
func TestC424_002_IsBootTimeoutExitCodeMapping(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{80, true},
		{81, false},
		{85, false},
		{0, false},
		{1, false},
		{127, false},
	}
	for _, c := range cases {
		got := clihealth.IsBootTimeoutExitCode(c.code)
		if got != c.want {
			t.Errorf("IsBootTimeoutExitCode(%d) = %v, want %v", c.code, got, c.want)
		}
	}
}

// TestC424_003_RepeatedStrikeBenchesDriver is the primary positive test: calling
// RecordBootStrike for the same driver ≥ DefaultBootBenchThreshold consecutive times
// must record a bench entry (benched=true returned on the Nth call). Validates the
// threshold guard: exactly at-threshold fires, below-threshold does not.
// RED: clihealth.Store.RecordBootStrike / clihealth.DefaultBootBenchThreshold absent → compile error.
func TestC424_003_RepeatedStrikeBenchesDriver(t *testing.T) {
	dir := t.TempDir()
	store := clihealth.NewStore(dir, func() time.Time { return time.Now() })
	driver := "codex-tmux"
	thresh := clihealth.DefaultBootBenchThreshold
	if thresh < 2 {
		t.Fatalf("DefaultBootBenchThreshold must be ≥ 2, got %d", thresh)
	}

	// Calls 1 through thresh-1 must NOT bench.
	for i := 1; i < thresh; i++ {
		benched, err := store.RecordBootStrike(driver)
		if err != nil {
			t.Fatalf("RecordBootStrike call %d: unexpected error: %v", i, err)
		}
		if benched {
			t.Errorf("RecordBootStrike call %d/%d: benched=true before threshold (want false)", i, thresh)
		}
	}

	// The Nth call (= threshold) MUST bench.
	benched, err := store.RecordBootStrike(driver)
	if err != nil {
		t.Fatalf("RecordBootStrike (threshold call): unexpected error: %v", err)
	}
	if !benched {
		t.Errorf("RecordBootStrike at threshold (%d): benched=false, want true — repeated boot-timeout must bench the driver", thresh)
	}

	// Verify the bench is now active in the store.
	active := store.Active()
	if _, ok := active[driver]; !ok {
		t.Errorf("after threshold strikes, Active() does not contain %q — bench not recorded in store", driver)
	}
}

// TestC424_004_SingleStrikeDoesNotBench is the load-bearing negative test:
// a single RecordBootStrike must NOT bench the driver. A transient boot failure
// must remain retryable; only persistent failures (≥ threshold) should bench.
// RED: clihealth.Store.RecordBootStrike absent → compile error.
func TestC424_004_SingleStrikeDoesNotBench(t *testing.T) {
	dir := t.TempDir()
	store := clihealth.NewStore(dir, func() time.Time { return time.Now() })
	driver := "claude-tmux"

	benched, err := store.RecordBootStrike(driver)
	if err != nil {
		t.Fatalf("RecordBootStrike: unexpected error: %v", err)
	}
	if benched {
		t.Errorf("RecordBootStrike (1 of %d): benched=true after single strike — single transient boot failure must NOT bench", clihealth.DefaultBootBenchThreshold)
	}

	// Active bench store must be empty after 1 strike.
	active := store.Active()
	if len(active) != 0 {
		t.Errorf("Active() = %v after single strike, want empty — bench must not be recorded until threshold", active)
	}
}

// TestC424_005_BootBenchIsDriverScoped asserts the boot bench is keyed by the DRIVER
// name ("codex-tmux"), not the CLI family ("codex"). Benching "codex-tmux" must NOT
// affect "codex" (headless) or "claude-tmux" (different driver). This is the
// anti-gaming predicate: family-scoped would wrongly block headless use of a CLI whose
// only failure is the tmux-REPL boot (Scout Finding 2).
// RED: clihealth.Store.RecordBootStrike / clihealth.DefaultBootBenchThreshold absent → compile error.
func TestC424_005_BootBenchIsDriverScoped(t *testing.T) {
	dir := t.TempDir()
	store := clihealth.NewStore(dir, func() time.Time { return time.Now() })
	driver := "codex-tmux"
	thresh := clihealth.DefaultBootBenchThreshold

	// Drive "codex-tmux" past the bench threshold.
	for i := 0; i < thresh; i++ {
		if _, err := store.RecordBootStrike(driver); err != nil {
			t.Fatalf("RecordBootStrike call %d: %v", i+1, err)
		}
	}

	// "codex-tmux" must be benched.
	active := store.Active()
	if _, ok := active[driver]; !ok {
		t.Fatalf("expected %q in Active() after %d strikes, got %v", driver, thresh, active)
	}

	// "codex" (headless family, different key) must NOT be benched.
	if _, ok := active["codex"]; ok {
		t.Errorf("Active()[\"codex\"] is set, but bench was for %q — boot bench must be driver-scoped, not family-scoped", driver)
	}

	// "claude-tmux" (different driver entirely) must NOT be benched.
	if _, ok := active["claude-tmux"]; ok {
		t.Errorf("Active()[\"claude-tmux\"] is set — bench for %q must not propagate to other drivers", driver)
	}
}

// ===================== T2 — ollama-liveness-strategy =====================

// TestC424_006_OllamaDetectorThinkingConverging is the primary positive test:
// OllamaDetector must classify LivenessConverging with higher confidence than
// DefaultDetector when the pane contains the "Thinking..." header but NO new
// content delta occurred between intervals (pure-thinking phase, chrome-only delta).
// DefaultDetector sees BusyButStagnant (no content lines) — OllamaDetector's
// "Thinking..." layer overrides to Converging (the model is actively processing).
// RED: panestream.NewOllamaDetector absent → compile error.
func TestC424_006_OllamaDetectorThinkingConverging(t *testing.T) {
	p := panestream.Profiles["ollama"]
	base := panestream.NewDefaultDetector(3)
	det := panestream.NewOllamaDetector(3)

	// Frame A: ollama prompt with "Thinking..." but no content lines yet.
	// PaneBusy=true (no "Send a message" idle placeholder → ollama is busy).
	thinkingFrame := "user@host /tmp % ollama run gemma4:latest\n>>> what is tmux?\nThinking...\n"

	// Prime both detectors on the initial thinking frame.
	base.Assess(thinkingFrame, p)
	det.Assess(thinkingFrame, p)

	// Feed the same frame again: no new content lines since the last interval.
	// "Thinking..." header is still present (model still processing).
	baseState, baseConf := base.Assess(thinkingFrame, p)
	ollamaState, ollamaConf := det.Assess(thinkingFrame, p)

	// OllamaDetector must classify as Converging — "Thinking..." proves the model is live.
	if ollamaState != panestream.LivenessConverging {
		t.Errorf("OllamaDetector on Thinking... frame: got %v, want LivenessConverging", ollamaState)
	}

	// OllamaDetector confidence must exceed DefaultDetector's (the token-layer uplift).
	if ollamaConf <= baseConf {
		t.Errorf("OllamaDetector confidence %v not > DefaultDetector %v (state=%v) on Thinking... frame; uplift required",
			ollamaConf, baseConf, baseState)
	}
}

// TestC424_007_OllamaDetectorNoThinkingFallsBack is the load-bearing negative test:
// when "Thinking..." is NOT present, OllamaDetector must produce byte-identical
// state+confidence to DefaultDetector on the same frame sequence. The ollama layer
// must be a strict composition — no side effects when the thinking signal is absent.
// RED: panestream.NewOllamaDetector absent → compile error.
func TestC424_007_OllamaDetectorNoThinkingFallsBack(t *testing.T) {
	p := panestream.Profiles["ollama"]
	base := panestream.NewDefaultDetector(3)
	det := panestream.NewOllamaDetector(3)

	// A frame with no "Thinking..." header — just a completed answer.
	noThinkingFrame := "user@host /tmp % ollama run gemma4:latest\n>>> what is tmux?\n*   tmux is a terminal multiplexer.\n*   It keeps sessions alive.\n>>> Send a message (/? for help)\n"

	for range 3 {
		base.Assess(noThinkingFrame, p)
		det.Assess(noThinkingFrame, p)
	}
	baseState, baseConf := base.Assess(noThinkingFrame, p)
	ollamaState, ollamaConf := det.Assess(noThinkingFrame, p)

	if ollamaState != baseState {
		t.Errorf("OllamaDetector (no Thinking...): state %v ≠ DefaultDetector %v — fallback must be byte-identical", ollamaState, baseState)
	}
	if ollamaConf != baseConf {
		t.Errorf("OllamaDetector (no Thinking...): conf %v ≠ DefaultDetector %v — fallback must be byte-identical", ollamaConf, baseConf)
	}
}

// TestC424_008_OllamaDetectorNoPanicEdgeCases asserts OllamaDetector never panics on
// edge/OOD inputs: empty pane, partial "Thinking" (no trailing "..."), pure chrome,
// and nil-equivalent whitespace-only frames. All must return a valid LivenessState.
// RED: panestream.NewOllamaDetector absent → compile error.
func TestC424_008_OllamaDetectorNoPanicEdgeCases(t *testing.T) {
	p := panestream.Profiles["ollama"]
	edgeCases := []struct {
		name  string
		frame string
	}{
		{"empty", ""},
		{"whitespace-only", "   \n  \n"},
		{"partial-thinking-header", "Thinking\n"},  // no trailing "..."
		{"thinking-mid-word", "DeepThinking...\n"}, // "Thinking..." is suffix, not the header
		{"pure-chrome", "⠋⠙⠹ processing...\n"},
		{"idle-placeholder-only", ">>> Send a message (/? for help)\n"},
	}
	validStates := map[panestream.LivenessState]bool{
		panestream.LivenessConverging:      true,
		panestream.LivenessBusyButStagnant: true,
		panestream.LivenessIdle:            true,
		panestream.LivenessHung:            true,
	}
	for _, tc := range edgeCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("OllamaDetector panicked on edge case %q: %v", tc.name, r)
				}
			}()
			det := panestream.NewOllamaDetector(3)
			det.Assess(tc.frame, p) // prime
			state, conf := det.Assess(tc.frame, p)
			if !validStates[state] {
				t.Errorf("edge case %q: invalid state %v (not in {Converging,Busy,Idle,Hung})", tc.name, state)
			}
			if conf < 0 || conf > 1.0 {
				t.Errorf("edge case %q: confidence %v out of [0,1]", tc.name, conf)
			}
		})
	}
}

// TestC424_009_DetectorForOllamaRoutesOllama asserts DetectorFor returns a detector
// that activates the "Thinking..." layer for the ollama profile — verified by the
// behavioral outcome: higher confidence than DefaultDetector on a thinking frame
// where no content delta occurred (the same condition C424_006 tests directly, but
// here via the registry path to confirm DetectorFor wires correctly).
// RED: panestream.DetectorFor returning NewDefaultDetector for "ollama" → conf not elevated.
func TestC424_009_DetectorForOllamaRoutesOllama(t *testing.T) {
	p := panestream.Profiles["ollama"]
	probe := panestream.DetectorFor(p)
	if probe == nil {
		t.Fatal("DetectorFor(ollama) = nil")
	}
	base := panestream.NewDefaultDetector(0)

	// Thinking frame: model is processing, no content delta expected.
	thinkingFrame := "user@host /tmp % ollama run gemma4:latest\n>>> what is tmux?\nThinking...\n"

	probe.Assess(thinkingFrame, p)
	base.Assess(thinkingFrame, p)

	_, baseConf := base.Assess(thinkingFrame, p)
	_, probeConf := probe.Assess(thinkingFrame, p)

	if probeConf <= baseConf {
		t.Errorf("DetectorFor(ollama) confidence %v not > DefaultDetector %v on Thinking... frame; registry must route to OllamaDetector", probeConf, baseConf)
	}
}

// TestC424_010_NoCliNameInStopReview asserts stopreview.go still contains no
// hardcoded CLI-name string literals after T1 and T2 changes. Pre-existing GREEN:
// the reviewer was already branch-free (cycle-423 C423_007). This predicate is the
// regression guard — adding an ollama DetectorFor branch in the reviewer violates
// the abstraction. Declared // acs-predicate: config-check per the single-source waiver
// (the only mechanical way to enforce the zero-CLI-branch design constraint).
func TestC424_010_NoCliNameInStopReview(t *testing.T) { // acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "bridge", "stopreview.go")
	for _, cli := range []string{"claude", "codex", "agy", "ollama"} {
		acsassert.FileNotContains(t, path, `"`+cli+`"`)
	}
}
