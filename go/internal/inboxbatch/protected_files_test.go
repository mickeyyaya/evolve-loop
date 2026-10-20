package inboxbatch_test

import (
	"strings"
	"testing"
)

const cycle2000Route = `{"id":"router-active-worktree-fleet-pin-lane","kind":"test","files":["go/internal/router/router.go"],"route":"lane","protected_files":["go/internal/core/cyclerun.go"]}`

func TestConsoleRouted_AProtectedFileTheLoopProvedBindsLikeADeclaredOne(t *testing.T) {
	ok, reason := routed(t, cycle2000Route)
	if !ok || !strings.Contains(reason, "go/internal/core/cyclerun.go") || !strings.Contains(reason, "route:lane cannot relax") {
		t.Errorf("a proven protected file must keep the item off the lanes whatever the route: routed=%v %q", ok, reason)
	}
	if ok, reason := routed(t, strings.Replace(cycle2000Route, `"go/internal/core/cyclerun.go"`, `"go/internal/router/other.go"`, 1)); ok {
		t.Errorf("a proven file outside the protected surface does not route: %q", reason)
	}
}
