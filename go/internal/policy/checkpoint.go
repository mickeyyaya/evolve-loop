package policy

import "fmt"

const defaultCheckpointKeepPerWorktree = 20

type CheckpointPolicy struct {
	KeepPerWorktree int `json:"keep_per_worktree,omitempty"`
}

func (c *CheckpointPolicy) UnmarshalJSON(raw []byte) error {
	var decoded struct {
		KeepPerWorktree *int `json:"keep_per_worktree"`
	}
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	if decoded.KeepPerWorktree == nil {
		*c = CheckpointPolicy{}
		return nil
	}
	if *decoded.KeepPerWorktree <= 0 {
		return fmt.Errorf("checkpoint: keep_per_worktree must be positive, got %d; remove the key for the default of %d", *decoded.KeepPerWorktree, defaultCheckpointKeepPerWorktree)
	}
	*c = CheckpointPolicy{KeepPerWorktree: *decoded.KeepPerWorktree}
	return nil
}

func (p Policy) CheckpointConfig() CheckpointPolicy {
	c := CheckpointPolicy{KeepPerWorktree: defaultCheckpointKeepPerWorktree}
	if p.Checkpoint != nil && p.Checkpoint.KeepPerWorktree > 0 {
		c.KeepPerWorktree = p.Checkpoint.KeepPerWorktree
	}
	return c
}
