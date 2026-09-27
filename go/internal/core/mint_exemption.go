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

// isActiveMintPhasePath exempts only the exact two paths a real mint
// writes, for a registered, TTL-fresh, content-verified name; anything
// else — a novel name, a companion payload, an unclamped spec — still
// fires the guard.
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

// verifiedActiveMints trusts clamp parity, not provenance: with the OS
// sandbox off, any on-disk phase.json is forgeable, so this filters
// registered names to those whose spec passes the registrar's own clamp
// (PhaseSpec, name match, optional, ValidateUserSpec) rather than
// authenticating the file itself.
func verifiedActiveMints(projectRoot string, mints map[string]bool) map[string]bool {
	verified := make(map[string]bool, len(mints))
	for name := range mints {
		if !mintNameRE.MatchString(name) {
			continue // path-segment safety: never let a forged name traverse
		}
		specPath := filepath.Join(projectRoot, ".evolve", "phases", name, "phase.json")
		// Lstat before read: the registrar writes a small regular file, so a
		// FIFO, a symlink, or an oversized file is not a mint and must not be read.
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
