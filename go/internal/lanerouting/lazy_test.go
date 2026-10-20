package lanerouting

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type recorded struct{ events []signalcenter.Event }

func (r *recorded) listen(e signalcenter.Event) { r.events = append(r.events, e) }

func TestLazyForbidden_WithoutTheProfileEmitsTheRegisteredFallbackCodeOnceAndJudgesProtectedSurface(t *testing.T) {
	var got recorded
	forbidden := LazyForbidden(t.TempDir(), "writer", got.listen)
	if !forbidden("go/internal/guards/role.go") || forbidden(".evolve/profiles/historian.json") {
		t.Fatal("the fallback judges protected surface only")
	}
	_ = forbidden("go/internal/guards/ship.go")
	if len(got.events) != 1 {
		t.Fatalf("a profile that will not load must be loud exactly once, got %d events", len(got.events))
	}
	e := got.events[0]
	if e.Code != CodeRoutingFallback || e.Module != signalcenter.ModuleInbox || e.Kind != signalcenter.KindInboxWarning || e.Severity != signalcenter.SeverityWarn {
		t.Fatalf("the fallback must be the registered inbox WARN code: %+v", e)
	}
	if e.Fields["profile"] != "writer" || e.Reason == "" || !signalcenter.ValidOrigin(e.Origin) {
		t.Fatalf("the event names the profile, the load error and a valid origin: %+v", e)
	}
}

func TestLazyForbidden_JudgesTheSandboxDeniedPathSilentlyWhenTheProfileLoads(t *testing.T) {
	root := t.TempDir()
	writeProfile(t, root, "writer", ".evolve/profiles")
	var got recorded
	forbidden := LazyForbidden(root, "writer", got.listen)
	if !forbidden(".evolve/profiles/historian.json") || forbidden("go/internal/fleet/fleet.go") {
		t.Fatal("a loaded profile adds its sandbox denials to protected surface")
	}
	if len(got.events) != 0 {
		t.Fatalf("a loaded profile emits nothing, got %+v", got.events)
	}
}

func TestLazyForbidden_ARootThatNeverRoutesEmitsNothing(t *testing.T) {
	var got recorded
	_ = LazyForbidden(t.TempDir(), "writer", got.listen)
	if len(got.events) != 0 {
		t.Fatalf("building the predicate must not load or emit, got %+v", got.events)
	}
}

func TestCodeRoutingFallback_IsRegisteredToTheInboxModule(t *testing.T) {
	if m, ok := signalcenter.IsRegistered(CodeRoutingFallback); !ok || m != signalcenter.ModuleInbox {
		t.Fatalf("%s is not registered to the inbox module (module=%q)", CodeRoutingFallback, m)
	}
}
