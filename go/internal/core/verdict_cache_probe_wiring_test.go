//go:build integration

package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

func TestVerdictCacheProbeEligibilityWiring(t *testing.T) {
	tests := []struct {
		name  string
		dirty bool
	}{
		{name: "fresh worktree at base is ineligible", dirty: false},
		{name: "changed worktree stays eligible", dirty: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, _ := initVerdictCacheProbeRepo(t)
			wt := detachedWorktree(t, repo)
			if tt.dirty {
				if err := os.WriteFile(filepath.Join(wt, "f.txt"), []byte("changes"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			ctx := context.Background()
			candidate := worktreeContentSHA(ctx, repo, wt, t.TempDir())
			base := worktreeBaseTreeSHA(ctx, wt, "")
			if candidate == "" || base == "" {
				t.Fatalf("content identities unresolved: candidate=%q base=%q", candidate, base)
			}
			if tt.dirty == (candidate == base) {
				t.Fatalf("fixture invalid: dirty=%t but candidate==base is %t", tt.dirty, candidate == base)
			}

			// The oracle: the shared predicate decides probe eligibility.
			wantEligible := verdictcache.ProbeEligible(base, candidate)

			now := func() time.Time { return time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC) }
			if err := verdictcache.NewStore(repo, now).Put(verdictcache.Entry{
				TreeSHA: candidate, Cycle: 99, Verdict: VerdictPASS,
				ArtifactSHA256: "test-artifact", ArtifactPath: "test-artifact",
			}); err != nil {
				t.Fatalf("seed verdict cache: %v", err)
			}

			var got verdictCacheLookupObservation
			calls := 0
			o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 100}}, &fakeLedger{}, buildRunners(nil),
				WithWorktreeProvisioner(fixedWorktree{dir: wt}),
				WithVerdictCacheLookupHook(func(sha string, skipped bool, matched bool, _ verdictcache.Entry) {
					calls++
					got = verdictCacheLookupObservation{sha, skipped, matched}
				}),
			)
			o.now = now
			if _, err := o.RunCycle(ctx, CycleRequest{ProjectRoot: repo, GoalHash: "g"}); err != nil {
				t.Fatalf("RunCycle: %v", err)
			}
			if calls != 1 {
				t.Fatalf("cache probe calls = %d, want 1", calls)
			}
			if got.sha != candidate {
				t.Fatalf("probe sha = %q, want %q", got.sha, candidate)
			}
			if got.skipped == wantEligible {
				t.Fatalf("orchestrator skipped=%t but verdictcache.ProbeEligible(base=%s, candidate=%s)=%t — "+
					"the shadow probe does not derive eligibility from the shared predicate",
					got.skipped, base, candidate, wantEligible)
			}
			if got.matched != wantEligible {
				t.Fatalf("orchestrator matched=%t, want %t for the seeded entry", got.matched, wantEligible)
			}
		})
	}
}
