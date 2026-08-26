package profiles

// deep_tier_family_arrangement_test.go — the 2026-08-26 operator directive:
// deep/top-tier task types run on codex (gpt-5.6-sol at xhigh), EXCEPT the two
// adversarial checks whose independence from the codex builder is the
// pipeline's anti-gaming core (cross-family floor: builder=codex ⇒ its graders
// are another family) and the advisor brain. Pins the WHOLE arrangement so a
// single-profile drift — either direction — is loud: a mover slipping back to
// claude silently sheds sol leverage; auditor/adversarial-review slipping to
// codex silently puts codex in judgment of codex.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDeepTierFamilyArrangement(t *testing.T) {
	dir := realProfilesDir(t)
	exceptions := map[string]string{
		"auditor":            "claude-tmux", // cross-family floor vs the codex builder
		"adversarial-review": "claude-tmux", // adversarial independence, same principle
		"router":             "agy-tmux",    // advisor brain — separate decision
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			continue
		}
		var p struct {
			CLI         string   `json:"cli"`
			Tier        string   `json:"model_tier_default"`
			CLIFallback []string `json:"cli_fallback"`
		}
		if json.Unmarshal(raw, &p) != nil {
			continue
		}
		if p.Tier != "deep" && p.Tier != "top" {
			continue
		}
		name := e.Name()[:len(e.Name())-len(".json")]
		checked++
		if want, ok := exceptions[name]; ok {
			if p.CLI != want {
				t.Errorf("%s: cli=%q, want %q — the adversarial/advisor exceptions are load-bearing", name, p.CLI, want)
			}
			continue
		}
		if p.CLI != "codex-tmux" {
			t.Errorf("%s: cli=%q, want codex-tmux (deep→gpt-5.6-sol arrangement, 2026-08-26)", name, p.CLI)
		}
		if len(p.CLIFallback) != 1 || p.CLIFallback[0] != "claude-tmux" {
			t.Errorf("%s: cli_fallback=%v, want [claude-tmux] (universal fallback; agy banned)", name, p.CLIFallback)
		}
	}
	if checked < 20 {
		t.Fatalf("only %d deep/top profiles checked — the arrangement guard lost its corpus", checked)
	}
}
