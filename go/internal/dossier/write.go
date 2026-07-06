package dossier

import (
	"context"
	"fmt"
	"path/filepath"

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
// dir, scoped by pathspec so no unrelated staged change is swept in. A re-write
// with identical content (nothing staged) is a no-op, never an empty commit.
func commitPair(dir, base string) error {
	ctx := context.Background()
	g := gitexec.Default(dir)
	jsonName, mdName := base+".json", base+".md"

	if err := g.Run(ctx, "add", "--", jsonName, mdName); err != nil {
		return err
	}
	// diff --cached exit 0 == nothing staged for these paths (identical rewrite).
	if _, _, code, err := g.Capture(ctx, "diff", "--cached", "--quiet", "--", jsonName, mdName); err != nil {
		return err
	} else if code == 0 {
		return nil
	}
	msg := fmt.Sprintf("dossier: %s closeout", base)
	return g.Run(ctx, "commit", "-m", msg, "--", jsonName, mdName)
}
