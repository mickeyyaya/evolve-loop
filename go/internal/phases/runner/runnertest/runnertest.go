package runnertest

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable/gatesignal"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
)

const (
	productionSettleRetries  = 15
	productionSettleInterval = 200 * time.Millisecond
)

func FreeSleep(d time.Duration) { _ = d }

type emptyArtifactVerifier struct{}

func (emptyArtifactVerifier) VerifyForClassification(_ gatesignal.Check, phase string, roots phasecontract.Roots) (deliverable.Result, error) {
	return deliverable.Result{Phase: phase, ArtifactPath: roots.DispatchedArtifact, Violations: []deliverable.Violation{{Code: deliverable.CodeEmptyArtifact}}}, nil
}

func EmptyArtifactVerifier() runner.ContractVerifier { return emptyArtifactVerifier{} }

type SleepRecorder struct{ slept []time.Duration }

func (r *SleepRecorder) Sleep(d time.Duration) { r.slept = append(r.slept, d) }

func (r *SleepRecorder) Calls() []time.Duration { return append([]time.Duration(nil), r.slept...) }

func (r *SleepRecorder) AssertProductionLadder(t testing.TB) {
	t.Helper()
	if len(r.slept) != productionSettleRetries {
		t.Errorf("settle sleeps=%d, want %d", len(r.slept), productionSettleRetries)
	}
	for i, d := range r.slept {
		if d != productionSettleInterval {
			t.Errorf("settle sleep %d=%v, want %v", i, d, productionSettleInterval)
		}
	}
}
