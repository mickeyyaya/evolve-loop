package signalcenter

// render_test.go — RenderCodes projects the registry into the markdown that
// docs/architecture/signal-codes.md carries between its GENERATED markers
// (design §5.4, S2). One source (RegisterCode), one projection. The pure
// projection is tested on a fixed snapshot: the live registry is mutated by
// parallel registry tests, so two live renders may legitimately differ.

import (
	"strings"
	"testing"
)

func TestRenderCodes_OneTablePerModuleSortedAndDeterministic(t *testing.T) {
	t.Parallel()
	snapshot := map[Module][]CodeDoc{
		ModuleShip:         {{Code: "SHIP_GIT_IO", Doc: "git I/O"}, {Code: "SHIP_ARGS", Doc: "bad args"}},
		ModuleOrchestrator: {{Code: "ORCHESTRATOR_QUOTA_PAUSED", Doc: "paused"}},
	}
	out := renderCodes(snapshot)
	if out != renderCodes(snapshot) {
		t.Fatal("the projection of one snapshot is deterministic")
	}
	want := "### orchestrator\n\n| Code | Meaning |\n|---|---|\n| `ORCHESTRATOR_QUOTA_PAUSED` | paused |\n\n" +
		"### ship\n\n| Code | Meaning |\n|---|---|\n| `SHIP_ARGS` | bad args |\n| `SHIP_GIT_IO` | git I/O |\n\n"
	if out != want {
		t.Errorf("one table per module, modules and codes sorted:\n got %q\nwant %q", out, want)
	}
	if renderCodes(nil) != "" {
		t.Error("an empty registry renders nothing")
	}
}

func TestRenderCodes_LiveRegistryCarriesTheBuiltInCodes(t *testing.T) {
	t.Parallel()
	out := RenderCodes()
	var panicked CodeDoc
	for _, d := range RegisteredCodes()[ModuleSignalCenter] {
		if d.Code == CodeListenerPanicked {
			panicked = d
		}
	}
	row := "| `" + string(CodeListenerPanicked) + "` | " + panicked.Doc + " |\n"
	if !strings.Contains(out, "### signalcenter\n\n| Code | Meaning |\n|---|---|\n") || !strings.Contains(out, row) {
		t.Errorf("the live projection lists the built-in codes under their module:\n%s", out)
	}
}
