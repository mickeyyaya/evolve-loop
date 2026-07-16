package guards

// integrity_surface_manifest_test.go — regression pins for the L4
// protected-surface SSOT refactor (architecture review 2026-07-16). The
// fragment list became the exported ProtectedSurfaceManifest registry; these
// tests prove the refactor is behavior-preserving (every pre-refactor fragment
// survives byte-for-byte and still denies) and that every entry — including the
// L4 perimeter additions — satisfies the invariants IsProtectedSurface's
// Contains matching relies on. The durable cross-tree tripwire (every
// gate-shaped file must be covered by the manifest) lives in
// go/acs/regression/protectedsurface.

import (
	"strings"
	"testing"
)

// preRefactorFragments is the EXACT fragment set of the pre-L4
// protectedSurfaceFragments literal. The SSOT refactor must be
// behavior-preserving: every one of these must survive in
// ProtectedSurfaceManifest byte-for-byte. Additions are fine (L4 deliberately
// GROWS the perimeter); removals or edits of these are a regression.
var preRefactorFragments = []string{
	"/go/acs/regression/",
	"/go/internal/acssuite/",
	"/go/internal/guards/",
	"/go/internal/flagregistry/registry_table.go",
	"/go/internal/flagregistry/registry_ceiling_test.go",
	"/knowledge-base/research/flag-campaign-plan.json",
	"/skills/audit/",
	"/skills/adversarial-testing/",
	"/.claude/settings.json",
	"/.evolve/policy.json",
}

// TestProtectedSurfaceManifest_KeepsEveryPreRefactorFragment proves the SSOT
// refactor dropped nothing: each pre-refactor fragment appears verbatim in the
// exported manifest.
func TestProtectedSurfaceManifest_KeepsEveryPreRefactorFragment(t *testing.T) {
	var manifest []ProtectedSurfaceEntry = ProtectedSurfaceManifest
	got := make(map[string]bool, len(manifest))
	for _, e := range manifest {
		got[e.Fragment] = true
	}
	for _, frag := range preRefactorFragments {
		if !got[frag] {
			t.Errorf("pre-refactor fragment %q missing from ProtectedSurfaceManifest — "+
				"the SSOT refactor must be behavior-preserving (removals/edits of "+
				"pre-L4 entries are a regression)", frag)
		}
	}
}

// TestProtectedSurfaceManifest_EveryEntryDenies is the table test over the
// WHOLE manifest (pre-refactor entries + L4 additions): a path containing each
// fragment must be denied in both worktree-absolute and repo-relative form, and
// each entry must satisfy the well-formedness invariants the Contains matcher
// relies on — lower-case (matching case-folds the path, never the fragment),
// slash-normalized, leading slash, and a non-empty rationale (the manifest is
// its own documentation).
func TestProtectedSurfaceManifest_EveryEntryDenies(t *testing.T) {
	if len(ProtectedSurfaceManifest) == 0 {
		t.Fatal("ProtectedSurfaceManifest is empty — the control plane has no boundary")
	}
	for _, e := range ProtectedSurfaceManifest {
		t.Run(e.Fragment, func(t *testing.T) {
			if e.Fragment != strings.ToLower(e.Fragment) {
				t.Errorf("fragment %q is not lower-case; IsProtectedSurface folds the PATH "+
					"to lower, so an upper-case fragment can never match", e.Fragment)
			}
			if strings.Contains(e.Fragment, "\\") {
				t.Errorf("fragment %q contains a backslash; fragments are slash-normalized", e.Fragment)
			}
			if !strings.HasPrefix(e.Fragment, "/") {
				t.Errorf("fragment %q lacks a leading slash; anchored dir/file fragments "+
					"start with / so 'go/acs' cannot match 'lego/acs'", e.Fragment)
			}
			if e.Rationale == "" {
				t.Errorf("fragment %q has no Rationale; every manifest entry must document "+
					"why its surface grades/gates a cycle", e.Fragment)
			}
			// A directory entry protects files under it; a file entry protects the
			// file itself.
			sample := e.Fragment
			if strings.HasSuffix(sample, "/") {
				sample += "x.go"
			}
			abs := "/users/x/.evolve/worktrees/cycle-9" + sample
			if !IsProtectedSurface(abs) {
				t.Errorf("IsProtectedSurface(%q) = false, want true (worktree-absolute form "+
					"of manifest entry %q must deny)", abs, e.Fragment)
			}
			rel := strings.TrimPrefix(sample, "/")
			if !IsProtectedSurface(rel) {
				t.Errorf("IsProtectedSurface(%q) = false, want true (repo-relative form of "+
					"manifest entry %q must deny)", rel, e.Fragment)
			}
		})
	}
}

// TestProtectedSurfaceManifest_NonProtectedPathsStillAllow pins the NARROW half
// of the boundary after the L4 perimeter growth: ordinary source right NEXT to
// the newly protected surfaces must stay writable by a cycle.
func TestProtectedSurfaceManifest_NonProtectedPathsStillAllow(t *testing.T) {
	allowed := []string{
		// core stays writable — only its four gate-shaped files are pinned.
		"/wt/go/internal/core/orchestrator.go",
		// ship phase impl stays writable — only binary_staging_guard*.go is pinned.
		"/wt/go/internal/phases/ship/native.go",
		// cycles legitimately write their OWN predicates (regression/ is the gate).
		"/wt/go/acs/cycle21/predicates_test.go",
		// non-SSOT flagregistry code stays writable.
		"/wt/go/internal/flagregistry/registry.go",
		"/wt/README.md",
	}
	for _, p := range allowed {
		if IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = true, want false (the manifest must stay "+
				"NARROW; over-blocking breaks legitimate cycle writes)", p)
		}
	}
}
