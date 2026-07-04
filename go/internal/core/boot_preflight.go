package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// boot_preflight.go — boot-time recovery primitives for a dirty/tampered main
// tree (cycle 507, task wire-boot-recovery-functions; the piece cycle 506's
// audit F1 flagged as unwired). When a leak escapes into the main tree, EVERY
// subsequent cycle's tree-diff guard FAILs, attributing the pre-existing dirt to
// whichever phase runs first and wedging the loop until a human `git stash`es.
// These functions let runLoop's boot path self-heal before the first cycle
// dispatches — non-destructively (stash, not checkout) and only for tracked
// source, never the loop's own managed dirs.

// classifyDirtyPaths splits git-status paths into those to quarantine (leaked
// tracked source) and those to ignore (the loop's own managed dirs — .evolve/
// and knowledge-base/ — whose in-flight cycle writes must never trigger a
// false-positive quarantine of the loop's own state).
func classifyDirtyPaths(paths []string) (quarantine, ignored []string) {
	for _, p := range paths {
		if isLoopManagedPath(p) {
			ignored = append(ignored, p)
			continue
		}
		quarantine = append(quarantine, p)
	}
	return quarantine, ignored
}

// isLoopManagedPath reports whether p lives under a loop-managed directory whose
// churn is normal cycle activity, not a leak to quarantine.
func isLoopManagedPath(p string) bool {
	p = strings.TrimPrefix(p, "./")
	return strings.HasPrefix(p, ".evolve/") || strings.HasPrefix(p, "knowledge-base/")
}

// QuarantineDirtyTree stashes leaked tracked-source dirt so `git status
// --porcelain` is clean for the next cycle's tree-diff guard. It is
// NON-DESTRUCTIVE: the content is preserved in a named stash (recoverable via
// `git stash pop`), never discarded. Only the classified quarantine paths are
// stashed (`-u -- <paths>`), so the loop's own managed dirs are left untouched.
// Returns stashed=false (no error) when the tree has no quarantinable dirt.
func QuarantineDirtyTree(ctx context.Context, repoRoot, label string) (bool, error) {
	dirty, err := porcelainPaths(ctx, repoRoot)
	if err != nil {
		return false, err
	}
	quarantine, _ := classifyDirtyPaths(dirty)
	if len(quarantine) == 0 {
		return false, nil
	}
	args := append([]string{"stash", "push", "--include-untracked", "-m", label, "--"}, quarantine...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("quarantine: git stash: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

// porcelainPaths returns the set of dirty paths reported by `git status
// --porcelain` in repoRoot (empty for a clean tree).
func porcelainPaths(ctx context.Context, repoRoot string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("quarantine: git status: %w", err)
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		// Porcelain v1: "XY <path>" (2 status chars + space). Renames render as
		// "old -> new" — take the destination.
		p := strings.TrimSpace(line[3:])
		if idx := strings.Index(p, " -> "); idx >= 0 {
			p = p[idx+len(" -> "):]
		}
		p = strings.Trim(p, "\"")
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// ShipSHAMismatch reports whether the on-disk ship binary's SHA-256 differs from
// expectedSHA (the SELF_SHA_TAMPERED cascade, caught at boot instead of only
// when the ship phase fails). Returns the actual on-disk SHA so the caller can
// name the drift; a matching SHA is not a false positive.
func ShipSHAMismatch(binPath, expectedSHA string) (bool, string, error) {
	data, err := os.ReadFile(binPath)
	if err != nil {
		return false, "", fmt.Errorf("ship-sha: read %s: %w", binPath, err)
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	return actual != expectedSHA, actual, nil
}
