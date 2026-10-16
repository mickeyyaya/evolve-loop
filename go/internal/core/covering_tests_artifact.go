package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
)

const coveringTestsArtifact = "covering-tests.md"

const coveringTestsMaxBytes = 16 * 1024

func writeCoveringTests(ctx context.Context, worktree, workspace string, warn io.Writer) {
	if worktree == "" || workspace == "" {
		return
	}
	pkgs := changedGoTestPackages(changedWorktreePaths(ctx, worktree))
	pkgs = append(pkgs, changedpkgs.DirectImporters(worktree, pkgs)...)
	files := changedpkgs.CoveringTests(worktree, pkgs)
	if len(files) == 0 {
		return
	}
	body, omitted := renderCoveringTests(files)
	path := filepath.Join(workspace, coveringTestsArtifact)
	if err := atomicwrite.Bytes(path, []byte(body)); err != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN covering-tests: write %s: %v (test-amplification falls back to its own search)\n", path, err)
	}
	if omitted > 0 {
		fmt.Fprintf(warn, "[orchestrator] WARN covering-tests: corpus TRUNCATED — %d of %d covering-test paths omitted (cap %d bytes); test-amplification sees a partial corpus\n",
			omitted, len(files), coveringTestsMaxBytes)
	}
}

var coveringTestsDeriver = writeCoveringTests

func ensureCoveringTests(ctx context.Context, worktree, workspace string, warn io.Writer) {
	if worktree == "" || workspace == "" {
		return
	}
	path := filepath.Join(workspace, coveringTestsArtifact)
	if _, err := os.Stat(path); err == nil {
		return
	}
	coveringTestsDeriver(ctx, worktree, workspace, warn)
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN covering-tests: no corpus at %s — test-amplification falls back to an UNSCOPED whole-repo search; this run's context-token figure is not comparable to a scoped one\n", path)
	}
}

func renderCoveringTests(files []string) (string, int) {
	var b strings.Builder
	b.WriteString("# Covering Tests — changed packages\n\n")
	b.WriteString("Existing `_test.go` files in the packages this cycle changed. This is the\n")
	b.WriteString("in-scope test corpus: amplify against these instead of searching the repo.\n\n")
	header := b.Len()
	for i, f := range files {
		line := "- `" + sanitizeCorpusPath(f) + "`\n"
		if b.Len()+len(line) > coveringTestsMaxBytes && b.Len() > header {
			fmt.Fprintf(&b, "\n> TRUNCATED: %d of %d covering-test paths omitted (corpus exceeded %d bytes).\n",
				len(files)-i, len(files), coveringTestsMaxBytes)
			return b.String(), len(files) - i
		}
		b.WriteString(line)
	}
	return b.String(), 0
}

func sanitizeCorpusPath(p string) string {
	return strings.Map(func(r rune) rune {
		if r == '`' || unicode.IsControl(r) {
			return '_'
		}
		return r
	}, p)
}
