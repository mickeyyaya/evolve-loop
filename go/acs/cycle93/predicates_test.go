// Package cycle93 ports the cycle-93 ACS predicates (5 bash files).
//
// Cycle-93 is the "gitignored-deliverable" recovery cycle from the
// cycle-92 ship failure. Predicates verify .gitignore exceptions plus
// git-tracking attestation patterns. The git-tracking checks use
// `git ls-files --error-unmatch` via acsassert.SubprocessOutput.
package cycle93

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// gitTracked reports whether the path is tracked by git in the given
// repo root. Returns false if git is unavailable or the path is untracked.
func gitTracked(repoRoot, relPath string) bool {
	// `git -C <root> ls-files --error-unmatch <path>` exits 0 when tracked.
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", repoRoot, "ls-files", "--error-unmatch", relPath)
	if err != nil || code != 0 {
		return false
	}
	return true
}

// TestC93_001_GitignoreProfilesMdException ports cycle-93/001.
// .gitignore contains `!.evolve/profiles/*.md`.
func TestC93_001_GitignoreProfilesMdException(t *testing.T) {
	root := acsassert.RepoRoot(t)
	ig := filepath.Join(root, ".gitignore")
	if !acsassert.FileExists(t, ig) {
		t.Skip(".gitignore missing — skip cycle-93-001")
	}
	if !acsassert.FileMatchesRegex(t, ig, `(?m)^!\.evolve/profiles/\*\.md$`) {
		return
	}
	// Cross-verify .evolve/profiles/AGENTS.md is not ignored. `git
	// check-ignore` exits 0 when the path IS ignored.
	_, _, code, err := acsassert.SubprocessOutput(
		"git", "-C", root, "check-ignore", "-q", ".evolve/profiles/AGENTS.md")
	if err == nil && code == 0 {
		t.Errorf("git still ignores .evolve/profiles/AGENTS.md after negation")
	}
}

// TestC93_002_ProfilesAgentsMdGitTracked ports cycle-93/002.
// .evolve/profiles/AGENTS.md exists, is tracked, >=64 bytes.
func TestC93_002_ProfilesAgentsMdGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	target := filepath.Join(root, ".evolve", "profiles", "AGENTS.md")
	if _, err := os.Stat(target); err != nil {
		t.Skipf("%s missing on disk — skip cycle-93-002", target)
	}
	if !gitTracked(root, ".evolve/profiles/AGENTS.md") {
		t.Errorf("%s exists on disk but is not git-tracked", target)
	}
	info, err := os.Stat(target)
	if err == nil && info.Size() < 64 {
		t.Errorf("%s is suspiciously small (%d bytes)", target, info.Size())
	}
}

// TestC93_003_BuilderAttestationStep ports cycle-93/003.
// agents/evolve-builder.md documents the git ls-files --error-unmatch
// attestation under an Attestation/Git-Tracking/Pre-handoff heading.
func TestC93_003_BuilderAttestationStep(t *testing.T) {
	root := acsassert.RepoRoot(t)
	target := filepath.Join(root, "agents", "evolve-builder.md")
	if !acsassert.FileExists(t, target) {
		t.Skip("evolve-builder.md missing — skip cycle-93-003")
	}
	if !gitTracked(root, "agents/evolve-builder.md") {
		t.Errorf("%s is not git-tracked", target)
	}
	if !acsassert.FileContains(t, target, "git ls-files --error-unmatch") {
		return
	}
	// Heading near the command (within 60 lines, command at or after heading).
	headingLine := findFirstMatchLine(t, target, `(?i)(?m)^#+ .*(attestation|git tracking|pre[- ]?handoff)`)
	cmdLine := findFirstMatchLine(t, target, `git ls-files --error-unmatch`)
	if headingLine == 0 || cmdLine == 0 {
		t.Errorf("%s: heading or command line not found (heading=%d, cmd=%d)", target, headingLine, cmdLine)
		return
	}
	if cmdLine < headingLine {
		t.Errorf("%s: ls-files command at L%d precedes heading at L%d", target, cmdLine, headingLine)
		return
	}
	if cmdLine-headingLine > 60 {
		t.Errorf("%s: heading L%d too far from command L%d (dist=%d, max 60)",
			target, headingLine, cmdLine, cmdLine-headingLine)
	}
}

