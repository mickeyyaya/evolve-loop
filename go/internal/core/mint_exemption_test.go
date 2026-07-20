package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifiedActiveMints pins the content-verification half of the mint
// exemption (security review, Finding 1): a registered NAME is kept only when
// its on-disk .evolve/phases/<name>/phase.json passes the same spec-side
// clamp the registrar enforces — parses, user-floor valid (ValidateUserSpec),
// optional, and spec name == dir name. A forged registry entry therefore buys
// nothing beyond what a sanctioned mint could already carry.
func TestVerifiedActiveMints(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		dir := filepath.Join(root, ".evolve", "phases", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "phase.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("good-mint", `{"name":"good-mint","optional":true}`)
	write("bad-optional", `{"name":"bad-optional","optional":false}`)
	write("bad-mismatch", `{"name":"other-name","optional":true}`)
	write("bad-garbage", `{not json`)
	// "bad-missing" has a registry entry but no file at all; "../bad-trav"
	// is a forged path-traversal name that must be dropped BEFORE any
	// filesystem access (path-segment safety).
	// "bad-huge" and "bad-symlink" pin the read-bounding half (security
	// delta review): the spec read must be Lstat-gated to a small REGULAR
	// file, so a planted FIFO/symlink/giant file cannot hang or exhaust the
	// guard (and a symlink cannot point the read outside the tree).
	write("bad-huge", `{"name":"bad-huge","optional":true,"pad":"`+strings.Repeat("x", 1<<20)+`"}`)
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "phases", "bad-symlink"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join(root, ".evolve", "phases", "good-mint", "phase.json"),
		filepath.Join(root, ".evolve", "phases", "bad-symlink", "phase.json"),
	); err != nil {
		t.Fatal(err)
	}
	mints := map[string]bool{
		"good-mint": true, "bad-optional": true, "bad-mismatch": true,
		"bad-garbage": true, "bad-missing": true, "../bad-trav": true,
		"bad-huge": true, "bad-symlink": true,
	}
	got := verifiedActiveMints(root, mints)
	if len(got) != 1 || !got["good-mint"] {
		t.Errorf("verifiedActiveMints=%v, want {good-mint} only", got)
	}
	if out := verifiedActiveMints(root, nil); len(out) != 0 {
		t.Errorf("nil input must stay empty; got %v", out)
	}
}

// TestIsActiveMintPhasePath pins the classifier's scope so the exemption
// cannot widen: ONLY the two paths a legitimate mint writes —
// .evolve/phases/<registered-name> (bare dir entry) and
// .evolve/phases/<registered-name>/phase.json — are exempt. Companion payload
// files under a registered name, prefix look-alikes, other deliverables,
// source paths, and an empty registry all stay armed. Companion end-to-end
// pins live in treediff_crosslane_mint_test.go (integration tag).
func TestIsActiveMintPhasePath(t *testing.T) {
	mints := map[string]bool{"gate-wiring-proof": true, "gate": true}
	cases := []struct {
		path string
		want bool
	}{
		{".evolve/phases/gate-wiring-proof/phase.json", true},
		{".evolve/phases/gate-wiring-proof", true},                    // bare dir entry for a registered mint
		{".evolve/phases/gate-wiring-proof/nested/extra.json", false}, // a mint writes ONLY phase.json — payloads abort
		{".evolve/phases/gate-wiring-proof/payload.sh", false},        // smuggled companion file aborts
		{".evolve/phases/gate-wiring-proof/", false},                  // trailing-slash dir handled by the bare-dir rule, not mint
		{".evolve/phases/evil/phase.json", false},                     // unregistered name
		{".evolve/phases/gate-wiring-proof-evil/x.json", false},       // segment match, not prefix ("gate" registered too)
		{".evolve/phases/", false},                                    // no name segment
		{".evolve/phases", false},                                     // bare deliverable root
		{".evolve/evals/gate-wiring-proof.md", false},                 // other deliverables unaffected
		{".evolve/phasesx/gate-wiring-proof/phase.json", false},       // exact dir prefix only
		{"go/internal/core/leak.go", false},                           // source never exempt
		{"x/.evolve/phases/gate-wiring-proof/phase.json", false},      // top-level .evolve only
	}
	for _, tc := range cases {
		if got := isActiveMintPhasePath(mints, tc.path); got != tc.want {
			t.Errorf("isActiveMintPhasePath(%q)=%v, want %v", tc.path, got, tc.want)
		}
	}
	if isActiveMintPhasePath(nil, ".evolve/phases/gate-wiring-proof/phase.json") {
		t.Error("nil registry must exempt nothing")
	}
}
