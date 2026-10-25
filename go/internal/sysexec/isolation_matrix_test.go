package sysexec

import (
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
)

func TestIsolationMatrix_N2_NoLLMSpawnOutsideTheBridgeLaunchAPI(t *testing.T) {
	t.Parallel()
	spawns := treeLLMSpawns(t)
	var failure error
	if len(spawns) > 0 {
		failure = fmt.Errorf("%d LLM spawn(s) outside internal/bridge's launch API, first %s", len(spawns), spawns[0])
	}
	note, err := isolation.Judge("N2", failure)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note)
}
