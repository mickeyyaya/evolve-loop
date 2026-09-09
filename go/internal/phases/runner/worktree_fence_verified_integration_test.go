//go:build integration

package runner

import (
	"context"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"testing"
)

func TestRun_FenceVerificationCannotBeSuppliedByCaller(t *testing.T) {
	for _, tc := range []struct {
		name     string
		readOnly bool
		repo     bool
		verified bool
	}{
		{"restored", true, true, true},
		{"source writer", false, true, false},
		{"snapshot unavailable", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.repo {
				dir = fenceRepo(t)
			}
			observed := !tc.verified
			hooks := &fakeHooks{phase: "audit", agent: "evolve-audit", verdict: core.VerdictPASS}
			hooks.onClassify = func(req core.PhaseRequest) { observed = req.WorktreeVerified }
			r := New(Options{Hooks: hooks, Bridge: &mutatingBridge{}, Prompts: fakePromptsFS("evolve-audit", "body")})
			_, err := r.Run(context.Background(), core.PhaseRequest{
				ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: dir,
				WorktreeReadOnly: tc.readOnly, WorktreeVerified: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if observed != tc.verified {
				t.Fatalf("classification trusted caller's fence claim: got %v, want %v", observed, tc.verified)
			}
		})
	}
}
