package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// mintNameRE is the same kebab-case floor phasespec/phaseregistrar apply to
// phase names. Enforced here BEFORE any filesystem access because registry
// names are attacker-influenceable strings used as a path segment — a forged
// "../x" must never leave .evolve/phases/.
var mintNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// maxMintSpecBytes bounds the spec read in verifiedActiveMints. A registrar-
// persisted phase.json is a few hundred bytes; 1 MiB is generous headroom
// while denying a planted-giant-file memory exhaustion.
const maxMintSpecBytes = 1 << 20

// isActiveMintPhasePath reports whether a main-tree write is a VERIFIED
// advisor mint's phase config — the third guard-only classifier beside
// isLegitimateMainTreePath and isScoutEvalMaterialization. In fleet mode both
// lanes diff the SAME shared tree with per-lane baselines, so lane A's mint of
// .evolve/phases/<name>/phase.json lands in lane B's post-phase diff and was
// charged to lane B as a deliverable leak (cycle-967 false-abort). The
// registrar records every minted name in the shared mintregistry BEFORE
// persisting its files; the guard exempts a leaked path IFF it is one of the
// EXACTLY TWO paths a mint writes — .evolve/phases/<name> (bare dir entry) or
// .evolve/phases/<name>/phase.json — for a registered, TTL-fresh, CONTENT-
// VERIFIED name (mints must already be filtered through verifiedActiveMints).
// A novel phase-config name, a companion payload under a registered name, and
// an unclamped spec all still fire the guard — the deliverable-leak pin
// (TestIsScoutEvalMaterialization's {scout, .evolve/phases/x/phase.json,
// false}) keeps governing smuggles.
func isActiveMintPhasePath(mints map[string]bool, p string) bool {
	const pfx = ".evolve/phases/"
	if len(mints) == 0 || !strings.HasPrefix(p, pfx) {
		return false
	}
	rest := strings.TrimPrefix(p, pfx)
	name, sub, nested := strings.Cut(rest, "/")
	if nested && sub != "phase.json" {
		return false // a mint writes EXACTLY phase.json — payloads abort
	}
	return name != "" && mints[name]
}

// verifiedActiveMints filters registered mint names to those whose on-disk
// .evolve/phases/<name>/phase.json passes the registrar's spec-side clamp:
// parses as a PhaseSpec, spec name matches the directory, optional (the user-
// phase floor), and ValidateUserSpec reports no violations. This is the
// content half of the exemption (security review, Finding 1): the registry
// file itself is not authenticated — with the OS sandbox off ANY on-disk
// state is forgeable — so the exemption's invariant is clamp PARITY, not
// provenance: a forged entry can never smuggle anything a sanctioned mint
// could not already carry, and anything unclamped still aborts loudly.
func verifiedActiveMints(projectRoot string, mints map[string]bool) map[string]bool {
	verified := make(map[string]bool, len(mints))
	for name := range mints {
		if !mintNameRE.MatchString(name) {
			continue // path-segment safety: never let a forged name traverse
		}
		specPath := filepath.Join(projectRoot, ".evolve", "phases", name, "phase.json")
		// Lstat gate before the read (security delta review): the registrar
		// writes a small REGULAR file, so anything else — a FIFO that would
		// block ReadFile forever, a symlink that would point the read out of
		// the tree, an oversized file — is not a mint and must not be read.
		fi, err := os.Lstat(specPath)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxMintSpecBytes {
			continue
		}
		raw, err := os.ReadFile(specPath)
		if err != nil {
			continue
		}
		var spec phasespec.PhaseSpec
		if json.Unmarshal(raw, &spec) != nil {
			continue
		}
		if spec.Name != name || !spec.Optional || len(phasespec.ValidateUserSpec(spec)) > 0 {
			continue
		}
		verified[name] = true
	}
	return verified
}
