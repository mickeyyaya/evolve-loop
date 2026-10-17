package guards

import (
	"strings"
	"testing"
)

func TestIsProtectedScope_ADirectoryThatOnlyContainsAFragmentIsNotProtected(t *testing.T) {
	containsOnly := []string{
		"go/internal/core/",
		"go/internal/phases/runner/",
		"go/internal/phases/",
		"/wt/go/internal/phases/runner",
		"Go/Internal/Core/",
		"go/",
		"go/internal/core/../../internal/",
	}
	for _, p := range containsOnly {
		if IsProtectedScope(p) {
			t.Errorf("IsProtectedScope(%q) = true, want false (the directory only contains protected surface)", p)
		}
	}
	outOfScope := []string{
		"go/internal/inboxbatch/",
		"docs/architecture/",
		"go/internal/core/observer.go",
		"go/Makefile",
		"phases/runner",
		"shadow/enforce",
		"/",
		"",
		".",
		"go",
	}
	for _, p := range outOfScope {
		if IsProtectedScope(p) {
			t.Errorf("IsProtectedScope(%q) = true, want false (must not over-block)", p)
		}
	}
}

func TestIsProtectedScope_ADirectoryInsideAFragmentIsProtected(t *testing.T) {
	inside := []string{
		"go/internal/bridge/sub/",
		"go/internal/bridge/",
		"go/internal/bridge",
		"/wt/go/internal/phases/runner/verdict/settle",
		"Skills/Audit/references/",
		"go/internal/core/../bridge/sub/",
	}
	for _, p := range inside {
		if !IsProtectedScope(p) {
			t.Errorf("IsProtectedScope(%q) = false, want true (the directory lies inside a protected fragment)", p)
		}
	}
}

func TestIsProtectedScope_AProtectedFileSpellingIsProtected(t *testing.T) {
	files := []string{
		"go/internal/core/build_handoff_floor.go",
		"go/internal/phases/runner/runner.go",
		"/wt/go/internal/guards/role.go",
		"skills/audit/SKILL.md",
	}
	for _, p := range files {
		if !IsProtectedScope(p) {
			t.Errorf("IsProtectedScope(%q) = false, want true (a protected file stays protected)", p)
		}
	}
}

func TestIsProtectedSurface_MembershipStaysMembership(t *testing.T) {
	members := []string{
		"go/internal/bridge",
		"github.com/mickeyyaya/evolve-loop/go/internal/bridge",
		"go/internal/bridge/driver_codex.go",
		"go/internal/core/orchestrator.go",
	}
	for _, p := range members {
		if !IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = false, want true", p)
		}
	}
	notMembers := []string{
		"go/internal/core/",
		"go/internal/phases/runner/",
		"go/cmd/evolve",
		"go/internal/bridgex",
	}
	for _, p := range notMembers {
		if IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = true, want false (membership must not widen to scope)", p)
		}
	}
}

func TestIsProtectedScope_ImpliedByMembership(t *testing.T) {
	var paths []string
	for _, e := range ProtectedSurfaceManifest {
		rel := strings.TrimPrefix(e.Fragment, "/")
		paths = append(paths, rel, "/wt"+e.Fragment, strings.TrimSuffix(rel, "/"))
	}
	paths = append(paths, "go/internal/bridge", "go/internal/core/", "docs/x.md", "go/Makefile")
	for _, p := range paths {
		if IsProtectedSurface(p) && !IsProtectedScope(p) {
			t.Errorf("IsProtectedSurface(%q) but not IsProtectedScope — the seed would admit what the breaker refuses", p)
		}
	}
}
