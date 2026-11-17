package main

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
)

func productionLocate(t *testing.T, root string) func(orchestrator.Lane) (int, bool) {
	t.Helper()
	b, _, _ := supplyCoordinator(t, root)
	return b.orchestratorPool(oversightFleet(orchestrator.StageEnforce), nil, 1).Locate
}

func TestLaneLocator_ANewLifeOfATodoNeverTakesTheCycleOfItsEarlierLife(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pinLaneCycle(t, root, 61, "A")
	locate := productionLocate(t, root)

	firstLife := orchestrator.Lane{Key: "A", Scope: []string{"A"}, Started: time.Now().Add(-time.Hour)}
	if got, ok := locate(firstLife); !ok || got != 61 {
		t.Fatalf("locate of the first life = %d, %v, want 61", got, ok)
	}

	secondLife := orchestrator.Lane{Key: "A", Scope: []string{"A"}, Started: time.Now().Add(-time.Minute)}
	if got, ok := locate(secondLife); ok {
		t.Fatalf("locate of the new life = %d, true, want none: cycle 61 is newer than the new start but it belongs to the first life", got)
	}

	pinLaneCycle(t, root, 62, "A")
	if got, ok := locate(secondLife); !ok || got != 62 {
		t.Errorf("locate of the new life once its own cycle exists = %d, %v, want 62", got, ok)
	}
}

func TestLaneLocator_AFirstLifeStillFindsItsCycleAndAnotherTodoKeepsItsOwn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pinLaneCycle(t, root, 50, "B")
	pinLaneCycle(t, root, 61, "A")
	locate := productionLocate(t, root)

	started := time.Now().Add(-time.Hour)
	if got, ok := locate(orchestrator.Lane{Key: "A", Scope: []string{"A"}, Started: started}); !ok || got != 61 {
		t.Errorf("locate of A = %d, %v, want 61", got, ok)
	}
	if got, ok := locate(orchestrator.Lane{Key: "B", Scope: []string{"B"}, Started: started}); !ok || got != 50 {
		t.Errorf("locate of B = %d, %v, want 50: the floor of A does not apply to B", got, ok)
	}
}
