// Package projecthash computes the 8-char SHA256 namespace used to
// isolate per-project state (worktrees, .evolve/, ledger, cycle-state).
//
// Byte-exact port of the bash pipeline at
// scripts/dispatch/preflight-environment.sh:260-264:
//
//	PROJECT_HASH=$(printf '%s' "$EVOLVE_PROJECT_ROOT" | shasum -a 256 | head -c 8)
//	# Linux:
//	PROJECT_HASH=$(printf '%s' "$EVOLVE_PROJECT_ROOT" | sha256sum | head -c 8)
//	# Empty project root:
//	PROJECT_HASH="default"
//
// Equivalence is load-bearing: cross-language drift would invalidate
// existing worktree paths under .evolve/worktrees/<hash>/.
package projecthash

import (
	"crypto/sha256"
	"encoding/hex"
)

// Compute returns the lower-hex first 8 chars of SHA256(input).
// Equivalent to `printf '%s' "$input" | shasum -a 256 | head -c 8`.
func Compute(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])[:8]
}

// ForProjectRoot returns the 8-char hash for a project root, or the
// literal "default" when root is empty — matching the bash fallback
// at preflight-environment.sh:264.
func ForProjectRoot(root string) string {
	if root == "" {
		return "default"
	}
	return Compute(root)
}
