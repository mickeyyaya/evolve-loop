package guards

import (
	"strings"
	"testing"
)

// TestIsProtectedScope_DirectorySpellingCoversTheSurfaceInside (F29): the SCOPE
// projection answers the question a declared fix surface poses — a directory
// spelling (trailing slash, or a last segment with no extension) is in scope
// when a manifest fragment lies inside it. Before F29 the console classifier
// had only membership, so `tokenopt-handoff-digests-per-edge-remainder` (files
// go/internal/core/, go/internal/phases/runner/) reached a lane four times
// (cycles 1646, 1651, 1654, 1656) and triage's breaker refused its card each
// time.
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

// TestIsProtectedSurface_MembershipStaysMembership: the MEMBERSHIP projection
// the ship tripwire, the role write-guard, the fleet preflight and triage's
// breaker use is NOT widened to scope — a directory that merely contains
// protected files is not itself a member (the breaker must never get stricter
// than the seed that screens for it). Its one F29 fix: a path NAMING a
// protected directory without the trailing slash (a package or import path)
// is a member of that directory.
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

// TestIsProtectedScope_ImpliedByMembership pins the invariant that makes the
// seed-time screen sound: every member is in scope — for every manifest
// fragment (repo-relative, with and without a worktree prefix) and the table
// cases above, so the classifier using scope refuses at least what the breaker
// using membership would.
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
