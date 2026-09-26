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

// carrySatisfied is ADR-0105 B4. The tree ship holds is not the audited one, but a carry record of this
// audit says the audited change was rebased byte for byte onto the base it now sits on; ship verifies the
// ledger chain the record sits in, then re-proves the ancestry and the bytes itself. The writer refuses a
// record whose gates are not green, so a chained record's gates need no second look.
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

// gitAt runs git in the directory it is given and returns its real exit code, as treedelta's contract asks.
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
