package policy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCheckpointConfig_KeepsTwentyPerWorktreeUnlessAPositiveNumberIsSet(t *testing.T) {
	for name, tc := range map[string]struct {
		p    policy.Policy
		want int
	}{
		"an absent block keeps 20":  {policy.Policy{}, 20},
		"an empty block keeps 20":   {policy.Policy{Checkpoint: &policy.CheckpointPolicy{}}, 20},
		"zero cannot delete them":   {policy.Policy{Checkpoint: &policy.CheckpointPolicy{KeepPerWorktree: 0}}, 20},
		"a negative cannot either":  {policy.Policy{Checkpoint: &policy.CheckpointPolicy{KeepPerWorktree: -3}}, 20},
		"an explicit positive wins": {policy.Policy{Checkpoint: &policy.CheckpointPolicy{KeepPerWorktree: 5}}, 5},
	} {
		if got := tc.p.CheckpointConfig().KeepPerWorktree; got != tc.want {
			t.Errorf("%s: keep_per_worktree = %d, want %d", name, got, tc.want)
		}
	}
}

func TestCheckpointConfig_TheRetentionReachesTheVerbFromPolicyJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"checkpoint":{"keep_per_worktree":7}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := policy.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.CheckpointConfig().KeepPerWorktree; got != 7 {
		t.Errorf("keep_per_worktree = %d, want 7", got)
	}
}

func TestCheckpointPolicy_UnmarshalJSONRefusesAnUnknownKey(t *testing.T) {
	var c policy.CheckpointPolicy
	err := c.UnmarshalJSON([]byte(`{"keep_per_worktee":3}`))
	if err == nil || !strings.Contains(err.Error(), "checkpoint") || !strings.Contains(err.Error(), "keep_per_worktee") {
		t.Fatalf("UnmarshalJSON(typo) err = %v, want a checkpoint error naming the unknown key", err)
	}
	if err := c.UnmarshalJSON([]byte(`{"keep_per_worktree":3}`)); err != nil || c.KeepPerWorktree != 3 {
		t.Fatalf("UnmarshalJSON(valid) = %v, keep %d; want nil, 3", err, c.KeepPerWorktree)
	}
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"checkpoint":{"keep":3}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.Load(path); err == nil {
		t.Error("Load accepted a checkpoint block with an unknown key; the block decodes strictly")
	}
}

func TestCheckpointPolicy_UnmarshalJSONRefusesANonPositiveRetention(t *testing.T) {
	for _, text := range []string{`{"keep_per_worktree":0}`, `{"keep_per_worktree":-2}`} {
		var c policy.CheckpointPolicy
		if err := c.UnmarshalJSON([]byte(text)); err == nil || !strings.Contains(err.Error(), "keep_per_worktree") {
			t.Errorf("UnmarshalJSON(%s) err = %v, want a refusal naming keep_per_worktree", text, err)
		}
	}
	var empty policy.CheckpointPolicy
	if err := empty.UnmarshalJSON([]byte(`{}`)); err != nil || empty.KeepPerWorktree != 0 {
		t.Errorf("UnmarshalJSON({}) = %v, keep %d; want an empty block accepted", err, empty.KeepPerWorktree)
	}
}
