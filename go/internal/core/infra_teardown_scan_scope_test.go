package core

// infra_teardown_scan_scope_test.go — cycle-1270 Task 1
// (`infra-teardown-predicate-single-source`), the open residual.
//
// The consolidation itself is landed and green: IsInfraTeardownError is adopted
// at orchestrator.go and cyclerun_dispatch.go, and
// TestInfraTeardownUnion_SpelledExactlyOnce guards it. But that guard scans
// `.` — internal/core ONLY. The predicate's consumers live OUTSIDE it
// (phases/runner, bridge), so a re-spelled union in either package passes the
// "spelled exactly once" check untouched.
//
// No live duplicate exists today, which is precisely why this is pinned now:
// the guard's whole purpose is the item's own "if a THIRD sentinel is ever
// added" concern, and a guard that cannot see two thirds of its own blast
// radius does not serve it.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// infraTeardownUnionScanRoots are the directories the uniqueness guard must
// cover: internal/core (the definition) plus every package that consumes the
// sentinels. Paths are relative to internal/core, where the test runs.
var infraTeardownUnionScanRoots = []string{
	".",
	"../bridge",
	"../phases/runner",
}

// findInfraTeardownUnionSpellingsAcross scans several roots and namespaces each
// hit by its root, so two same-named files in different packages stay
// distinguishable. It reuses the AST walker verbatim — the shape is a UNIQUENESS
// assertion over parsed expressions, not a source-grep, and widening the roots
// must not degrade it to one.
func findInfraTeardownUnionSpellingsAcross(roots ...string) ([]string, error) {
	var all []string
	for _, root := range roots {
		sites, err := findInfraTeardownUnionSpellings(root)
		if err != nil {
			return nil, err
		}
		for _, s := range sites {
			all = append(all, filepath.ToSlash(filepath.Join(root, s)))
		}
	}
	sort.Strings(all)
	return all, nil
}

func TestInfraTeardownUnion_ScanCoversConsumerPackages(t *testing.T) {
	// The roots must actually exist — a widening that silently names a
	// nonexistent directory buys the same blindness it claims to remove.
	for _, root := range infraTeardownUnionScanRoots {
		if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
			t.Fatalf("scan root %s is not a directory (%v) — the widened guard would silently see nothing there", root, err)
		}
	}

	sites, err := findInfraTeardownUnionSpellingsAcross(infraTeardownUnionScanRoots...)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	const canonical = "errors.go:IsInfraTeardownError"
	if len(sites) != 1 || !strings.HasSuffix(sites[0], canonical) {
		t.Fatalf("the (timeout OR transient) union is spelled at %d site(s) across %v: %v\n"+
			"want EXACTLY one — %s. Every other site meaning the same concept must call "+
			"IsInfraTeardownError; timeout-ONLY and transient-ONLY sites are different predicates "+
			"and must NOT be collapsed into it to satisfy this (TestC1270_005 pins that).",
			len(sites), infraTeardownUnionScanRoots, sites, canonical)
	}
}

func TestInfraTeardownUnion_DetectsPlantedDuplicateOutsideCore(t *testing.T) {
	// Anti-no-op. Widening the ROOT LIST without widening what the scanner
	// actually sees would pass "roots include X" and fail this: a synthetic
	// union planted in a package outside internal/core must be REPORTED.
	planted := t.TempDir()
	src := `package elsewhere

import "errors"

func teardownish(e error) bool {
	return errors.Is(e, ErrArtifactTimeout) || errors.Is(e, ErrTransientBridgeFailure)
}
`
	if err := os.WriteFile(filepath.Join(planted, "planted.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}

	sites, err := findInfraTeardownUnionSpellingsAcross(planted)
	if err != nil {
		t.Fatalf("scan planted root: %v", err)
	}
	if len(sites) != 1 || !strings.HasSuffix(sites[0], "planted.go:teardownish") {
		t.Fatalf("scan of a root outside internal/core reported %v, want the planted duplicate — "+
			"a guard that cannot SEE a re-spelling in a consumer package cannot enforce single-sourcing there", sites)
	}
}
