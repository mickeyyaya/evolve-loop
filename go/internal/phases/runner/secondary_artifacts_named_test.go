package runner

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type fakeSecondaryHooks struct {
	Hooks
	paths []string
}

func (f fakeSecondaryHooks) SecondaryArtifacts(_ core.PhaseRequest) []string { return f.paths }

func TestSecondaryArtifactsProvider_FeedsResolutionGlue(t *testing.T) {
	want := []string{filepath.Join("ws", "disposition.json")}
	var _ SecondaryArtifactsProvider = fakeSecondaryHooks{} // named interface binding

	got := secondaryArtifacts(fakeSecondaryHooks{paths: want}, core.PhaseRequest{})
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("provider hook not threaded: got %v want %v", got, want)
	}
	if got := secondaryArtifacts(fakeSecondaryHooks{}.Hooks, core.PhaseRequest{}); got != nil {
		t.Errorf("a Hooks without the optional interface must yield nil (legacy dispatch), got %v", got)
	}
}
