package config

import "testing"

func TestCommitEvidence_Default(t *testing.T) {
	cfg, _ := Load("")
	if cfg.CommitEvidence != StageOff {
		t.Fatalf("default CommitEvidence = %v, want StageOff", cfg.CommitEvidence)
	}
}
