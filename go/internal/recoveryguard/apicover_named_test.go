package recoveryguard

import (
	"context"
	"testing"
)

// TestAPICoverNamedExports names every exported symbol through the shape the correction ladder uses: a Scope
// for the dispatch, Begin before the recovery agent runs, End after it returns, Clean deciding whether the
// rung stands.
func TestAPICoverNamedExports(t *testing.T) {
	ctx := context.Background()
	scope := Scope{Worktree: worktree(t), Workspace: workspace(t)}
	var g *Guard
	g, err := Begin(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	var out Outcome = g.End(ctx)
	if !out.Clean() {
		t.Fatalf("an untouched dispatch must be clean, got %+v", out)
	}
}
