//go:build acs

// Package cycle425 materializes the cycle-425 acceptance criteria for two tasks:
// production wiring of the boot-timeout bench store into adapters.NewDefault (T1)
// and the agy liveness spinner strategy composed over DefaultDetector (T2).
//
// Goal: close the dead latency levers — the BootTimeoutStore wiring gap (no strikes
// ever recorded in production; engine.go:455 is a no-op with nil store) and the
// missing per-CLI strategy for agy (falls through to bare DefaultDetector, missing
// the ⣯ Generating... / esc to cancel affordance as a Converging signal).
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	wire-boottimeout-store-production (T1 — Medium):
//	  AC1  adapters.NewDefault sets BootTimeoutStore non-nil (positive)            → C425_001
//	  AC2  Single RecordBootStrike does NOT bench (negative/threshold guard)       → C425_002
//	  AC3  exit-81 (artifact timeout) is NOT classified as boot timeout (edge)     → C425_003
//	  AC4  engine+clihealth+llmroute suites green (regression)                    → manual+checklist
//
//	agy-liveness-spinner-strategy (T2 — Small):
//	  AC5  AgyDetector generating frame ⇒ LivenessConverging conf≥0.9 (positive)  → C425_004
//	  AC6  AgyDetector answer frame ⇒ DefaultDetector byte-identical (negative)   → C425_005
//	  AC7  DetectorFor("agy") → *AgyDetector + conf-uplift on generating frame    → C425_006
//	  AC8  stopreview.go has no CLI-name literals after T1+T2 (regression)        → C425_007
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C425_002 (1 strike < threshold → not benched), C425_005 (answer frame → default fallback)
//	Edge/OOD:  C425_003 (exit-81 ≠ boot timeout), C425_006 (type assertion + registry routing)
//	Semantic:  7 distinct dimensions: production-wiring / threshold-guard / exit-code-map /
//	           spinner-converging / answer-fallback / registry-route / no-cli-literal.
//
// 1:1 enforcement:
//
//	T1: predicate=3 (C425_001–C425_003) + manual+checklist=1 (AC4) → total=4 ✓
//	T2: predicate=4 (C425_004–C425_007) + manual=0                 → total=4 ✓
package cycle425

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	adapterbridge "github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
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

// ===================== T1 — wire-boottimeout-store-production =====================

// TestC425_001_BootTimeoutStoreWiredInProduction is the primary positive test:
// adapters.NewDefault(projectRoot) must return an Adapter whose BootTimeoutStoreWired()
// method returns true, confirming the boot-timeout bench store is injected into the
// production engine deps so exit-80 strikes are recorded (engine.go:455).
// RED: adapterbridge.Adapter.BootTimeoutStoreWired() absent → compile error.
func TestC425_001_BootTimeoutStoreWiredInProduction(t *testing.T) {
	root := acsassert.SetupTempProject(t)
	adapter := adapterbridge.NewDefault(root)
	if !adapter.BootTimeoutStoreWired() {
		t.Errorf("adapters.NewDefault(%q).BootTimeoutStoreWired() = false; "+
			"production Adapter must set a non-nil BootTimeoutStore so engine.go:455 can record exit-80 strikes", root)
	}
}

// TestC425_002_SingleStrikeDoesNotBench is the load-bearing negative test:
// a single RecordBootStrike call must NOT bench the driver — one transient boot
// failure must remain retryable. Only consecutive failures at the threshold bench.
// Pre-existing GREEN (clihealth mechanism landed cycle 424); kept as regression guard
// for this cycle's production wiring.
func TestC425_002_SingleStrikeDoesNotBench(t *testing.T) {
	dir := t.TempDir()
	store := clihealth.NewStore(dir, func() time.Time { return time.Now() })
	driver := "claude-tmux"

	benched, err := store.RecordBootStrike(driver)
	if err != nil {
		t.Fatalf("RecordBootStrike: unexpected error: %v", err)
	}
	if benched {
		t.Errorf("RecordBootStrike (1 of %d): benched=true — a single transient boot failure must NOT bench the driver",
			clihealth.DefaultBootBenchThreshold)
	}
	if active := store.Active(); len(active) != 0 {
		t.Errorf("Active() = %v after single strike, want empty — bench must not be recorded until threshold", active)
	}
}

// TestC425_003_WrongExitCodeNotBootTimeout asserts that exit code 81
// (ExitREPLArtifactTimeout) is NOT classified as a boot timeout. A misclassification
// here would cause artifact-timeout dispatches to accumulate spurious boot strikes
// and bench a healthy driver after 2 artifact timeouts.
// Pre-existing GREEN (IsBootTimeoutExitCode correct since cycle 424); kept as regression guard.
func TestC425_003_WrongExitCodeNotBootTimeout(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{80, true},  // ExitREPLBootTimeout — the only boot timeout
		{81, false}, // ExitREPLArtifactTimeout — must NOT bench as boot
		{85, false}, // prompt escalation — must NOT bench as boot
		{0, false},  // success
		{1, false},  // generic failure
	}
	for _, c := range cases {
		got := clihealth.IsBootTimeoutExitCode(c.code)
		if got != c.want {
			t.Errorf("IsBootTimeoutExitCode(%d) = %v, want %v", c.code, got, c.want)
		}
	}
}

// ===================== T2 — agy-liveness-spinner-strategy =====================

