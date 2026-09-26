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

// classifyDirtyPaths splits dirty paths into leaked tracked source to quarantine and loop-managed paths to ignore.
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

func isLoopManagedPath(p string) bool {
	p = strings.TrimPrefix(p, "./")
	// Boot recovery re-pins the ship binary in the same pass; stashing it would reopen the SHA mismatch the repin heals.
	return p == "go/bin/evolve" ||
		strings.HasPrefix(p, ".evolve/") || strings.HasPrefix(p, "knowledge-base/")
}

// QuarantineDirtyTree moves leaked tracked-source dirt into a named stash, never discarding it or touching loop-managed dirs.
func QuarantineDirtyTree(ctx context.Context, repoRoot, label string) (bool, error) {
	dirty, err := porcelainPaths(ctx, repoRoot)
	if err != nil {
		return false, err
	}
	quarantine, _ := classifyDirtyPaths(dirty)
	if len(quarantine) == 0 {
		return false, nil
	}
	// git may report a collapsed untracked parent dir that classifyDirtyPaths cannot exclude, so the pathspec
	// excludes the ship binary too.
	args := append([]string{"stash", "push", "--include-untracked", "-m", label, "--"}, quarantine...)
	args = append(args, ":(exclude)go/bin/evolve")
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("quarantine: git stash: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

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
		// Porcelain v1 is "XY <path>"; a rename renders as "old -> new", so take the destination.
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

// ShipSHAMismatch reports whether the ship binary's SHA-256 differs from expectedSHA, and returns the actual SHA.
func ShipSHAMismatch(binPath, expectedSHA string) (bool, string, error) {
	data, err := os.ReadFile(binPath)
	if err != nil {
		return false, "", fmt.Errorf("ship-sha: read %s: %w", binPath, err)
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	return actual != expectedSHA, actual, nil
}
