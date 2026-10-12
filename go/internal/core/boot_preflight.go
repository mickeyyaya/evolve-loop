package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

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
	return p == "go/bin/evolve" ||
		strings.HasPrefix(p, ".evolve/") || strings.HasPrefix(p, "knowledge-base/")
}

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
	args = append(args, ":(exclude)go/bin/evolve")
	cmd := sysexec.Command(ctx, "git", args...)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("quarantine: git stash: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

func porcelainPaths(ctx context.Context, repoRoot string) ([]string, error) {
	cmd := sysexec.Command(ctx, "git", "status", "--porcelain")
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
		p := porcelainDestination(strings.TrimSpace(line[3:]))
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

func ShipSHAMismatch(binPath, expectedSHA string) (bool, string, error) {
	data, err := os.ReadFile(binPath)
	if err != nil {
		return false, "", fmt.Errorf("ship-sha: read %s: %w", binPath, err)
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	return actual != expectedSHA, actual, nil
}

func porcelainDestination(entry string) string {
	const renameArrow = " -> "
	if idx := strings.Index(entry, renameArrow); idx >= 0 {
		entry = entry[idx+len(renameArrow):]
	}
	return strings.Trim(entry, "\"")
}
