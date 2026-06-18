// Package gitexec isolates the git CLI behind one small, injectable type
// (P2/P3 of ADR-0050). It depends only on internal/sysexec — the command
// seam — so callers can fake every git invocation in the fast test tier and
// the git dependency lives in exactly one leaf package.
package gitexec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const gitBin = "git"

// Git runs git commands in Dir through the injected Exec seam. Exec must be
// non-nil — production code uses Default(dir); tests inject a fake RunFunc.
type Git struct {
	Dir  string          // git working directory; "" inherits the caller's cwd
	Exec sysexec.RunFunc // command-execution seam (required)
}

// Default returns a Git rooted at dir backed by the production runner
// (sysexec.DefaultRunner).
func Default(dir string) Git {
	return Git{Dir: dir, Exec: sysexec.DefaultRunner}
}

// Capture runs `git <args>` in g.Dir and returns stdout, stderr, and the exit
// code. A non-zero exit is reported via exitCode, NOT err — load-bearing for
// callers that branch on the code (e.g. `git diff --quiet` rc=1 means
// "differences"). err is non-nil only for unrecoverable failures.
func (g Git) Capture(ctx context.Context, args ...string) (stdout, stderr string, exitCode int, err error) {
	return sysexec.Capture(ctx, g.Exec, g.Dir, gitBin, args...)
}

// Output runs `git <args>` and returns trimmed stdout; ANY non-zero exit (or
// unrecoverable error) is folded into the returned error. Use it for queries
// where a non-zero exit IS a failure (rev-parse, describe, symbolic-ref).
func (g Git) Output(ctx context.Context, args ...string) (string, error) {
	return sysexec.Output(ctx, g.Exec, g.Dir, gitBin, args...)
}

// Run runs `git <args>` for side effects (add, commit, checkout), discarding
// stdout; a non-zero exit or unrecoverable error is returned as an error.
func (g Git) Run(ctx context.Context, args ...string) error {
	_, err := g.Output(ctx, args...)
	return err
}

// HEAD returns the trimmed commit SHA of HEAD (`git rev-parse HEAD`).
func (g Git) HEAD(ctx context.Context) (string, error) {
	return g.Output(ctx, "rev-parse", "HEAD")
}

// DirtyPaths runs `git status --porcelain -uall` and returns the sorted set of
// dirty paths: tracked-modified AND untracked files, plus the SOURCE side of any
// rename/copy (a rename dirties both paths). -uall lists every untracked file
// individually (never a bare directory), so the result is file-exact.
func (g Git) DirtyPaths(ctx context.Context) ([]string, error) {
	out, stderr, code, err := g.Capture(ctx, "status", "--porcelain", "-uall")
	if err != nil {
		return nil, fmt.Errorf("gitexec: git status: %w", err)
	}
	if code != 0 {
		return nil, fmt.Errorf("gitexec: git status exit=%d: %s", code, strings.TrimSpace(stderr))
	}
	set := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		set[PorcelainPath(line)] = true
		if old := PorcelainOldPath(line); old != "" {
			set[old] = true
		}
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}

// PorcelainPath extracts the path from a `git status --porcelain` line. Lines are
// "XY <path>"; a rename/copy is "XY <old> -> <new>" (take the new path). Quotes
// (paths with special chars) are trimmed best-effort. A line too short to hold a
// path returns "" rather than panicking.
func PorcelainPath(line string) string {
	if len(line) < 4 {
		return ""
	}
	p := strings.TrimSpace(line[3:])
	if i := strings.Index(p, " -> "); i >= 0 {
		p = p[i+4:]
	}
	return strings.Trim(p, "\"")
}

// PorcelainOldPath extracts the rename/copy SOURCE from a porcelain line
// ("XY <old> -> <new>"), or "" for non-rename lines (or lines too short).
func PorcelainOldPath(line string) string {
	if len(line) < 4 {
		return ""
	}
	p := strings.TrimSpace(line[3:])
	i := strings.Index(p, " -> ")
	if i < 0 {
		return ""
	}
	return strings.Trim(p[:i], "\"")
}

// WorktreeToken returns a short, stable, git-ref-safe discriminator derived from
// the absolute project root. Cycle worktree BRANCH names must embed it because
// git worktree branch names are GLOBAL to one object store: sibling worktrees of
// the same repo (e.g. several concurrent `evolve loop` runs, each on its own
// branch) share a single branch namespace, so a plain `cycle-<N>` branch created
// by the first run makes every other run's `git worktree add -B cycle-<N>` fail
// with "already used by worktree", silently degrading them into the main tree.
// The token is a pure function of the root: distinct roots yield distinct tokens
// (concurrent runs never clash) while one root is stable across resume/retry
// (the same cycle reuses its branch). Path variants of one dir (trailing slash,
// relative form) normalize to the same token.
//
// The root is absolutized via filepath.Abs; that can fail only when os.Getwd
// fails (effectively never in a live process), in which case the token falls
// back to the cleaned input verbatim — so callers MUST pass an absolute root for
// a stable token (the orchestrator already does).
func WorktreeToken(projectRoot string) string {
	root := filepath.Clean(projectRoot)
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:4]) // 8 hex chars: collision-safe per repo set
}
