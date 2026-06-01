//go:build e2e

// Coverage of `evolve setup detect|validate` — the kernel clamp behind the
// /setup skill that enforces the CLI×model integrity floor (ADR-0027):
//   - tier ∈ profile model_tier_envelope  → ERROR (exit 2)
//   - cli  ∈ profile allowed_clis          → ERROR (exit 2)
//   - builder family == auditor family     → WARN (exit 0); ERROR under --strict
//
// Driven in-process via runSetup (this is package main), so these are fast and
// host-independent: validate reads only the fixture llm_config.json + profiles
// written into a temp evolve-dir. (detect additionally scans the host for CLI
// binaries, so it is asserted on SHAPE only, not on host-specific values.)
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeSetupFixture lays out a temp evolve-dir with the given per-role profiles
// and an llm_config.json, and returns the evolve-dir path.
func writeSetupFixture(t *testing.T, profiles map[string]string, llmConfig string) string {
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
	if err := os.WriteFile(filepath.Join(evolveDir, "llm_config.json"), []byte(llmConfig), 0o644); err != nil {
		t.Fatalf("write llm_config: %v", err)
	}
	return evolveDir
}

// runValidate invokes `evolve setup validate` in-process and returns the exit
// code + combined stdout.
func runValidate(t *testing.T, evolveDir string, strict bool) (int, string) {
	t.Helper()
	args := []string{"validate", "--evolve-dir=" + evolveDir, "--project-root=" + evolveDir}
	if strict {
		args = append(args, "--strict")
	}
	var stdout, stderr bytes.Buffer
	code := runSetup(args, nil, &stdout, &stderr)
	return code, stdout.String() + stderr.String()
}

// envelope violation: builder tier below the profile's min → ERROR (exit 2).
func TestSetupValidate_EnvelopeViolation(t *testing.T) {
	evolveDir := writeSetupFixture(t,
		map[string]string{
			"builder": `{"name":"builder","allowed_clis":["all"],"model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"}}`,
		},
		`{"phases":{"builder":{"cli":"claude","tier":"fast"}}}`, // fast (rank 1) < min balanced (rank 2)
	)
	code, out := runValidate(t, evolveDir, false)
	if code != 2 {
		t.Fatalf("envelope violation exit = %d, want 2\n%s", code, out)
	}
	if !bytes.Contains([]byte(out), []byte("envelope")) {
		t.Errorf("output should name the envelope violation; got %q", out)
	}
}

// allowed_clis violation: builder cli outside the profile's allowed_clis → ERROR.
func TestSetupValidate_AllowedCLIsViolation(t *testing.T) {
	evolveDir := writeSetupFixture(t,
		map[string]string{
			"builder": `{"name":"builder","allowed_clis":["claude","agy"],"model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"}}`,
		},
		`{"phases":{"builder":{"cli":"codex","tier":"deep"}}}`, // codex ∉ {claude,agy}
	)
	code, out := runValidate(t, evolveDir, false)
	if code != 2 {
		t.Fatalf("allowed_clis violation exit = %d, want 2\n%s", code, out)
	}
}

// Cross-family: builder and auditor on the same family (both claude/anthropic)
// is a WARN by default (exit 0) and an ERROR under --strict (exit 2).
func TestSetupValidate_CrossFamily_WarnVsStrict(t *testing.T) {
	profiles := map[string]string{
		"builder": `{"name":"builder","allowed_clis":["all"],"model_tier_envelope":{"min":"balanced","default":"deep","max":"deep"}}`,
		"auditor": `{"name":"auditor","allowed_clis":["all"],"model_tier_envelope":{"min":"deep","default":"deep","max":"deep"}}`,
	}
	// Both claude → same family (anthropic); envelopes satisfied so ONLY the
	// cross-family rule can fire.
	cfg := `{"phases":{"builder":{"cli":"claude","tier":"deep"},"auditor":{"cli":"claude","tier":"deep"}}}`
	evolveDir := writeSetupFixture(t, profiles, cfg)

	if code, out := runValidate(t, evolveDir, false); code != 0 {
		t.Fatalf("cross-family default exit = %d, want 0 (WARN, not blocking)\n%s", code, out)
	}
	if code, out := runValidate(t, evolveDir, true); code != 2 {
		t.Fatalf("cross-family --strict exit = %d, want 2 (ERROR)\n%s", code, out)
	}
}

// Cross-family clean: different families (claude builder, codex auditor) → no
// cross-family violation, exit 0 even under --strict.
func TestSetupValidate_CrossFamily_DifferentFamiliesPass(t *testing.T) {
	profiles := map[string]string{
		"builder": `{"name":"builder","allowed_clis":["all"],"model_tier_envelope":{"min":"balanced","default":"deep","max":"deep"}}`,
		"auditor": `{"name":"auditor","allowed_clis":["all"],"model_tier_envelope":{"min":"deep","default":"deep","max":"deep"}}`,
	}
	cfg := `{"phases":{"builder":{"cli":"claude","tier":"deep"},"auditor":{"cli":"codex","tier":"deep"}}}`
	evolveDir := writeSetupFixture(t, profiles, cfg)

	if code, out := runValidate(t, evolveDir, true); code != 0 {
		t.Fatalf("different-family --strict exit = %d, want 0\n%s", code, out)
	}
}

// detect --json must emit a well-formed digest: a clis array and a phases
// array. Host CLI presence varies, so only the SHAPE is asserted.
func TestSetupDetect_JSONShape(t *testing.T) {
	evolveDir := writeSetupFixture(t,
		map[string]string{
			"builder": `{"name":"builder","allowed_clis":["claude","agy"],"model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"}}`,
			"auditor": `{"name":"auditor","allowed_clis":["all"],"model_tier_envelope":{"min":"deep","default":"deep","max":"deep"}}`,
		},
		`{"phases":{}}`,
	)
	var stdout, stderr bytes.Buffer
	code := runSetup([]string{"detect", "--json", "--evolve-dir=" + evolveDir, "--project-root=" + evolveDir}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("detect --json exit = %d, want 0\n%s", code, stderr.String())
	}
	var rep struct {
		CLIs   []map[string]any `json:"clis"`
		Phases []map[string]any `json:"phases"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("detect --json not valid JSON: %v\n%s", err, stdout.String())
	}
	if len(rep.CLIs) == 0 {
		t.Errorf("detect digest should enumerate known CLIs; got none\n%s", stdout.String())
	}
	if len(rep.Phases) == 0 {
		t.Errorf("detect digest should enumerate per-phase routing; got none\n%s", stdout.String())
	}
}
