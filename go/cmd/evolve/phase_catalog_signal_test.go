package main

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func routingDefects(events []signalcenter.Event) []string {
	var reasons []string
	for _, e := range events {
		if e.Code == phasespec.CodeRoutingDefect {
			reasons = append(reasons, e.Reason)
		}
	}
	return reasons
}

func TestLoadPhaseCatalog_EmitsEachCatalogDefectAsTheRegisteredCode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeUserPhase(t, root, "ghost-check", map[string]any{"name": "ghost-check", "optional": true, "archetype": "evaluate"})
	var got []signalcenter.Event
	_, _, _ = loadPhaseCatalog(root, filepath.Join(root, "absent-registry.json"), prompts.NewFromFS(fstest.MapFS{}), func(e signalcenter.Event) { got = append(got, e) })
	reasons := strings.Join(routingDefects(got), "\n")
	if !strings.Contains(reasons, "ghost-check") || !strings.Contains(reasons, "builtin registry load failed") {
		t.Fatalf("the demoted spec and the registry load failure must each be a coded routing defect, got %q", reasons)
	}
	if len(routingDefects(got)) != len(got) {
		t.Fatalf("every catalog warning carries the routing-defect code: %+v", got)
	}
}

func TestWireOrchestrator_AnUnresolvableUserAnchorReachesTheSignalCenter(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeUserPhase(t, root, "orphan-check", map[string]any{"name": "orphan-check", "optional": true, "archetype": "evaluate", "after": "nowhere"})
	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	defer d.CloseSignals()
	d.Signals.Flush()
	if reasons := strings.Join(routingDefects(d.Signals.Recent()), "\n"); !strings.Contains(reasons, "orphan-check anchor nowhere") {
		t.Fatalf("an unresolvable anchor must reach the cycle's Signal Center, not only stderr; got %q", reasons)
	}
}

func TestWireRoutingPlanAdvisor_ARefusedTableIsAnErrorAndTheCatalogDefectsReachTheConsole(t *testing.T) {
	root, _ := wiringRoot(t, `{"cli_routing":{"clis":["agy"],"default":["agy"]}}`)
	writeUserPhase(t, root, "ghost-check", map[string]any{"name": "ghost-check", "optional": true, "archetype": "evaluate"})
	var console strings.Builder
	adv, err := wireRoutingPlanAdvisor(root, &console)
	if err == nil || adv != nil || !strings.Contains(err.Error(), "refuses to route") {
		t.Fatalf("a refused routing table must fail the advisor wiring: adv=%v err=%v", adv, err)
	}
	if out := console.String(); !strings.Contains(out, "WARN "+string(phasespec.CodeRoutingDefect)) || !strings.Contains(out, "ghost-check") {
		t.Fatalf("`evolve routing-plan` must print each catalog defect as the coded line on its console, got %q", out)
	}
}
