package core

// build_removal_check.go — deterministic truth-check for the removal claims a
// build report makes about its own worktree (inbox item `tdd-topn-binding-gate`,
// acceptance criterion 2: "a build report claiming a removal that did not happen
// fails build-selfcheck deterministically").
//
// The cycle-660 incident this closes: build-report.md asserted that orphaned RED
// scaffolds were "already removed by a concurrent actor" while the files were
// still sitting in the worktree, and the false claim passed review undetected —
// the prose was the only evidence and nobody checked the tree. A claim about
// tree state is machine-checkable, so it is checked by machine, not read.
//
// Prose is deliberately NOT parsed: a natural-language matcher on "removed"
// would false-block honest reports and is exactly the proxy-signal failure this
// gate exists to end. The claim surface is a structured fenced ```json block
// carrying a "removedPaths" array (mirroring the fenced-JSON handoff contract
// topngate already uses); anything else is invisible to this check.
//
// Every ambiguity is fail-open (nil): no workspace/worktree, no report, no
// parseable block, malformed JSON, an empty list, or a path escaping the
// worktree. The floor can never false-block a build over its own plumbing —
// downstream deterministic gates (ACS toolchain, apicover, CI) stay armed.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// removalClaim is the structured claim block a build report emits. Unrelated
// fenced JSON (e.g. topngate's testFiles handoff) unmarshals with a nil
// RemovedPaths and is skipped.
type removalClaim struct {
	RemovedPaths []string `json:"removedPaths"`
}

// RemovalClaimFailures is a BuildFloorCheckFn: it reads the build report,
// collects every path the report claims to have removed, and returns one
// failure line per claimed path that STILL EXISTS under the worktree.
func RemovalClaimFailures(_ context.Context, in ReviewInput) []string {
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
			continue // absolute or escaping path — not ours to adjudicate
		}
		if _, err := os.Stat(abs); err != nil {
			continue // genuinely gone (or unstattable) — honest claim
		}
		failures = append(failures, fmt.Sprintf(
			"build report claims %q was removed, but it still exists in the worktree — remove it or correct the claim (false removal claims are the cycle-660 class)",
			claimed))
	}
	return failures
}

// readBuildReport loads the build report from its canonical location, falling
// back to the promoted copy the correction ladder writes under deliverables/.
func readBuildReport(workspace string) (string, bool) {
	for _, rel := range []string{"build-report.md", filepath.Join("deliverables", "build-report.md")} {
		if b, err := os.ReadFile(filepath.Join(workspace, rel)); err == nil {
			return string(b), true
		}
	}
	return "", false
}

// parseRemovalClaims extracts every removedPaths entry from the report's fenced
// ```json blocks. Unparseable blocks are skipped, never fatal.
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

// fencedJSONBlocks returns the bodies of every ```json fenced block in body.
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

// resolveInWorktree maps a claimed worktree-relative path to an absolute path,
// rejecting absolute paths and any path that escapes the worktree.
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
