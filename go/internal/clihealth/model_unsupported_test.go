package clihealth

import (
	"strings"
	"testing"
	"time"
)

func TestModelUnsupported_BenchesForTheLongestCooldownWithAnOperatorAction(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	if Benchable(ModelUnsupportedPattern) || QuotaPattern(ModelUnsupportedPattern) {
		t.Fatal("model_unsupported never benches a whole family, and it is no quota pattern: only a model scope may hold it")
	}

	entry := NewBenchEntry(Entry{}, "codex", ModelUnsupportedPattern, "model gpt-5.6-sol is not supported", now)

	if !entry.BenchedUntil.Equal(now.Add(MaxCooldown)) {
		t.Errorf("BenchedUntil = %v, want now + MaxCooldown: the exit is deterministic for the pair", entry.BenchedUntil)
	}
	if !strings.Contains(entry.OperatorAction, "re-pin") || !strings.Contains(entry.OperatorAction, "accept the fallback") {
		t.Errorf("OperatorAction = %q, want the two operator choices", entry.OperatorAction)
	}
}
