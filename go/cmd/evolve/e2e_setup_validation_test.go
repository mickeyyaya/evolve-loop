//go:build e2e

// Coverage of `evolve setup detect` — the kernel clamp behind the /setup skill
// that enforces the CLI×model integrity floor (ADR-0027). Step 9b removed the
// standalone `setup validate` subcommand (+ the llm_config.json it clamped); the
// same floor is now applied by policy.ValidatePin and surfaced inline by detect:
//   - pinned tier ∉ profile model_tier_envelope → phase `pin_violation` (+ dispatch hard-fail)
//   - pinned cli  ∉ profile allowed_clis         → phase `pin_violation` (+ dispatch hard-fail)
//
// Driven in-process via runSetup (this is package main), so these are fast and
// host-independent: the pin overlay reads only the fixture .evolve/policy.json +
// profiles. (detect additionally scans the host for CLI binaries, so the clis[]
// array is asserted on SHAPE only, not on host-specific values.)
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeSetupFixture lays out a temp evolve-dir with the given per-role profiles
// and an optional policy.json (empty → no file), and returns the evolve-dir path.
func writeSetupFixture(t *testing.T, profiles map[string]string, policyJSON string) string {
	t.Helper()
	evolveDir := t.TempDir()
	profDir := filepath.Join(evolveDir, "profiles")
	if err := os.MkdirAll(profDir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	for role, body := range profiles {
		if err := os.WriteFile(filepath.Join(profDir, role+".json"), []byte(body), 0o644); err != nil {
			t.Fatalf("write profile %s: %v", role, err)
		}
	}
	if policyJSON != "" {
		if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
			t.Fatalf("write policy.json: %v", err)
		}
	}
	return evolveDir
}

// detectReport is the subset of the digest these tests assert on.
type detectReport struct {
	CLIs   []map[string]any `json:"clis"`
	Phases []struct {
		Role         string `json:"role"`
		Source       string `json:"source"`
		CurrentCLI   string `json:"current_cli"`
		CurrentTier  string `json:"current_tier"`
		PinViolation string `json:"pin_violation"`
	} `json:"phases"`
	PolicyError string `json:"policy_error"`
}

// runDetectJSON invokes `evolve setup detect --json` in-process and parses it.
func runDetectJSON(t *testing.T, evolveDir string) detectReport {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runSetup([]string{"detect", "--json", "--evolve-dir=" + evolveDir, "--project-root=" + evolveDir}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("detect --json exit = %d, want 0\n%s", code, stderr.String())
	}
	var rep detectReport
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("detect --json not valid JSON: %v\n%s", err, stdout.String())
	}
	return rep
}

func phase(rep detectReport, role string) (string, string, string, string) {
	for _, p := range rep.Phases {
		if p.Role == role {
			return p.Source, p.CurrentCLI, p.CurrentTier, p.PinViolation
		}
	}
	return "", "", "", ""
}

// envelope violation: a pinned tier below the profile's min is surfaced as a
// pin_violation naming the envelope.
func TestSetupDetect_PinEnvelopeViolation(t *testing.T) {
	evolveDir := writeSetupFixture(t,
		map[string]string{
			"builder": `{"name":"builder","cli":"claude-tmux","allowed_clis":["all"],"model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"}}`,
		},
		`{"pins":{"builder":{"cli":"claude","model":"haiku"}}}`, // haiku (rank 1) < min balanced (rank 2)
	)
	_, _, _, viol := phase(runDetectJSON(t, evolveDir), "builder")
	if viol == "" || !bytes.Contains([]byte(viol), []byte("envelope")) {
		t.Errorf("expected envelope pin_violation, got %q", viol)
	}
}

// allowed_clis violation: a pinned cli outside the profile's allowed_clis is
// surfaced as a pin_violation.
func TestSetupDetect_PinAllowedCLIsViolation(t *testing.T) {
	evolveDir := writeSetupFixture(t,
		map[string]string{
			"builder": `{"name":"builder","cli":"claude-tmux","allowed_clis":["claude","agy"],"model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"}}`,
		},
		`{"pins":{"builder":{"cli":"codex","model":"sonnet"}}}`, // codex ∉ {claude,agy}
	)
	_, _, _, viol := phase(runDetectJSON(t, evolveDir), "builder")
	if viol == "" || !bytes.Contains([]byte(viol), []byte("allowed_clis")) {
		t.Errorf("expected allowed_clis pin_violation, got %q", viol)
	}
}

// a valid pin overlays the routing with source=policy-pin and no violation.
func TestSetupDetect_PinValidNoViolation(t *testing.T) {
	evolveDir := writeSetupFixture(t,
		map[string]string{
			"builder": `{"name":"builder","cli":"claude-tmux","allowed_clis":["claude","agy"],"model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"}}`,
		},
		`{"pins":{"builder":{"cli":"claude","model":"opus"}}}`, // claude ∈ allowed, opus ∈ balanced..deep
	)
	src, cli, tier, viol := phase(runDetectJSON(t, evolveDir), "builder")
	if src != "policy-pin" || cli != "claude" || tier != "opus" {
		t.Errorf("expected pinned claude/opus, got src=%q cli=%q tier=%q", src, cli, tier)
	}
	if viol != "" {
		t.Errorf("valid pin should have no violation, got %q", viol)
	}
}

// a malformed policy.json sets the top-level policy_error and disables overlay.
func TestSetupDetect_MalformedPolicy(t *testing.T) {
	evolveDir := writeSetupFixture(t,
		map[string]string{
			"builder": `{"name":"builder","cli":"claude-tmux","allowed_clis":["all"],"model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"}}`,
		},
		`{"pins": {not valid json`,
	)
	rep := runDetectJSON(t, evolveDir)
	if rep.PolicyError == "" {
		t.Error("malformed policy.json should set policy_error")
	}
	if src, _, _, _ := phase(rep, "builder"); src == "policy-pin" {
		t.Error("malformed policy should not overlay pins")
	}
}

// detect --json must emit a well-formed digest: a clis array and a phases
// array. Host CLI presence varies, so only the SHAPE is asserted.
func TestSetupDetect_JSONShape(t *testing.T) {
	evolveDir := writeSetupFixture(t,
		map[string]string{
			"builder": `{"name":"builder","cli":"claude-tmux","allowed_clis":["claude","agy"],"model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"}}`,
			"auditor": `{"name":"auditor","cli":"codex-tmux","allowed_clis":["all"],"model_tier_envelope":{"min":"deep","default":"deep","max":"deep"}}`,
		},
		"",
	)
	rep := runDetectJSON(t, evolveDir)
	if len(rep.CLIs) == 0 {
		t.Errorf("detect digest should enumerate known CLIs; got none")
	}
	if len(rep.Phases) == 0 {
		t.Errorf("detect digest should enumerate per-phase routing; got none")
	}
}
