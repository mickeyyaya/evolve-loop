package core

import (
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

var (
	_ = (*cycleRun).recordAndBranch
	_ = (*cycleRun).selectNext
	_ = (*Orchestrator).registerMintedPhases
	_ = mintConfigsFrom
	_ = (*StateMachine).SpineSatisfiedUpTo
	_ = router.ClampPlanToFloorWith
)

func TestArchitectureSeams_FoundationsExist(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("../panetrust/panetrust.go")
	if err != nil {
		t.Fatalf("read panetrust source: %v", err)
	}
	if !strings.Contains(string(src), "func redactSecrets(") {
		t.Error("seam panetrust.redactSecrets missing/renamed — WS3-S1 (advisor prompt/response redaction) depends on it; reconcile ADR-0052 §seams")
	}
}
