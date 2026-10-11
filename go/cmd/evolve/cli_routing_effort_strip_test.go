package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestMoveProfileEfforts_AKeyStripFailureNamesTheProfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "scout.json"), []byte(`{"name":"scout","effort_level":"high"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := stripRetiredKeys
	t.Cleanup(func() { stripRetiredKeys = prev })
	stripRetiredKeys = func([]byte, ...string) ([]byte, error) { return nil, errors.New("want a JSON object") }

	_, err := moveProfileEfforts(dir, policy.CLIRouting{}, map[string][]byte{})

	if err == nil || err.Error() != "scout.json: want a JSON object" {
		t.Fatalf("err = %v, want scout.json: want a JSON object", err)
	}
}

func TestMoveProfileEfforts_StripsTheRetiredKeysAndKeepsTheRest(t *testing.T) {
	dir := t.TempDir()
	body := `{"name":"scout","effort_level":"high","model_tier_default":"deep","extra":1}`
	if err := os.WriteFile(filepath.Join(dir, "scout.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rewrites := map[string][]byte{}

	block, err := moveProfileEfforts(dir, policy.CLIRouting{}, rewrites)

	if err != nil {
		t.Fatalf("moveProfileEfforts: %v", err)
	}
	if got := string(rewrites["scout.json"]); got != `{"name":"scout","model_tier_default":"deep","extra":1}` {
		t.Errorf("rewrite = %s, want the effort key removed and the rest kept byte for byte", got)
	}
	if block.Agents["scout"].Effort != "high" {
		t.Errorf("agents.scout.effort = %q, want high moved into the table", block.Agents["scout"].Effort)
	}
}
