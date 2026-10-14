package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

type removalClaim struct {
	RemovedPaths []string `json:"removedPaths"`
}

func RemovalClaimFailures(ctx context.Context, in ReviewInput) []string {
	if in.Workspace == "" || in.Worktree == "" {
		return nil
	}
	body, ok := readBuildReport(in.Workspace)
	if !ok {
		return nil
	}
	var failures []string
	for _, claimed := range parseRemovalClaims(body) {
		abs, ok := resolveInWorktree(in.Worktree, claimed)
		if !ok {
			continue
		}
		if _, err := os.Stat(abs); err != nil {
			if !trackedInGitIndex(ctx, in.Worktree, claimed) {
				continue
			}
			failures = append(failures, fmt.Sprintf(
				"build report claims %q was removed, but it still exists in the Git index — retire it as a tracked deletion or correct the claim",
				claimed))
			continue
		}
		failures = append(failures, fmt.Sprintf(
			"build report claims %q was removed, but it still exists in the worktree — remove it or correct the claim (false removal claims are the cycle-660 class)",
			claimed))
	}
	return failures
}

func readBuildReport(workspace string) (string, bool) {
	name := phasecontract.ArtifactFilename(string(PhaseBuild))
	for _, rel := range []string{name, filepath.Join("deliverables", name)} {
		if b, err := os.ReadFile(filepath.Join(workspace, rel)); err == nil {
			return string(b), true
		}
	}
	return "", false
}

func parseRemovalClaims(body string) []string {
	var out []string
	for _, block := range fencedJSONBlocks(body) {
		var claim removalClaim
		if err := json.Unmarshal([]byte(block), &claim); err != nil {
			continue
		}
		for _, p := range claim.RemovedPaths {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func fencedJSONBlocks(body string) []string {
	var blocks []string
	var cur []string
	inBlock := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			if strings.HasPrefix(trimmed, "```") && strings.EqualFold(strings.TrimPrefix(trimmed, "```"), "json") {
				inBlock, cur = true, nil
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			blocks = append(blocks, strings.Join(cur, "\n"))
			inBlock = false
			continue
		}
		cur = append(cur, line)
	}
	return blocks
}

func resolveInWorktree(worktree, claimed string) (string, bool) {
	if filepath.IsAbs(claimed) {
		return "", false
	}
	abs := filepath.Clean(filepath.Join(worktree, claimed))
	rel, err := filepath.Rel(worktree, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return abs, true
}

func trackedInGitIndex(ctx context.Context, worktree, path string) bool {
	_, code, gitErr := gitCapture(ctx, worktree, "ls-files", "--error-unmatch", "--", path)
	return gitErr == nil && code == 0
}
