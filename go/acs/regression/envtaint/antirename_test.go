//go:build acs

package envtaint

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// GetenvKeysFromSrc underlies the anti-rename invariant (ADR-0064 Pillar 2, M2):
// it collects every compile-time-constant key passed to os.Getenv / os.LookupEnv
// in src, at ANY prefix (so a dial renamed out of the EVOLVE_ namespace is
// visible), folding split-consts (so "HO"+"ME" -> HOME and the rename dodge
// "FO"+"O" cannot hide), and dropping dynamic (non-constant) keys.
func TestGetenvKeysFromSrc_AllPrefixesFoldsDynamicExcluded(t *testing.T) {
	const src = `package p

import "os"

var _ = os.Getenv("EVOLVE_PROJECT_ROOT") // an EVOLVE_ dial
var _ = os.Getenv("HO" + "ME")           // folded external var
var _ = os.LookupEnv("CI")               // LookupEnv is an env read too

// a dynamic key has no fixed name to police, so it is excluded.
func r(k string) string { return os.Getenv("DYN_" + k) }
`
	got, err := GetenvKeysFromSrc(src)
	if err != nil {
		t.Fatalf("GetenvKeysFromSrc: %v", err)
	}
	want := []string{"CI", "EVOLVE_PROJECT_ROOT", "HOME"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetenvKeysFromSrc = %v, want %v", got, want)
	}
}

// TestNoUnregisteredNonEvolveGetenvKey is the anti-rename gate (M2): every
// constant os.Getenv / os.LookupEnv key in production must be EVOLVE_-prefixed or
// in the pinned externalEnvAllowlist. A NEW non-EVOLVE_ key — e.g. a dial renamed
// out of the EVOLVE_ namespace to shrink the registry — fails here.
func TestNoUnregisteredNonEvolveGetenvKey(t *testing.T) {
	repo := acsassert.RepoRoot(t)
	keys, skipped, err := GetenvConstKeys(filepath.Join(repo, "go"))
	if err != nil {
		t.Fatalf("GetenvConstKeys: %v", err)
	}
	if len(skipped) > 0 {
		t.Logf("envtaint: %d unparseable file(s) skipped: %v", len(skipped), skipped)
	}
	if len(keys) < 10 {
		t.Fatalf("os.Getenv key set implausibly small (%d) — the production scan is likely broken", len(keys))
	}
	for _, k := range keys {
		if strings.HasPrefix(k, "EVOLVE_") || externalEnvAllowlist[k] {
			continue
		}
		t.Errorf("os.Getenv(%q): non-EVOLVE_ env key with no allowlist entry.\n"+
			"  Operator dials use the reserved EVOLVE_ prefix. If this is a legitimate external or\n"+
			"  IPC variable, add it to externalEnvAllowlist (a Pillar-1 protected surface). A dial\n"+
			"  renamed out of the EVOLVE_ namespace to dodge the registry is caught here.", k)
	}
}

// TestRenameDodge_NonEvolveKeyIsFlagged proves the anti-rename check catches a
// dial renamed out of the EVOLVE_ namespace via split-const — the go/ast scan and
// the EVOLVE_-only read-set both miss it; the fold-aware all-prefix collector
// surfaces it as a non-allowlisted non-EVOLVE_ key.
func TestRenameDodge_NonEvolveKeyIsFlagged(t *testing.T) {
	const renamed = `package p

import "os"

// renamed from EVOLVE_WORKTREE_BASE and split so a literal scan cannot see it.
var _ = os.Getenv("WO" + "RKTREE_BASE")
`
	keys, err := GetenvKeysFromSrc(renamed)
	if err != nil {
		t.Fatalf("GetenvKeysFromSrc: %v", err)
	}
	var flagged []string
	for _, k := range keys {
		if !strings.HasPrefix(k, "EVOLVE_") && !externalEnvAllowlist[k] {
			flagged = append(flagged, k)
		}
	}
	if len(flagged) != 1 || flagged[0] != "WORKTREE_BASE" {
		t.Fatalf("rename dodge not flagged: keys=%v flagged=%v", keys, flagged)
	}
}
