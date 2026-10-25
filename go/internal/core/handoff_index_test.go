package core

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
)

func TestHandoffAsksTravelInTheIndex(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-3")
	writeIndexFixture(t, filepath.Join(ws, "handoff-"+request.PairTDDBuild+".json"), `{"summary":"two asks for the build","asks":[{"id":"a1"}]}`)
	var warn strings.Builder
	writeInputIndex(root, ws, 3, PhaseBuild, &warn)
	idx, err := inputindex.Read(ws)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range idx.Entries {
		found = found || e.Kind == "handoff-asks" && e.Path == ".evolve/runs/cycle-3/handoff-"+request.PairTDDBuild+".json" && e.Summary == "two asks for the build"
	}
	if !found {
		t.Fatalf("the asks of the tdd agent must reach the build through its index, got %+v", idx.Entries)
	}
	rejection := enforcedHandoffRejection(string(PhaseBuild), request.PairTDDBuild, handoff.Check{Outcome: "unmet", Unmet: []string{"a1"}})
	if !strings.Contains(rejection.Remediation, "the handoff-asks entry of inputs/index.json") {
		t.Fatalf("the correction must send the agent to the index entry, not to the raw output of the earlier agent: %q", rejection.Remediation)
	}
}
