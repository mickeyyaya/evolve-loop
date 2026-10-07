package ship

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
)

const codeCarryNotReProven signalcenter.Code = "SHIP_CARRY_NOT_REPROVEN"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, codeCarryNotReProven, "an identical-rebase carry record named the bound audit but ship could not re-prove it; the reason names the failed check (record, ledger chain, ancestry, tree bytes or patch-id) and the change is audited again; fields.carry_cycle")
}

func carrySatisfied(ctx context.Context, opts *Options, dir, actual string) (bool, string) {
	rec, found, reason := boundCarryRecord(opts, actual)
	if reason == "" {
		reason = reProveCarry(ctx, opts, heldCarry{dir: dir, tree: actual, record: rec})
	}
	if reason == "" {
		return true, fmt.Sprintf(" (the byte-identical rebase carry of cycle %d, re-proven)", rec.Cycle)
	}
	if found {
		opts.Signals.Emit(signalcenter.Event{
			Cycle: opts.CycleID, RunID: opts.RunID, Phase: "ship", Module: signalcenter.ModuleShip, Origin: "carrySatisfied",
			Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: codeCarryNotReProven,
			Reason: "carry not re-proven: " + reason, Fields: map[string]string{"carry_cycle": strconv.Itoa(rec.Cycle)},
		})
	}
	return false, reason
}

func boundCarryRecord(opts *Options, actual string) (ledger.CompositionVerdict, bool, string) {
	ref := opts.internalAuditArtifactSHA
	if ref == "" {
		return ledger.CompositionVerdict{}, false, "the audit binding names no audit artifact"
	}
	rec, found, err := ledger.LatestCompositionVerdict(filepath.Join(opts.ProjectRoot, ".evolve", "ledger.jsonl"), ledger.IdenticalRebaseMethod, ref)
	switch {
	case err != nil:
		return rec, false, "the ledger is unreadable: " + err.Error()
	case !found:
		return rec, false, "no identical-rebase carry names audit " + ref
	case rec.AuditedTreeSHA != opts.internalAuditBoundTreeSHA:
		return rec, true, fmt.Sprintf("the carry of cycle %d names the audited tree %s, not the bound %s", rec.Cycle, rec.AuditedTreeSHA, opts.internalAuditBoundTreeSHA)
	case rec.TreeStateSHA != actual:
		return rec, true, fmt.Sprintf("the carry of cycle %d names the tree %s, not %s", rec.Cycle, rec.TreeStateSHA, actual)
	}
	return rec, true, ""
}

type heldCarry struct {
	dir, tree string
	record    ledger.CompositionVerdict
}

func reProveCarry(ctx context.Context, opts *Options, held heldCarry) string {
	rec, dir := held.record, held.dir
	if dir == "" {
		dir = opts.ProjectRoot
	}
	if err := ledger.New(filepath.Join(opts.ProjectRoot, ".evolve"), ledger.WithSignals(opts.Signals)).Verify(ctx); err != nil {
		return "the ledger does not verify: " + err.Error()
	}
	for _, edge := range [][2]string{{rec.AuditedBase, rec.GitHead}, {rec.GitHead, "HEAD"}} {
		if !isAncestorAt(ctx, opts, dir, edge[0], edge[1]) {
			return fmt.Sprintf("%s is not an ancestor of %s", edge[0], edge[1])
		}
	}
	audited, _, ok, err := treedelta.Identical(ctx, gitAt(opts), dir, rec.AuditedBase, rec.AuditedTreeSHA, rec.GitHead, held.tree)
	if err != nil || !ok {
		return fmt.Sprintf("the change on %s is not byte for byte the audited change (err=%v)", rec.GitHead, err)
	}
	if patchID, err := ledger.PatchID(audited); err != nil || patchID != rec.PatchID {
		return fmt.Sprintf("the re-derived patch-id %s is not the record's %s (err=%v)", patchID, rec.PatchID, err)
	}
	return ""
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
