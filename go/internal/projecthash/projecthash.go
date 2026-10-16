// Package projecthash computes the 8-char SHA256 namespace used to isolate
// per-project state (worktrees, .evolve/, ledger, cycle-state).
package projecthash

import (
	"crypto/sha256"
	"encoding/hex"
)

func Compute(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])[:8]
}

func ForProjectRoot(root string) string {
	if root == "" {
		return "default"
	}
	return Compute(root)
}
