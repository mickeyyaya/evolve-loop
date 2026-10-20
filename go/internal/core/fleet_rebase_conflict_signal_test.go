package core_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRecoverFromShipError_AFleetRebaseConflictPublishesItsPaths(t *testing.T) {
	t.Parallel()
	f := newConflictFixture(t, "lane line\n", nil)
	var mu sync.Mutex
	var got []signalcenter.Event
	c := signalcenter.New()
	c.Subscribe(func(e signalcenter.Event) {
		mu.Lock()
		defer mu.Unlock()
		if e.Code == core.CodeFleetRebaseConflict {
			got = append(got, e)
		}
	})
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip: f.ship, core.PhaseAudit: f.audit, core.PhaseBuild: f.build, core.PhaseTDD: f.tdd, core.PhaseDebugger: f.dbg,
	}), core.WithWorktreeProvisioner(fixedWorktree{dir: f.dir}), core.WithSignalCenter(c))

	_ = runCompositionCycle(t, o)
	c.Flush()

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || !strings.Contains(got[0].Fields["paths"], "shared.md") || got[0].Kind != signalcenter.KindShipWarning {
		t.Fatalf("conflict events = %+v, want one ship warning naming shared.md", got)
	}
	if m, ok := signalcenter.IsRegistered(core.CodeFleetRebaseConflict); !ok || m != signalcenter.ModuleOrchestrator {
		t.Errorf("IsRegistered(%q) = %q, %v, want the orchestrator module", core.CodeFleetRebaseConflict, m, ok)
	}
}
