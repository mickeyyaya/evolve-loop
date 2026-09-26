package inboxbatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func protectedStub(protected ...string) func(string) bool {
	set := map[string]bool{}
	for _, p := range protected {
		set[p] = true
	}
	return func(path string) bool { return set[path] }
}

func TestConsoleRouted_RoutePrefixConsole(t *testing.T) {
	for _, route := range []string{"console-manual", "console-salvage", "CONSOLE-MANUAL", " console-manual "} {
		routed, reason := ConsoleRouted(Item{ID: "x", Route: route}, nil)
		if !routed {
			t.Errorf("route %q must console-route the item", route)
		}
		if !strings.Contains(reason, "route:") {
			t.Errorf("reason must carry the route provenance, got %q", reason)
		}
	}
}

func TestConsoleRouted_ProtectedFilesAutoRoute(t *testing.T) {
	it := Item{ID: "x", Files: []string{"go/internal/other/ok.go", "go/internal/guards/role.go"}}
	routed, reason := ConsoleRouted(it, protectedStub("go/internal/guards/role.go"))
	if !routed {
		t.Fatal("item declaring a protected fix surface must console-route")
	}
	if !strings.Contains(reason, "go/internal/guards/role.go") {
		t.Errorf("reason must name the protected path, got %q", reason)
	}
}

func TestConsoleRouted_AnnotatedFileEntryFirstToken(t *testing.T) {
	it := Item{ID: "x", Files: []string{"go/internal/guards/role.go (implement the documented allowance)"}}
	routed, _ := ConsoleRouted(it, protectedStub("go/internal/guards/role.go"))
	if !routed {
		t.Fatal("annotated files entry must still match on its leading path token")
	}
}

// The override wins only over a directory-scope derivation; a declared protected file still routes.
func TestConsoleRouted_ExplicitLaneOverrideWins(t *testing.T) {
	it := Item{ID: "x", Route: "lane", Files: []string{"go/internal/guards/"}}
	if routed, _ := ConsoleRouted(it, protectedStub("go/internal/guards/")); routed {
		t.Fatal("route:lane must override a directory-scope derivation")
	}
	it.Files = []string{"go/internal/guards/role.go"}
	if routed, _ := ConsoleRouted(it, protectedStub("go/internal/guards/role.go")); !routed {
		t.Fatal("route:lane must not relax a declared protected file")
	}
}

func TestConsoleRouted_NilPredicateRouteFieldStillBinds(t *testing.T) {
	if routed, _ := ConsoleRouted(Item{ID: "x", Route: "console-manual"}, nil); !routed {
		t.Fatal("nil predicate must not disable explicit route routing")
	}
	if routed, _ := ConsoleRouted(Item{ID: "x", Files: []string{"a.go"}}, nil); routed {
		t.Fatal("nil predicate + no route field must stay dispatchable")
	}
}

func TestPartitionConsole_SplitsWithReasons(t *testing.T) {
	items := []Item{
		{ID: "a"},
		{ID: "b", Route: "console-manual"},
		{ID: "c", Files: []string{"go/internal/guards/role.go"}},
		{ID: "d"},
	}
	disp, console, reasons := PartitionConsole(items, protectedStub("go/internal/guards/role.go"))
	if len(disp) != 2 || disp[0].ID != "a" || disp[1].ID != "d" {
		t.Fatalf("dispatchable = %+v, want [a d]", disp)
	}
	if len(console) != 2 || console[0].ID != "b" || console[1].ID != "c" {
		t.Fatalf("console = %+v, want [b c]", console)
	}
	if len(reasons) != 2 || !strings.Contains(reasons[0], "b") || !strings.Contains(reasons[1], "c") {
		t.Fatalf("reasons must name each routed item, got %v", reasons)
	}
}

func TestLoadDir_RouteFieldParsedAndSanitized(t *testing.T) {
	dir := t.TempDir()
	body := `{"id":"r1","route":"console-manual\u0007extra"}`
	if err := os.WriteFile(filepath.Join(dir, "r1.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	items, warns, err := LoadDir(dir)
	if err != nil || len(items) != 1 {
		t.Fatalf("LoadDir: items=%d err=%v", len(items), err)
	}
	if !strings.HasPrefix(items[0].Route, "console-manual") || strings.ContainsRune(items[0].Route, 0x07) {
		t.Errorf("route must be parsed and control-char-sanitized, got %q", items[0].Route)
	}
	if len(warns) == 0 {
		t.Error("sanitization must be loud (warning expected)")
	}
}

func TestConsoleRouted_PipelineKindRoutesConsole(t *testing.T) {
	for _, kind := range []string{"pipeline-repair", "pipeline-integrity", "Pipeline-Repair", " pipeline-repair "} {
		routed, reason := ConsoleRouted(Item{ID: "x", Kind: kind}, nil)
		if !routed {
			t.Errorf("kind %q is pipeline-integrity work and must console-route", kind)
		}
		if !strings.Contains(reason, "kind:pipeline-") {
			t.Errorf("reason must carry the kind provenance, got %q", reason)
		}
	}
	for _, kind := range []string{"bug", "feature", "sweep", "", "loop-reliability"} {
		if routed, reason := ConsoleRouted(Item{ID: "x", Kind: kind}, nil); routed {
			t.Errorf("kind %q is lane work, got routed (%s)", kind, reason)
		}
	}
	it := Item{ID: "x", Kind: "pipeline-repair", Files: []string{"go/internal/guards/role.go"}}
	if _, reason := ConsoleRouted(it, protectedStub("go/internal/guards/role.go")); !strings.HasPrefix(reason, "kind:pipeline-repair") {
		t.Errorf("the kind reason outranks the files derivation, got %q", reason)
	}
	if _, reason := ConsoleRouted(Item{ID: "x", Kind: "pipeline-repair", Route: "console-manual"}, nil); !strings.HasPrefix(reason, "route:") {
		t.Errorf("the explicit route outranks the kind, got %q", reason)
	}
}

func TestConsoleRouted_PipelineKindHonorsOperatorLaneOverrideOnly(t *testing.T) {
	if routed, _ := ConsoleRouted(Item{ID: "x", Kind: "pipeline-repair", Route: "lane"}, nil); routed {
		t.Error("operator-authored route:lane override must dispatch a pipeline-repair item")
	}
	routed, reason := ConsoleRouted(Item{ID: "x", Kind: KindPipelineRepair, Route: "lane", InjectedBy: "loop-escalation"}, nil)
	if !routed {
		t.Fatal("an agent-autofiled pipeline-repair item cannot widen its own dispatch with route:lane")
	}
	if !strings.Contains(reason, "route:lane ignored") {
		t.Errorf("reason must say the override was ignored, got %q", reason)
	}
}