// TestC425_004_AgyDetectorGeneratingFrameConverging is the primary positive test:
// AgyDetector must classify LivenessConverging with confidence ≥ 0.9 on agy's
// generating frame (⣯ Generating... + esc to cancel affordance) when no new
// content delta occurred between intervals. DefaultDetector on the same frame
// pair would classify BusyButStagnant (no content lines) — AgyDetector's spinner
// layer overrides because the affordance proves the model is actively generating.
// RED: panestream.NewAgyDetector absent → compile error.
func TestC425_004_AgyDetectorGeneratingFrameConverging(t *testing.T) {
	root := acsassert.RepoRoot(t)
	p := panestream.Profiles["agy"]
	det := panestream.NewAgyDetector(3)

	thinkingFrame := readTestdataFrame(t, root, "agy/thinking.txt")
	det.Assess(thinkingFrame, p) // prime (establish baseline)

	state, conf := det.Assess(thinkingFrame, p)
	if state != panestream.LivenessConverging {
		t.Errorf("AgyDetector on ⣯ Generating... frame: got %v, want LivenessConverging "+
			"(spinner affordance proves model is live; must not classify BusyButStagnant)", state)
	}
	if conf < 0.9 {
		t.Errorf("AgyDetector on generating frame: conf = %.2f, want ≥ 0.9 "+
			"(high-confidence signal from explicit spinner affordance)", conf)
	}
}

// TestC425_005_AgyDetectorAnswerFrameDefaultParity is the load-bearing negative test:
// when the ⣯ Generating... affordance is ABSENT (answer frame, model already replied),
// AgyDetector must produce byte-identical state+confidence to DefaultDetector on the
// same frame sequence. The agy layer is a strict composition — no side effects when
// the generating signal is absent. Prevents gaming by overriding non-generating frames.
// RED: panestream.NewAgyDetector absent → compile error.
func TestC425_005_AgyDetectorAnswerFrameDefaultParity(t *testing.T) {
	root := acsassert.RepoRoot(t)
	p := panestream.Profiles["agy"]
	base := panestream.NewDefaultDetector(3)
	det := panestream.NewAgyDetector(3)

	answerFrame := readTestdataFrame(t, root, "agy/answer.txt")
	for range 3 {
		base.Assess(answerFrame, p)
		det.Assess(answerFrame, p)
	}
	baseState, baseConf := base.Assess(answerFrame, p)
	agyState, agyConf := det.Assess(answerFrame, p)

	if agyState != baseState {
		t.Errorf("AgyDetector (answer frame, no ⣯ Generating...): state %v ≠ DefaultDetector %v — "+
			"fallback must be byte-identical when spinner is absent", agyState, baseState)
	}
	if agyConf != baseConf {
		t.Errorf("AgyDetector (answer frame, no ⣯ Generating...): conf %.2f ≠ DefaultDetector %.2f — "+
			"fallback must be byte-identical when spinner is absent", agyConf, baseConf)
	}
}

// TestC425_006_DetectorForAgyRoutesToAgyDetector asserts that DetectorFor routes
// the "agy" profile to *AgyDetector (not DefaultDetector). Verified by:
// (a) type assertion — the concrete type must be *panestream.AgyDetector; and
// (b) behavioral check — higher confidence than DefaultDetector on the generating
// frame confirms the spinner layer is active via the registry path.
// RED: panestream.AgyDetector type absent → compile error;
//
//	agy still routes to DefaultDetector → behavioral failure (conf not elevated).
func TestC425_006_DetectorForAgyRoutesToAgyDetector(t *testing.T) {
	root := acsassert.RepoRoot(t)
	p := panestream.Profiles["agy"]
	probe := panestream.DetectorFor(p)
	if probe == nil {
		t.Fatal("DetectorFor(agy) = nil")
	}

	// Type assertion: registry must return *AgyDetector.
	if _, ok := probe.(*panestream.AgyDetector); !ok {
		t.Errorf("DetectorFor(agy) = %T, want *panestream.AgyDetector — "+
			"agy must be registered in DetectorFor, not fall through to default", probe)
	}

	// Behavioral check: DetectorFor(agy) yields higher confidence than DefaultDetector
	// on the generating frame (conf uplift proves AgyDetector layer is active).
	base := panestream.NewDefaultDetector(0)
	thinkingFrame := readTestdataFrame(t, root, "agy/thinking.txt")
	probe.Assess(thinkingFrame, p)
	base.Assess(thinkingFrame, p)
	_, probeConf := probe.Assess(thinkingFrame, p)
	_, baseConf := base.Assess(thinkingFrame, p)
	if probeConf <= baseConf {
		t.Errorf("DetectorFor(agy) conf %.2f not > DefaultDetector %.2f on generating frame; "+
			"registry must route to AgyDetector (spinner layer must uplift confidence)", probeConf, baseConf)
	}
}

// TestC425_007_NoCliNameInStopReview asserts stopreview.go still contains no
// hardcoded CLI-name string literals after T1 and T2 changes. The reviewer must
// remain branch-free — zero per-CLI logic — as pinned by C424_010 and the
// ADR-0047 projection seam. Adding AgyDetector to DetectorFor must NOT tempt an
// agy branch into the reviewer.
// Pre-existing GREEN (C424_010 guard); this predicate is the regression continuation.
// Declared // acs-predicate: config-check per the single-source waiver.
func TestC425_007_NoCliNameInStopReview(t *testing.T) { // acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "bridge", "stopreview.go")
	for _, cli := range []string{"claude", "codex", "agy", "ollama"} {
		acsassert.FileNotContains(t, path, `"`+cli+`"`)
	}
}
