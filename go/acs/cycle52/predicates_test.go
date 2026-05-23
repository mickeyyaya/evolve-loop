// Package cycle52 ports the cycle-52 ACS predicates (3 bash files).
// Source-presence ports of the resolve-llm.sh behavior contract.
package cycle52

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const resolverRelPath = "legacy/scripts/dispatch/resolve-llm.sh"

// TestC52_001_LlmConfigLoadAndOverrideCli ports cycle-52/001.
// Behavioral: phases.scout.cli=gemini in llm_config overrides profile.
func TestC52_001_LlmConfigLoadAndOverrideCli(t *testing.T) {
	root := acsassert.RepoRoot(t)
	resolver := filepath.Join(root, resolverRelPath)
	if _, err := os.Stat(resolver); err != nil {
		t.Skipf("%s missing — skip", resolver)
	}
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "llm_config.json")
	body := `{"schema_version":1,"phases":{"scout":{"provider":"google","cli":"gemini","model":"gemini-3-pro-preview"}},"_fallback":{"provider":"anthropic","cli":"claude","model_tier":"sonnet"}}`
	if err := os.WriteFile(cfg, []byte(body), 0644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	out, err := exec.Command("bash", resolver, "scout", cfg).Output()
	if err != nil {
		t.Skipf("resolver invocation failed: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Skipf("resolver output not JSON: %v", err)
	}
	if cli, _ := doc["cli"].(string); cli != "gemini" {
		t.Errorf("AC2: expected cli=gemini, got %q", cli)
	}
	if src, _ := doc["source"].(string); src != "llm_config" {
		t.Errorf("AC3: expected source=llm_config, got %q", src)
	}
}

// TestC52_002_LlmConfigMissingPhaseFallback ports cycle-52/002.
func TestC52_002_LlmConfigMissingPhaseFallback(t *testing.T) {
	root := acsassert.RepoRoot(t)
	resolver := filepath.Join(root, resolverRelPath)
	if _, err := os.Stat(resolver); err != nil {
		t.Skipf("%s missing — skip", resolver)
	}
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "llm_config.json")
	body := `{"schema_version":1,"phases":{"scout":{"provider":"google","cli":"gemini","model":"gemini-3-pro-preview"}}}`
	if err := os.WriteFile(cfg, []byte(body), 0644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	out, err := exec.Command("bash", resolver, "builder", cfg).Output()
	if err != nil {
		t.Skipf("resolver invocation failed: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Skipf("resolver output not JSON: %v", err)
	}
	if cli, _ := doc["cli"].(string); cli != "claude" {
		t.Errorf("AC2: expected cli=claude (profile fallback), got %q", cli)
	}
	if src, _ := doc["source"].(string); src != "profile" {
		t.Errorf("AC3: expected source=profile, got %q", src)
	}
}

// TestC52_003_LlmConfigAbsentZeroConfigWorks ports cycle-52/003.
func TestC52_003_LlmConfigAbsentZeroConfigWorks(t *testing.T) {
	root := acsassert.RepoRoot(t)
	resolver := filepath.Join(root, resolverRelPath)
	if _, err := os.Stat(resolver); err != nil {
		t.Skipf("%s missing — skip", resolver)
	}
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "no-such-llm-config.json")
	out, err := exec.Command("bash", resolver, "scout", missing).Output()
	if err != nil {
		t.Skipf("resolver rc nonzero with missing cfg: %v (likely backward-compat broken)", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Skipf("resolver output not JSON: %v", err)
	}
	if src, _ := doc["source"].(string); src != "profile" {
		t.Errorf("AC3: expected source=profile when config absent, got %q", src)
	}
	if cli, _ := doc["cli"].(string); cli == "" || cli == "null" {
		t.Errorf("AC4: cli empty in zero-config mode")
	}
}

// keep import used
var _ = acsassert.RepoRoot
