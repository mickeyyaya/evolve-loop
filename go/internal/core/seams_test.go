package core

import (
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// TestArchitectureSeams_FoundationsExist is a standing guard for ADR-0052
// advisor-maximization (WS0-S1). The design hooks a set of load-bearing seams;
// a concurrent refactor that renames or removes one must fail HERE, at the
// start of the advisor work, rather than midway through an implementation slice.
//
// The package-level references below are the real guard: each names a seam by
// its exact identity (receiver + method/func name via a method expression), so
// a rename or signature-incompatible move breaks the build. The single runtime
// check covers panetrust.redactSecrets, which is unexported and so cannot be
// referenced across the package boundary.
var (
	_ = (*cycleRun).recordAndBranch          // WS2-S0 post-scout re-plan hook site
	_ = (*cycleRun).selectNext               // next-phase selection (re-plan precedes it)
	_ = (*Orchestrator).registerMintedPhases // WS2-S6 mint-registration idempotency target
	_ = mintConfigsFrom                      // WS1-S2 recursion-guard / denylist target
	_ = writeRubricLines                     // WS5-S1 recipe-projection pattern
	_ = (*StateMachine).SpineSatisfiedUpTo   // floor's artifact-backed spine check
	_ = router.ClampPlanToFloorWith          // the sole trust boundary (integrity floor)
)

func TestArchitectureSeams_FoundationsExist(t *testing.T) {
	// panetrust.redactSecrets is unexported; WS3-S1 will route captured advisor
	// prompts/responses through it (likely via a new exported wrapper). Guard
	// the seam name so a rename surfaces before WS3-S1 wires it. (The compile-
	// time references above already guarantee the in-package + exported seams.)
	src, err := os.ReadFile("../panetrust/panetrust.go")
	if err != nil {
		t.Fatalf("read panetrust source: %v", err)
	}
	if !strings.Contains(string(src), "func redactSecrets(") {
		t.Error("seam panetrust.redactSecrets missing/renamed — WS3-S1 (advisor prompt/response redaction) depends on it; reconcile ADR-0052 §seams")
	}
}
