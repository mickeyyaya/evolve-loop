package inboxbatch

import "testing"

func TestConsoleRouted_RouteLaneRelaxesAPipelineKindForAnOperatorItem(t *testing.T) {
	item := Item{ID: "x", Kind: "pipeline-repair", Route: RouteLaneValue}

	if routed, why := ConsoleRouted(item, nil); routed {
		t.Errorf("ConsoleRouted = (true, %q); route %q relaxes the pipeline-kind derivation for an operator-authored item", why, RouteLaneValue)
	}
}
