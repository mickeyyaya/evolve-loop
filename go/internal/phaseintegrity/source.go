// Package phaseintegrity provides the production DigestSource for the per-phase
// integrity chain (ADR-0065): it reads the running binary, the binary's
// build-commit, the phase's agent profile/prompt, the phase's report artifact,
// and the worktree tree, and feeds them to phaseblock.Compute. It is the
// non-leaf glue (it shells git for the tree sha), kept separate from the
// git-free phaseblock leaf.
package phaseintegrity

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/selfsha"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

// gitTreeTimeout bounds the git write-tree exec so a slow/huge worktree or a
// stuck filesystem cannot hang the phase boundary.
const gitTreeTimeout = 30 * time.Second

// Source implements phaseblock.DigestSource from on-disk paths. Empty path
// fields are skipped (the digest field stays ""); a set-but-missing profile or
// report file is best-effort ("" rather than an error — capture never blocks a
// cycle). GitTree is the seam for the worktree tree sha (default: git write-tree);
// tests inject a deterministic stub.
type Source struct {
	BinaryPath   string // "" → the running executable (selfsha.Running)
	ProfilePath  string // "" → no profile sha
	ReportPath   string // "" → no report sha
	WorktreePath string // "" → no tree sha
	GitTree      func(worktree string) (string, error)
}

// BinarySHA hashes the binary at BinaryPath, or the running executable.
func (s Source) BinarySHA() (string, error) {
	if s.BinaryPath == "" {
		return selfsha.Running()
	}
	return selfsha.Of(s.BinaryPath)
}

// BinaryCommit returns the running binary's embedded build-commit (provenance).
func (s Source) BinaryCommit() string { return version.Commit() }

// ProfileSHA hashes the agent profile/prompt path, best-effort.
func (s Source) ProfileSHA() (string, error) { return shaOrEmpty(s.ProfilePath) }

// ReportSHA hashes the phase report artifact, best-effort.
func (s Source) ReportSHA() (string, error) { return shaOrEmpty(s.ReportPath) }

// TreeSHA returns the worktree's staged-index tree sha (git write-tree), or ""
// when the phase ran without a per-cycle worktree. Note: this is the STAGED
// index identity, not the full working tree — a phase that does not stage its
// output is not reflected.
func (s Source) TreeSHA() (string, error) {
	if s.WorktreePath == "" {
		return "", nil
	}
	if s.GitTree != nil {
		return s.GitTree(s.WorktreePath)
	}
	return defaultGitTree(s.WorktreePath)
}

// shaOrEmpty hashes path. An empty path or a genuinely-absent file yields ""
// (best-effort: a not-yet-written artifact is not a capture failure). Any OTHER
// stat error (permission denied, bad path) is surfaced — silently treating it
// as "" would record the same digest as "no artifact", a silent integrity gap.
func shaOrEmpty(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("phaseintegrity: stat %s: %w", path, err)
	}
	return selfsha.Of(path)
}

// defaultGitTree returns the staged-index tree object sha of a worktree, with a
// bounded timeout. The worktree must be an absolute path (internally always
// is) — guarding it keeps a leading-dash path from being read as a git flag.
func defaultGitTree(worktree string) (string, error) {
	if !filepath.IsAbs(worktree) {
		return "", fmt.Errorf("phaseintegrity: worktree path must be absolute, got %q", worktree)
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTreeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", worktree, "write-tree").Output()
	if err != nil {
		return "", fmt.Errorf("phaseintegrity: git write-tree in %s: %w", worktree, err)
	}
	return strings.TrimSpace(string(out)), nil
}
