package core

import (
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// TestArchitectureSeams_FoundationsExist guards a set of load-bearing seams
// against rename or removal: the package-level references below name each
// seam by its exact identity (receiver + method/func name via a method
// expression), so a rename or signature-incompatible move breaks the build.
var (
	_ = (*cycleRun).recordAndBranch          // post-scout re-plan hook site
	_ = (*cycleRun).selectNext               // next-phase selection (re-plan precedes it)
	_ = (*Orchestrator).registerMintedPhases // mint-registration idempotency target
	_ = mintConfigsFrom                      // recursion-guard / denylist target
	_ = (*StateMachine).SpineSatisfiedUpTo   // floor's artifact-backed spine check
	_ = router.ClampPlanToFloorWith          // the sole trust boundary (integrity floor)
)

func TestArchitectureSeams_FoundationsExist(t *testing.T) {
	// panetrust.redactSecrets is unexported, so it cannot be referenced across
	// the package boundary like the seams above; this runtime check guards its
	// name instead.
	src, err := os.ReadFile("../panetrust/panetrust.go")
	if err != nil {
		t.Fatalf("read panetrust source: %v", err)
	}
	if !strings.Contains(string(src), "func redactSecrets(") {
		t.Error("seam panetrust.redactSecrets missing/renamed — WS3-S1 (advisor prompt/response redaction) depends on it; reconcile ADR-0052 §seams")
	}
}
