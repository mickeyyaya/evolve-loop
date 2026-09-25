package inboxbatch

// consoleroute_test.go — RED contract for ADR-0074 I1 (typed routing authority).
// Cycles 1034/1035/1036 (2026-07-22, one wave) each burned a full pipeline on a
// task whose fix surface is the pipeline's own control plane
// (ProtectedSurfaceManifest paths a cycle may not write). The routing signal
// existed only as prose annotations no component consumed. This file pins the
// SINGLE deterministic classifier both consumers (triage prompt composition and
// inboxmover.Claim) must share: route-field plumbing plus a protected-fix-surface
// derivation, with an explicit operator override.

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

// A route field with the console prefix routes the item to the operator,
// regardless of files — this is the explicit form (console-manual,
// console-salvage, future console-* refinements).
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

// Derived form: any declared fix-surface file on the protected manifest routes
// the item out of lane reach even without a route field — the statically
// blocked class (1035 guard-phase-hook, 1036 role.go, cycle-858 policy.json).
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

// Real inbox items annotate files entries with prose suffixes
// ("go/internal/guards/role.go (implement allowance)") — the classifier must
// consult the path token, not the whole annotated string.
func TestConsoleRouted_AnnotatedFileEntryFirstToken(t *testing.T) {
	it := Item{ID: "x", Files: []string{"go/internal/guards/role.go (implement the documented allowance)"}}
	routed, _ := ConsoleRouted(it, protectedStub("go/internal/guards/role.go"))
	if !routed {
		t.Fatal("annotated files entry must still match on its leading path token")
	}
}

// route:"lane" is the operator override for false positives — it forces
// dispatchability even when a declared file is protected (e.g. the item only
// READS the surface). Explicit human routing beats the derivation.
func TestConsoleRouted_ExplicitLaneOverrideWins(t *testing.T) {
	it := Item{ID: "x", Route: "lane", Files: []string{"go/internal/guards/role.go"}}
	routed, _ := ConsoleRouted(it, protectedStub("go/internal/guards/role.go"))
	if routed {
		t.Fatal("route:lane must override the protected-files derivation")
	}
}

// A nil predicate disables only the derivation — the explicit route field still
// binds (inboxmover callers that cannot import guards still honor routing).
func TestConsoleRouted_NilPredicateRouteFieldStillBinds(t *testing.T) {
	if routed, _ := ConsoleRouted(Item{ID: "x", Route: "console-manual"}, nil); !routed {
		t.Fatal("nil predicate must not disable explicit route routing")
	}
	if routed, _ := ConsoleRouted(Item{ID: "x", Files: []string{"a.go"}}, nil); routed {
		t.Fatal("nil predicate + no route field must stay dispatchable")
	}
}

// PartitionConsole splits preserving input order and reports one reason line
// per routed item (loud exclusion — silent narrowing reads as full coverage).
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

// LoadDir must parse + sanitize the route field like every other rendered
// field (it reaches prompts and logs).
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

// Kind form (wave 6, cycle 1688, 2026-09-15): a `pipeline-*` item is
// pipeline-integrity work, which the operator owns (pipeline_fixes_console_first;
// the wave goal names it console-owned). The queue held 15 such items lane-eligible,
// 8 with no files[] list at all, so the files-derived rule had nothing to match:
// a lane claimed one, scout and triage ran, and the triage breaker refused the
// card for naming a protected surface — a full FAIL seal for work no lane may do.
// The kind is a first-class field the classifier must read, not prose.
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
	// Provenance precedence: the explicit route names itself first, then the
	// kind, then a protected file — callers print the reason verbatim.
	it := Item{ID: "x", Kind: "pipeline-repair", Files: []string{"go/internal/guards/role.go"}}
	if _, reason := ConsoleRouted(it, protectedStub("go/internal/guards/role.go")); !strings.HasPrefix(reason, "kind:pipeline-repair") {
		t.Errorf("the kind reason outranks the files derivation, got %q", reason)
	}
	if _, reason := ConsoleRouted(Item{ID: "x", Kind: "pipeline-repair", Route: "console-manual"}, nil); !strings.HasPrefix(reason, "route:") {
		t.Errorf("the explicit route outranks the kind, got %q", reason)
	}
}

// The operator's explicit route:"lane" override applies to the kind rule
// exactly as it applies to the files rule: honored for an operator-authored
// item, ignored for an agent-autofiled one (the ADR-0072 halt escalation
// autofiles pipeline-repair items — those stay console-owned however they
// are annotated).
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
