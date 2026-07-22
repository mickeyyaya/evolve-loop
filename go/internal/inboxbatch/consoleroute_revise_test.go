package inboxbatch

// consoleroute_revise_test.go — RED contracts for the architect-review
// revisions (ADR-0074): (F4) route:"lane" override provenance clamp — an
// agent-autofiled item may not override a protected derivation; (F5) the
// derivation scans ALL tokens of a files entry, not just the first; plus the
// id-resolver the plan-time gate (fleet.TodosFromTriage) consumes.

import (
	"os"
	"path/filepath"
	"testing"
)

// F4: autofiled provenance (injected_by set) ignores the lane override when a
// protected surface is declared — clamp-parity with ADR-0073: agent-authored
// fields cannot widen agent authority; only operator-authored items may force
// lane dispatch of a protected-surface task.
func TestConsoleRouted_AutofiledLaneOverrideIgnored(t *testing.T) {
	it := Item{ID: "x", Route: "lane", InjectedBy: "chronicle-escalation",
		Files: []string{"go/internal/guards/role.go"}}
	routed, reason := ConsoleRouted(it, protectedStub("go/internal/guards/role.go"))
	if !routed {
		t.Fatal("autofiled item must not lane-override a protected derivation")
	}
	if reason == "" {
		t.Fatal("clamped override must still carry the derivation reason")
	}
}

// F4 boundary: an autofiled item with route:"lane" and NO protected surface
// stays dispatchable — the clamp binds only where the derivation fires.
func TestConsoleRouted_AutofiledLaneNoProtectedStaysDispatchable(t *testing.T) {
	it := Item{ID: "x", Route: "lane", InjectedBy: "retrofile", Files: []string{"go/internal/other/ok.go"}}
	if routed, _ := ConsoleRouted(it, protectedStub("go/internal/guards/role.go")); routed {
		t.Fatal("clamp must not route items with no protected surface")
	}
}

// F5: a protected path ANYWHERE in a files entry routes the item — real items
// write "(see go/internal/guards/role.go)" and similar shapes.
func TestConsoleRouted_ProtectedPathAnyToken(t *testing.T) {
	it := Item{ID: "x", Files: []string{"allowance fix (see go/internal/guards/role.go)"}}
	routed, _ := ConsoleRouted(it, protectedStub("go/internal/guards/role.go"))
	if !routed {
		t.Fatal("protected path in a later token must still route")
	}
}

// RoutedResolver adapts a loaded inbox dir into the id→(routed, reason)
// closure fleet.TodosFromTriage consumes. Unknown ids (scout-originated work
// with no inbox item) are dispatchable.
func TestRoutedResolver_ClassifiesByID(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.json", `{"id":"lane-work"}`)
	write("b.json", `{"id":"operator-work","route":"console-manual"}`)

	resolve := RoutedResolver(dir, nil)
	if routed, _ := resolve("lane-work"); routed {
		t.Error("plain item must be dispatchable")
	}
	if routed, reason := resolve("operator-work"); !routed || reason == "" {
		t.Error("console item must resolve routed with a reason")
	}
	if routed, _ := resolve("scout-invented-task"); routed {
		t.Error("unknown id must be dispatchable (scout-originated work)")
	}
}
