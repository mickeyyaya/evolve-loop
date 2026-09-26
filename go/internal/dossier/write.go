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

// Write atomically writes d to dir as cycle-N.json and cycle-N.md. With commit
// it also commits exactly that pair, so the next phase's tree-diff guard never
// sees it untracked; pass false when dir is not a git working tree.
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

// commitPair commits the pair in the repo enclosing dir, through the injectable
// commitPairGit.
func commitPair(dir, base string) error {
	return commitPairGit(gitexec.Default(dir), base)
}

// commitMaxAttempts is one try plus three retries, so a stuck lock errors
// instead of hanging cycle finalization.
const commitMaxAttempts = 4

// commitBackoffBase is the linear backoff step (attempt N sleeps N*base): long
// enough for a concurrent lane's index.lock to clear, with the whole budget
// under a second.
const commitBackoffBase = 25 * time.Millisecond

// commitPairGit stages and commits the pair by pathspec, so no unrelated staged
// change is swept in. An identical rewrite is a no-op, a transient index.lock
// failure is retried with bounded backoff, and a permanent error fails fast.
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
			unstagePair(ctx, g, jsonName, mdName)
			return lastErr
		}
		if attempt < commitMaxAttempts {
			time.Sleep(time.Duration(attempt) * commitBackoffBase)
		}
	}
	unstagePair(ctx, g, jsonName, mdName)
	return fmt.Errorf("dossier: commit %s: giving up after %d attempts: %w", base, commitMaxAttempts, lastErr)
}

// unstagePair removes the pair from the index after a failed commit, so it
// never reaches the next tree-diff guard as a phantom staged change. The reset
// is best-effort: the caller returns the commit error either way.
func unstagePair(ctx context.Context, g gitexec.Git, jsonName, mdName string) {
	_ = g.Run(ctx, "reset", "--", jsonName, mdName)
}

// commitFailure renders a commit failure with git's stderr, so the sweep can
// log the real cause.
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
