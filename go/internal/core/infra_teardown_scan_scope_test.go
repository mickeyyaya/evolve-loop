package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var infraTeardownUnionScanRoots = []string{
	".",
	"../bridge",
	"../phases/runner",
}

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
	t.Parallel()

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
	t.Parallel()

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
