package core

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
)

func TestHandoffReviewer_TheProductionTreeDigestBindsARealWorktree(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	tracked := filepath.Join(repo.Dir, "probe-tracked.txt")
	if err := os.WriteFile(tracked, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, _, err := gitCapture(ctx, repo.Dir, "add", "probe-tracked.txt"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := gitCapture(ctx, repo.Dir, "commit", "-m", "probe"); err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	const name = "handoff-tdd-build.json"
	writeWSFile(t, ws, name, `{"schema_version":1,"round":1}`)
	path := filepath.Join(ws, name)
	pass := func(ReviewInput, string) handoff.Check { return handoff.Check{Outcome: string(handoff.ExitPass)} }
	r := NewHandoffReviewer(handoffStages("enforce"), pass)
	in := ReviewInput{Cycle: 4, Phase: "build", Workspace: ws, Worktree: repo.Dir, ProjectRoot: t.TempDir(), Stderr: io.Discard}

	first := r.Review(ctx, in)
	d1 := ledgerField(t, path, "output_digest")
	if !first.Approve || d1 == "" {
		t.Fatalf("round 1 on a real worktree: approve=%v digest=%q, want approved with a digest", first.Approve, d1)
	}
	again := r.Review(ctx, in)
	if !again.Approve || ledgerField(t, path, "output_digest") != d1 {
		t.Fatalf("the same tree must stay fresh: %+v digest=%q", again, ledgerField(t, path, "output_digest"))
	}
	if err := os.WriteFile(tracked, []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := r.Review(ctx, in)
	if stale.Approve || !stale.Retry || !strings.Contains(stale.Remediation, d1) {
		t.Fatalf("a changed tree at the same round must be stale: %+v", stale)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeWSFile(t, ws, name, strings.Replace(string(raw), `"round": 1`, `"round": 2`, 1))
	raised := r.Review(ctx, in)
	d2 := ledgerField(t, path, "output_digest")
	if !raised.Approve || d2 == d1 || d2 == "" {
		t.Fatalf("a raised round must restamp: approve=%v d1=%q d2=%q", raised.Approve, d1, d2)
	}
}
