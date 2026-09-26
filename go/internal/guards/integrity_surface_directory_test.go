package guards

import (
	"strings"
	"testing"
)

func TestIsProtectedScope_DirectorySpellingCoversTheSurfaceInside(t *testing.T) {
	inScope := []string{
		"go/internal/core/",             // holds orchestrator.go, cyclerun.go, …
		"go/internal/phases/runner/",    // holds runner.go, dispatch.go, verdict/
		"go/internal/phases/",           // a parent of protected phase files
		"/wt/go/internal/phases/runner", // no trailing slash, no extension: a directory spelling
		"Go/Internal/Core/",             // case-insensitive, like files
		"go/",                           // the whole tree contains the surface
	}
	for _, p := range inScope {
		if !IsProtectedScope(p) {
			t.Errorf("IsProtectedScope(%q) = false, want true (the directory contains protected surface)", p)
		}
	}
	outOfScope := []string{
		"go/internal/inboxbatch/",      // no protected file inside
		"docs/architecture/",           // docs are not control plane
		"go/internal/core/observer.go", // a FILE keeps membership: not a fragment
		"go/Makefile",                  // extension-less, read as a directory: nothing protected inside
		"phases/runner",                // a partial path: no fragment starts there
		"shadow/enforce",               // prose with a slash
		"/",                            // the root alone never matches every fragment
	}
	for _, p := range outOfScope {
		if IsProtectedScope(p) {
			t.Errorf("IsProtectedScope(%q) = true, want false (must not over-block)", p)
		}
	}
}

func TestIsProtectedSurface_MembershipStaysMembership(t *testing.T) {
	members := []string{
		"go/internal/bridge", // names the protected /go/internal/bridge/ directory
		"github.com/mickeyyaya/evolve-loop/go/internal/bridge", // an import path names it too
		"go/internal/bridge/driver_codex.go",
		"go/internal/core/orchestrator.go",
	}
	for _, p := range members {
		if !IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = false, want true", p)
		}
	}
	notMembers := []string{
		"go/internal/core/",          // CONTAINS protected files; membership says no — scope's job
		"go/internal/phases/runner/", // same
		"go/cmd/evolve",              // contains cmd_cycle_config.go — not itself a member
		"go/internal/bridgex",        // a sibling name, not the directory
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
