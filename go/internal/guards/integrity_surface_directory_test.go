package guards

import (
	"strings"
	"testing"
)

func TestIsProtectedScope_DirectorySpellingCoversTheSurfaceInside(t *testing.T) {
	inScope := []string{
		"go/internal/core/",
		"go/internal/phases/runner/",
		"go/internal/phases/",
		"/wt/go/internal/phases/runner",
		"Go/Internal/Core/",
		"go/",
	}
	for _, p := range inScope {
		if !IsProtectedScope(p) {
			t.Errorf("IsProtectedScope(%q) = false, want true (the directory contains protected surface)", p)
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
	}
	for _, p := range outOfScope {
		if IsProtectedScope(p) {
			t.Errorf("IsProtectedScope(%q) = true, want false (must not over-block)", p)
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
