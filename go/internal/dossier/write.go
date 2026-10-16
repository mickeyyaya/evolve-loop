package dossier

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

var renderMarkdown = RenderMarkdown

func Write(d *Dossier, dir string, commit bool) error {
	if d == nil {
		return fmt.Errorf("dossier: Write: dossier must not be nil")
	}
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

	mdBytes, err := renderMarkdown(d)
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

func commitPair(dir, base string) error {
	return commitPairGit(gitexec.Default(dir), base)
}

const commitMaxAttempts = 4

const commitBackoffBase = 25 * time.Millisecond

func commitPairGit(g gitexec.Git, base string) error {
	ctx := context.Background()
	jsonName, mdName := base+".json", base+".md"

	if err := g.Run(ctx, "add", "--", jsonName, mdName); err != nil {
		return fmt.Errorf("dossier: git add %s: %w", base, err)
	}
	if _, _, code, err := g.Capture(ctx, "diff", "--cached", "--quiet", "--", jsonName, mdName); err != nil {
		return fmt.Errorf("dossier: git diff %s: %w", base, err)
	} else if code == 0 {
		return nil
	}

	msg := fmt.Sprintf("dossier: %s closeout", path.Base(base))
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

func unstagePair(ctx context.Context, g gitexec.Git, jsonName, mdName string) {
	_ = g.Run(ctx, "reset", "--", jsonName, mdName)
}

func commitFailure(base string, code int, stderr string, err error) error {
	if err != nil {
		return fmt.Errorf("dossier: git commit %s: %w", base, err)
	}
	return fmt.Errorf("dossier: git commit %s: exit %d: %s", base, code, strings.TrimSpace(stderr))
}

func isTransientGitLock(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "index.lock") ||
		(strings.Contains(s, "unable to create") && strings.Contains(s, ".lock"))
}
