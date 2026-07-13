package bridge

// tokendriver_test.go — cycle-779 AC2 plumbing contract (named by ACS
// predicate C779_005): recordTokenUsage must forward the launch's CLI/driver
// identity into the tokenusage.Window it hands the resolver. Without it the
// resolver cannot dispatch per driver and uncovered drivers (agy/codex)
// surface as silent zeros — the 2026-07-13 all-zeros baseline defect.

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// TestRecordTokenUsage_PassesDriverToResolver: the Window the resolver
// receives carries req.CLI as its Driver, alongside the existing
// worktree/artifact/window context.
func TestRecordTokenUsage_PassesDriverToResolver(t *testing.T) {
	start := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	var got tokenusage.Window
	e := NewEngine(Deps{
		Now: func() time.Time { return start.Add(90 * time.Second) },
		TokenResolver: func(w tokenusage.Window) (tokenusage.Result, error) {
			got = w
			return tokenusage.Result{Source: tokenusage.SourceNone}, nil
		},
	})
	req := core.BridgeRequest{
		CLI:       "codex",
		Agent:     "build",
		Workspace: t.TempDir(),
		Worktree:  "/repo/worktrees/cycle-779",
	}
	var resp core.BridgeResponse
	e.recordTokenUsage(req, "gpt-5", 0, start, &resp)

	if got.Driver != "codex" {
		t.Errorf("Window.Driver = %q, want %q (recordTokenUsage drops req.CLI — per-driver dispatch impossible)", got.Driver, req.CLI)
	}
	if got.Worktree != req.Worktree {
		t.Errorf("Window.Worktree = %q, want %q (driver plumbing must not displace existing context)", got.Worktree, req.Worktree)
	}
}
