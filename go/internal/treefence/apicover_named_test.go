package treefence

import (
	"context"
	"strings"
	"testing"
)

// TestAPICoverNamedExports names and EXERCISES every exported symbol of this
// package (ADR-0069 new-package graduation) through the shapes its consumers
// — the phase runner's and the retro phase's worktree fence — rely on: the
// one-shot Take/Restore pair, and the Begin/End lifecycle with its Outcome
// rendered as diagnostics.
func TestAPICoverNamedExports(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := initRepo(t)

	// Take + Snapshot.Restore → Result.
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	var _ Snapshot = snap
	res, err := snap.Restore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var _ Result = res
	if snap.Worktree != dir || snap.Tree == "" || len(res.Restored) != 0 {
		t.Fatalf("unexpected shapes: %+v %+v", snap, res)
	}

	// Begin/End lifecycle: inert for a source writer, live for a read-only phase.
	var inert *Fence = Begin(ctx, dir, false)
	if inert.TakeErr() != nil || len(inert.End(ctx).Diagnostics("build")) != 0 {
		t.Fatal("a source writer's fence must be inert")
	}
	live := Begin(ctx, dir, true)
	if live.TakeErr() != nil {
		t.Fatalf("live fence: %v", live.TakeErr())
	}
	write(t, dir, "src/mat.go", "mutated\n")
	var out Outcome = live.End(ctx)
	diags := out.Diagnostics("audit")
	if len(out.Restored) != 1 || out.RestoreErr != nil || len(diags) != 1 ||
		!strings.Contains(diags[0].Message, "read-only phase audit wrote 1 path(s)") || diags[0].Severity != "warning" {
		t.Fatalf("outcome=%+v diags=%+v", out, diags)
	}

	// An untakeable fence reports itself and never errors the phase.
	broken := Begin(ctx, t.TempDir(), true)
	if broken.TakeErr() == nil {
		t.Fatal("a non-repository must be an untakeable fence")
	}
	if d := broken.End(ctx).Diagnostics("audit"); len(d) != 1 || !strings.Contains(d[0].Message, "snapshot unavailable") {
		t.Fatalf("untakeable fence diagnostics = %+v", d)
	}
	if got := (Outcome{RestoreErr: context.Canceled}).Diagnostics("retro"); len(got) != 1 || !strings.Contains(got[0].Message, "restore failed") {
		t.Fatalf("restore failure diagnostics = %+v", got)
	}
	if (*Fence)(nil).TakeErr() != nil || len((*Fence)(nil).End(ctx).Restored) != 0 {
		t.Fatal("a nil fence is inert")
	}
}
