package inboxbatch

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestConsoleRouted_AutofiledLaneNoProtectedStaysDispatchable(t *testing.T) {
	it := Item{ID: "x", Route: "lane", InjectedBy: "retrofile", Files: []string{"go/internal/other/ok.go"}}
	if routed, _ := ConsoleRouted(it, protectedStub("go/internal/guards/role.go")); routed {
		t.Fatal("clamp must not route items with no protected surface")
	}
}

func TestConsoleRouted_ProtectedPathAnyToken(t *testing.T) {
	it := Item{ID: "x", Files: []string{"allowance fix (see go/internal/guards/role.go)"}}
	routed, _ := ConsoleRouted(it, protectedStub("go/internal/guards/role.go"))
	if !routed {
		t.Fatal("protected path in a later token must still route")
	}
}

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
