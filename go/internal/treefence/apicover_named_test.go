package treefence

import (
	"context"
	"strings"
	"testing"
)

func TestAPICoverNamedExports(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := initRepo(t)

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

	staged := initRepo(t)
	base, err := TakeStaged(ctx, staged, nil)
	if err != nil {
		t.Fatal(err)
	}
	full, err := Take(ctx, staged)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := full.Differing(ctx, base); err != nil || len(got) != 1 || got[0] != "src/new_test.go" {
		t.Fatalf("Differing = %v %v, want [src/new_test.go]", got, err)
	}
	if _, err := base.Restore(ctx); err == nil {
		t.Fatal("a staged snapshot must not restore a worktree")
	}
}

func TestAPICoverNamedGrantExports(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := initRepo(t)
	fence := Begin(ctx, dir, true, "src/mat.go").WithWritableDirs("src/gen", "src/gen/")
	write(t, dir, "src/gen/new.go", "new\n")
	write(t, dir, "src/stray.go", "stray\n")

	var out Outcome = fence.End(ctx)

	diags := out.Diagnostics("build")
	var errs []string
	for _, d := range diags {
		if d.Severity == "error" {
			errs = append(errs, d.Message)
		}
	}
	if !out.Writer || !out.Verified || len(out.Created) != 1 || out.Created[0] != "src/stray.go" || len(out.Kept) != 1 || out.Kept[0] != "src/gen/new.go" {
		t.Fatalf("outcome %+v", out)
	}
	if len(errs) != 1 || !strings.Contains(errs[0], "src/stray.go") || !strings.Contains(errs[0], "--writable") || strings.Contains(errs[0], "src/gen/new.go") {
		t.Fatalf("error diagnostics %v", errs)
	}
	if (*Fence)(nil).WithWritableDirs("x") != nil {
		t.Fatal("a nil fence stays nil")
	}
}

func TestOutcomeDiagnosticsListsAtMostTheCapOfCreatedPaths(t *testing.T) {
	t.Parallel()
	created := make([]string, 0, 14)
	for i := 0; i < 14; i++ {
		created = append(created, "src/f"+string(rune('a'+i))+".go")
	}

	diags := (Outcome{Writer: true, Created: created}).Diagnostics("build")

	if len(diags) != 1 || diags[0].Severity != "error" || !strings.Contains(diags[0].Message, "+2 more") || strings.Contains(diags[0].Message, "src/fm.go") {
		t.Fatalf("diagnostics %+v", diags)
	}
}
