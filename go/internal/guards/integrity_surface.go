package guards

import (
	"path/filepath"
	"strings"
)

// protectedSurfaceFragments are slash-normalized path fragments identifying the
// pipeline INTEGRITY CONTROL PLANE: the deterministic gates that grade a cycle,
// the campaign metric SSOT, the guards themselves, the campaign contract, the
// grading rubrics, and the PreToolUse hook wiring.
//
// No autonomous phase agent may modify these — a cycle must never be able to
// edit the gate that judges it. This is the structural fix for the cycle-20
// breach, where the build agent edited
// go/acs/regression/flagreaders/readers_test.go (and the registry SSOT) to bless
// its own split-const dodge, and the audit approved the self-edit.
//
// Legitimate control-plane changes (e.g. an operator hardening a gate) go through
// human-gated `evolve ship --class manual` OUTSIDE any cycle, where the role
// guard's CycleID==0 path allows them — never an autonomous `--class cycle`.
//
// Fragments are matched anywhere in the slash-normalized path, so the boundary
// holds regardless of the file's physical root (a per-cycle worktree, the branch
// root, or main). The intentionally NARROW fragments (e.g. go/acs/regression/ but
// NOT go/acs/, registry_table.go but NOT the whole flagregistry package) preserve
// legitimate cycle behavior: a cycle still writes its own go/acs/cycleN/
// predicates and ordinary source.
var protectedSurfaceFragments = []string{
	"/go/acs/regression/",                                // standing deterministic gates
	"/go/internal/acssuite/",                             // the gate runner
	"/go/internal/guards/",                               // the guards (incl. this manifest + role.go)
	"/go/internal/flagregistry/registry_table.go",        // the campaign metric SSOT
	"/go/internal/flagregistry/registry_ceiling_test.go", // the ceiling ratchet gate
	"/knowledge-base/research/flag-campaign-plan.json",   // the campaign contract
	"/skills/audit/",                                     // the audit grading rubric
	"/skills/adversarial-review/",                        // the adversarial grading rubric
	"/.claude/settings.json",                             // PreToolUse hook wiring (repo + global ~/.claude)
	"/.evolve/policy.json",                               // gate-default overrides (eval/contract/swarm gates)
}

// IsProtectedSurface reports whether path targets the pipeline integrity control
// plane. path may be absolute or repo-relative; matching is on a slash-normalized
// fragment, so the boundary holds regardless of the file's physical root.
func IsProtectedSurface(path string) bool {
	if path == "" {
		return false
	}
	p := filepath.ToSlash(path)
	// Ensure a leading slash so a leading segment ("go/acs/regression/...") still
	// matches a "/go/acs/regression/" fragment.
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// Case-fold: macOS/Windows filesystems are case-insensitive, so "Go/ACS/..."
	// is the same path as "go/acs/..."; fragments are already lower-case.
	p = strings.ToLower(p)
	for _, frag := range protectedSurfaceFragments {
		if strings.Contains(p, frag) {
			return true
		}
	}
	return false
}
