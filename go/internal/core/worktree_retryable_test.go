package core

// These observe the retry announcement through the package's existing
// captureStderr helper (phase_bindings_selfcheck_failloud_test.go).
import (
	"strings"
	"testing"
)

func TestWorktreeAddRetry_NotAGitRepositoryCostsZeroBackoff(t *testing.T) {
	r := worktreeAddRetry("cycle-lane-1")
	if r.Retryable == nil {
		t.Fatal("core supplies no transience classifier — the shared loop then retries EVERYTHING, which is the 198s cmd/evolve tax unchanged (a classifier nothing passes is dead code)")
	}
	if r.Retryable(128, "fatal: not a git repository (or any of the parent directories): .git") {
		t.Error("core classified a permanent `not a git repository` as retryable — every cmd/evolve test that reaches provisioning over a t.TempDir() then pays 2s+4s for nothing")
	}
}

func TestWorktreeAddRetry_LockCollisionStillRetriesToBound(t *testing.T) {
	r := worktreeAddRetry("cycle-lane-1")
	if !r.Retryable(255, "Preparing worktree (new branch 'cycle-lane-1')\n") {
		t.Error("a lock-shaped rc=255 collision was classified as permanent — PR #401's absorber is disarmed and one collision again costs a lane its cycle")
	}
	if !r.Retryable(255, "") {
		t.Error("an unrecognised failure must stay retryable: a misclassified transient costs a cycle, a misclassified permanent costs 6s")
	}
}

func TestWorktreeAddRetry_AnnouncementDoesNotClaimUnclassifiedTransience(t *testing.T) {
	r := worktreeAddRetry("cycle-lane-1")
	if r.OnRetry == nil {
		t.Fatal("no retry announcement — contention must stay visible to the operator")
	}
	out := captureStderr(t, func() { r.OnRetry(1, 3, 255, "") })

	if strings.Contains(strings.ToLower(out), "transient") {
		t.Errorf("announcement claims transience it never established:\n\t%s\nOnRetry fires for every failure the classifier did not RULE OUT, which is not the same as one proven to be contention — that is how a permanent rc=128 came to be logged as contention 33 times per cmd/evolve run", strings.TrimSpace(out))
	}
	for _, want := range []string{"[worktree] retry 1/2", "cycle-lane-1", "rc=255"} {
		if !strings.Contains(out, want) {
			t.Errorf("announcement lost %q — honesty must not cost diagnosability:\n\t%s", want, strings.TrimSpace(out))
		}
	}
}
