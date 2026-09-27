package ship

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
)

func carrySatisfied(ctx context.Context, opts *Options, dir, actual string) (bool, string) {
	if opts.internalAuditArtifactSHA == "" {
		return false, ""
	}
	if dir == "" {
		dir = opts.ProjectRoot
	}
	evolveDir := filepath.Join(opts.ProjectRoot, ".evolve")
	rec, found, err := ledger.LatestCompositionVerdict(filepath.Join(evolveDir, "ledger.jsonl"), ledger.IdenticalRebaseMethod, opts.internalAuditArtifactSHA)
	if err != nil || !found || rec.AuditedTreeSHA != opts.internalAuditBoundTreeSHA || rec.TreeStateSHA != actual {
		return false, ""
	}
	if err := ledger.New(evolveDir, ledger.WithSignals(opts.Signals)).Verify(ctx); err != nil {
		return false, ""
	}
	for _, edge := range [][2]string{{rec.AuditedBase, rec.GitHead}, {rec.GitHead, "HEAD"}} {
		if !isAncestorAt(ctx, opts, dir, edge[0], edge[1]) {
			return false, ""
		}
	}
	audited, _, ok, err := treedelta.Identical(ctx, gitAt(opts), dir, rec.AuditedBase, rec.AuditedTreeSHA, rec.GitHead, actual)
	if err != nil || !ok {
		return false, ""
	}
	if patchID, err := ledger.PatchID(audited); err != nil || patchID != rec.PatchID {
		return false, ""
	}
	return true, fmt.Sprintf(" (the byte-identical rebase carry of cycle %d, re-proven)", rec.Cycle)
}

// gitAt returns git's real exit code, as treedelta's contract requires.
func gitAt(opts *Options) treedelta.Git {
	return func(ctx context.Context, dir string, args ...string) (string, int, error) {
		var out bytes.Buffer
		exit, err := opts.run(ctx, "git", append([]string{"-C", dir}, args...), &out, io.Discard)
		return out.String(), exit, err
	}
}

// isAncestorAt is the ancestry question asked in dir, where exit 1 is the answer "no", not a fault.
func isAncestorAt(ctx context.Context, opts *Options, dir, anc, desc string) bool {
	exit, err := opts.run(ctx, "git", []string{"-C", dir, "merge-base", "--is-ancestor", anc, desc}, io.Discard, io.Discard)
	return err == nil && exit == 0
}
