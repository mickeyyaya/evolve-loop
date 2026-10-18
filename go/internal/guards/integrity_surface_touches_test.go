package guards

import (
	"strings"
	"testing"
)

func TestTouchesProtectedSurface_ADirectoryThatContainsAFragmentIsProtected(t *testing.T) {
	containing := []string{
		"go/internal/core",
		"go/internal/core/",
		"go/cmd/evolve",
		"go/internal/phases/runner",
		"go/internal/phases/",
		"/wt/go/internal/phases/runner",
		"Go/Internal/Core/",
		"go/internal/core/../../internal/",
		"go/",
		"go",
		"skills",
		"agents/",
		".evolve",
	}
	for _, p := range containing {
		if !TouchesProtectedSurface(p) {
			t.Errorf("TouchesProtectedSurface(%q) = false, want true (the directory holds protected surface)", p)
		}
	}
}

func TestTouchesProtectedSurface_APathInsideAFragmentOrEqualToOneIsProtected(t *testing.T) {
	inside := []string{
		"go/internal/bridge",
		"go/internal/bridge/",
		"go/internal/bridge/sub/",
		"/wt/go/internal/phases/runner/verdict/settle",
		"Skills/Audit/references/",
		"go/internal/core/build_handoff_floor.go",
		"/wt/go/internal/guards/role.go",
		".evolve/policy.json",
	}
	for _, p := range inside {
		if !TouchesProtectedSurface(p) {
			t.Errorf("TouchesProtectedSurface(%q) = false, want true", p)
		}
	}
}

func TestTouchesProtectedSurface_APathThatMeetsNoFragmentIsFree(t *testing.T) {
	free := []string{
		"go/internal/inboxbatch/",
		"go/internal/inboxsplit",
		"go/internal/core/observer.go",
		"go/internal/core/observer.go/",
		"go/internal/core/build_handoff",
		"go/internal/bridgex",
		"go/internal/phases/runner/runner_x.go",
		"go/Makefile",
		"docs",
		"docs/architecture/",
		"cmd/evolve",
		"phases/runner",
		"shadow/enforce",
		"/",
		"",
		".",
	}
	for _, p := range free {
		if TouchesProtectedSurface(p) {
			t.Errorf("TouchesProtectedSurface(%q) = true, want false (must not over-block)", p)
		}
	}
}

func TestTouchesProtectedSurface_EveryAncestorOfEveryFragmentIsProtected(t *testing.T) {
	for _, e := range ProtectedSurfaceManifest {
		segments := strings.Split(strings.Trim(e.Fragment, "/"), "/")
		for n := 1; n <= len(segments); n++ {
			ancestor := strings.Join(segments[:n], "/")
			for _, spelled := range []string{ancestor, ancestor + "/", "/wt/" + ancestor} {
				if !TouchesProtectedSurface(spelled) {
					t.Errorf("TouchesProtectedSurface(%q) = false, want true (an ancestor of the fragment %q)", spelled, e.Fragment)
				}
			}
		}
	}
}

func TestTouchesProtectedSurface_ImpliedByMembership(t *testing.T) {
	for _, e := range ProtectedSurfaceManifest {
		rel := strings.TrimPrefix(e.Fragment, "/")
		for _, p := range []string{rel, "/wt" + e.Fragment, strings.TrimSuffix(rel, "/")} {
			if IsProtectedSurface(p) && !TouchesProtectedSurface(p) {
				t.Errorf("IsProtectedSurface(%q) but not TouchesProtectedSurface", p)
			}
		}
	}
}

func TestTouchesProtectedSurface_LeavesMembershipAndScopeAlone(t *testing.T) {
	for _, p := range []string{"go/internal/core", "go/internal/core/", "go/cmd/evolve", "go/"} {
		if IsProtectedSurface(p) || IsProtectedScope(p) {
			t.Errorf("%q became protected membership or scope; only TouchesProtectedSurface may widen to containment", p)
		}
	}
}
