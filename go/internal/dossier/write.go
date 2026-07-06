package dossier

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// Write persists d to dir as cycle-N.json and cycle-N.md using atomic
// temp+rename via atomicwrite.Bytes. When commit is true, the two files are
// then git-added and committed (scoped to just those paths) in the repo that
// contains dir, so the closeout dossier lands as the "ONE committed artifact"
// this package promises — leaving the main tree clean instead of tripping the
// next phase's tree-diff guard on the untracked pair. commit==false writes the
// files only (git untouched); use it when dir is not a git working tree.
func Write(d *Dossier, dir string, commit bool) error {
	if dir == "" {
		return fmt.Errorf("dossier: Write: dir must not be blank")
	}
	base := fmt.Sprintf("cycle-%d", d.Cycle)

	jsonBytes, err := RenderJSON(d)
	if err != nil {
		return fmt.Errorf("dossier: Write: %w", err)
	}
	if err := atomicwrite.Bytes(filepath.Join(dir, base+".json"), jsonBytes); err != nil {
		return fmt.Errorf("dossier: Write JSON: %w", err)
	}

	mdBytes, err := RenderMarkdown(d)
	if err != nil {
		return fmt.Errorf("dossier: Write: %w", err)
	}
	if err := atomicwrite.Bytes(filepath.Join(dir, base+".md"), mdBytes); err != nil {
		return fmt.Errorf("dossier: Write markdown: %w", err)
	}

	if commit {
		if err := commitPair(dir, base); err != nil {
			return fmt.Errorf("dossier: Write commit: %w", err)
		}
	}
	return nil
}

// commitPair stages and commits cycle-<base>.{json,md} in the git repo enclosing
// dir. Thin wrapper over commitPairGit with the production git runner; kept so
// existing callers (Write) pass a dir, while the retry/backoff logic lives in
// one injectable place the fast test tier can drive without racing real locks.
func commitPair(dir, base string) error {
	return commitPairGit(gitexec.Default(dir), base)
}

// commitMaxAttempts bounds the transient-lock retry: 1 initial + 3 retries. A
// genuinely stuck lock surfaces as an error in a small, fixed number of tries
// rather than hanging cycle finalization (cycle-564 write_retry_test bounds).
const commitMaxAttempts = 4

// commitBackoffBase is the linear per-attempt backoff step (attempt N sleeps
// N*base) — enough to let a concurrent fleet lane's index.lock clear, small
// enough that the whole bounded budget stays well under a second.
const commitBackoffBase = 25 * time.Millisecond

// commitPairGit stages and commits cycle-<base>.{json,md} in g's repo, scoped by
// pathspec so no unrelated staged change is swept in. A re-write with identical
// content (nothing staged) is a no-op, never an empty commit. A transient git
// index.lock failure on commit — the cycle-564 root cause: concurrent fleet
// lanes sharing one repo contend on .git/index.lock, which the old un-retried
// commitPair swallowed, permanently orphaning 9 recorded cycles' dossiers — is
// retried with bounded linear backoff. A non-lock (permanent) error fails fast.
func commitPairGit(g gitexec.Git, base string) error {
	ctx := context.Background()
	jsonName, mdName := base+".json", base+".md"

	if err := g.Run(ctx, "add", "--", jsonName, mdName); err != nil {
		return fmt.Errorf("dossier: git add %s: %w", base, err)
	}
	// diff --cached exit 0 == nothing staged for these paths (identical rewrite).
	if _, _, code, err := g.Capture(ctx, "diff", "--cached", "--quiet", "--", jsonName, mdName); err != nil {
		return fmt.Errorf("dossier: git diff %s: %w", base, err)
	} else if code == 0 {
		return nil
	}

	msg := fmt.Sprintf("dossier: %s closeout", base)
	var lastErr error
	for attempt := 1; attempt <= commitMaxAttempts; attempt++ {
		_, stderr, code, err := g.Capture(ctx, "commit", "-m", msg, "--", jsonName, mdName)
		if err == nil && code == 0 {
			return nil
		}
		lastErr = commitFailure(base, code, stderr, err)
		if !isTransientGitLock(stderr) {
			return lastErr // permanent error → don't burn the retry budget on an unwinnable failure
		}
		if attempt < commitMaxAttempts {
			time.Sleep(time.Duration(attempt) * commitBackoffBase)
		}
	}
	return fmt.Errorf("dossier: commit %s: giving up after %d attempts: %w", base, commitMaxAttempts, lastErr)
}

// commitFailure renders a commit failure, carrying the underlying git stderr so
// the sweep can log it loudly instead of silently swallowing it (AC3).
func commitFailure(base string, code int, stderr string, err error) error {
	if err != nil {
		return fmt.Errorf("dossier: git commit %s: %w", base, err)
	}
	return fmt.Errorf("dossier: git commit %s: exit %d: %s", base, code, strings.TrimSpace(stderr))
}

// isTransientGitLock reports whether git stderr names an index.lock contention —
// the recoverable, retry-worthy failure mode under concurrent fleet-lane commits.
func isTransientGitLock(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "index.lock") ||
		(strings.Contains(s, "unable to create") && strings.Contains(s, ".lock"))
}