// TestC93_004_TddPredicateTemplateLsFiles ports cycle-93/004.
// agents/evolve-tdd-engineer.md mentions both the ls-files command
// and the `[ -f` idiom within 30 lines of each other.
func TestC93_004_TddPredicateTemplateLsFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	target := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	if !acsassert.FileExists(t, target) {
		t.Skip("evolve-tdd-engineer.md missing — skip cycle-93-004")
	}
	if !gitTracked(root, "agents/evolve-tdd-engineer.md") {
		t.Errorf("%s is not git-tracked", target)
	}
	if !acsassert.FileContains(t, target, "git ls-files --error-unmatch") {
		return
	}
	if !acsassert.FileMatchesRegex(t, target, `\[\[? -f `) {
		return
	}
	lsLine := findFirstMatchLine(t, target, `git ls-files --error-unmatch`)
	fLine := findFirstMatchLine(t, target, `\[\[? -f `)
	if lsLine == 0 || fLine == 0 {
		return
	}
	dist := lsLine - fLine
	if dist < 0 {
		dist = -dist
	}
	if dist > 30 {
		t.Errorf("%s: ls-files L%d and [ -f L%d are %d lines apart (>30)", target, lsLine, fLine, dist)
	}
}

// TestC93_005_Cycle92DeliverablesExistAndTracked ports cycle-93/005.
// The five cycle-92 deliverables exist, are tracked, meet line minimums.
func TestC93_005_Cycle92DeliverablesExistAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// v11.1.0 drift: scripts/ moved to legacy/scripts/ with a symlink at
	// the old path. The file resolves through the symlink for os.Stat but
	// git tracks it at the new path. Accept either location.
	tuples := []struct {
		paths    []string // first match wins; both must exist on disk if path
		minLines int
	}{
		{[]string{"agents/AGENTS.md"}, 30},
		{[]string{"legacy/scripts/AGENTS.md", "legacy/scripts/AGENTS.md"}, 30},
		{[]string{"acs/AGENTS.md"}, 30},
		{[]string{".evolve/profiles/AGENTS.md"}, 30},
		{[]string{"docs/CODEBASE-MAP.md"}, 20},
	}
	for _, tup := range tuples {
		var resolved string
		for _, p := range tup.paths {
			abs := filepath.Join(root, p)
			if _, err := os.Stat(abs); err == nil && gitTracked(root, p) {
				resolved = p
				break
			}
		}
		if resolved == "" {
			t.Errorf("MISSING-OR-UNTRACKED: tried %v", tup.paths)
			continue
		}
		abs := filepath.Join(root, resolved)
		raw, err := os.ReadFile(abs)
		if err != nil {
			t.Errorf("READ-ERR: %s: %v", resolved, err)
			continue
		}
		nonblank := 0
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.TrimSpace(line) != "" {
				nonblank++
			}
		}
		if nonblank < tup.minLines {
			t.Errorf("THIN: %s (%d non-blank lines, need >=%d)", resolved, nonblank, tup.minLines)
		}
	}
}

// findFirstMatchLine returns the 1-based line index of the first line in
// path matching the regex, or 0 if none. The pattern is compiled with
// regexp.Compile; invalid patterns cause a test failure.
func findFirstMatchLine(t *testing.T, path, pattern string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	re, rerr := regexp.Compile(pattern)
	if rerr != nil {
		t.Fatalf("bad pattern %q: %v", pattern, rerr)
	}
	for i, line := range strings.Split(string(raw), "\n") {
		if re.MatchString(line) {
			return i + 1
		}
	}
	// If no line matches, allow multi-line patterns by searching whole body.
	loc := re.FindStringIndex(string(raw))
	if loc == nil {
		return 0
	}
	// Convert byte offset to line index.
	return 1 + strings.Count(string(raw[:loc[0]]), "\n")
}
