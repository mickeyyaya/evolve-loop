package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunSetup_Dispatch(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := runSetup(nil, nil, &out, &errb); rc != 10 {
		t.Errorf("no subcommand: rc=%d want 10", rc)
	}
	errb.Reset()
	if rc := runSetup([]string{"bogus"}, nil, &out, &errb); rc != 10 {
		t.Errorf("unknown subcommand: rc=%d want 10", rc)
	}
	if !strings.Contains(errb.String(), "unknown subcommand") {
		t.Errorf("missing diagnostic: %q", errb.String())
	}
}

// detectPhase is the subset of a detect-report phase entry the setup tests assert on.
type detectPhase struct {
	Role         string `json:"role"`
	Source       string `json:"source"`
	CurrentCLI   string `json:"current_cli"`
	CurrentTier  string `json:"current_tier"`
	PinViolation string `json:"pin_violation"`
}

// phaseFromDetectJSON runs `evolve setup detect --json` and returns the named
// phase entry (the durable per-phase view the /setup loop inspects).
func phaseFromDetectJSON(t *testing.T, role string) detectPhase {
	t.Helper()
	var out, errb bytes.Buffer
	if rc := runSetup([]string{"detect", "--json"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("detect --json: rc=%d (%s)", rc, errb.String())
	}
	var rep struct {
		Phases []detectPhase `json:"phases"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parse detect json: %v\n%s", err, out.String())
	}
	for _, p := range rep.Phases {
		if p.Role == role {
			return p
		}
	}
	t.Fatalf("%s phase not found in detect report", role)
	return detectPhase{}
}

func TestRunSetup_DetectPinsAndComplete(t *testing.T) {
	project := t.TempDir()
	evolveDir := filepath.Join(project, ".evolve")
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)

	setupWrite(t, filepath.Join(evolveDir, "profiles", "auditor.json"), `{
	  "cli":"codex-tmux","model_tier_default":"sonnet",
	  "model_tier_envelope":{"min":"deep","default":"deep","max":"deep"},
	  "cross_family_with":"builder","allowed_clis":["all"]
	}`)
	policyPath := filepath.Join(evolveDir, "policy.json")

	// Within-envelope pin → detect overlays it as policy-pin, no violation.
	setupWrite(t, policyPath, `{"pins":{"auditor":{"cli":"codex","model":"opus"}}}`)
	a := phaseFromDetectJSON(t, "auditor")
	if a.Source != "policy-pin" || a.CurrentCLI != "codex" || a.CurrentTier != "opus" {
		t.Fatalf("clean pin: got %+v", a)
	}
	if a.PinViolation != "" {
		t.Fatalf("clean pin should have no violation, got %q", a.PinViolation)
	}

	// Below-envelope pin → detect surfaces a pin_violation (deep..deep rejects balanced).
	setupWrite(t, policyPath, `{"pins":{"auditor":{"cli":"codex","model":"balanced"}}}`)
	a = phaseFromDetectJSON(t, "auditor")
	if a.PinViolation == "" {
		t.Fatalf("below-envelope pin should report a violation, got %+v", a)
	}

	// Complete stamps the marker.
	var out, errb bytes.Buffer
	if rc := runSetup([]string{"complete"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("complete: rc=%d want 0 (%s)", rc, errb.String())
	}
	raw, _ := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	var st map[string]json.RawMessage
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if _, ok := st["setupCompletedAt"]; !ok {
		t.Error("complete did not stamp setupCompletedAt")
	}
}

// TestRunSetup_RecommendJSON: `setup recommend --json` exits 0 and emits the
// configured presets (3 from the shipped default) regardless of host CLIs —
// presets come from the public config, not from detection.
func TestRunSetup_RecommendJSON(t *testing.T) {
	project := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	var out, errb bytes.Buffer
	if rc := runSetup([]string{"recommend", "--json"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("recommend: rc=%d (%s)", rc, errb.String())
	}
	var rr struct {
		Presets []struct {
			Name string `json:"name"`
		} `json:"presets"`
		Default string `json:"default"`
	}
	if err := json.Unmarshal(out.Bytes(), &rr); err != nil {
		t.Fatalf("recommend emitted non-JSON: %v\n%s", err, out.String())
	}
	if len(rr.Presets) != 3 || rr.Default != "recommended" {
		t.Errorf("want 3 presets + default recommended, got %d / %q", len(rr.Presets), rr.Default)
	}
}

// TestRunSetup_RecommendHuman: human mode exits 0 and prints something.
func TestRunSetup_RecommendHuman(t *testing.T) {
	project := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	var out, errb bytes.Buffer
	if rc := runSetup([]string{"recommend"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("recommend human: rc=%d (%s)", rc, errb.String())
	}
	if out.Len() == 0 {
		t.Error("recommend human mode should print a table")
	}
}

// TestRunSetup_ApplyMissingPreset: --preset is required → bad-args exit 10.
func TestRunSetup_ApplyMissingPreset(t *testing.T) {
	project := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	var out, errb bytes.Buffer
	if rc := runSetup([]string{"apply"}, nil, &out, &errb); rc != 10 {
		t.Errorf("missing --preset: rc=%d want 10 (%s)", rc, errb.String())
	}
}

// TestRunSetup_ApplyUnknownPreset: an unknown preset is a runtime refusal (exit
// 1) naming the valid set — host-independent (rejected before any pin write).
func TestRunSetup_ApplyUnknownPreset(t *testing.T) {
	project := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	var out, errb bytes.Buffer
	if rc := runSetup([]string{"apply", "--preset", "turbo"}, nil, &out, &errb); rc != 1 {
		t.Errorf("unknown preset: rc=%d want 1 (%s)", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "recommended") {
		t.Errorf("unknown-preset diagnostic should name valid presets, got %q", errb.String())
	}
}

// TestRunSetup_ApplyWritesPolicy is host-robust: foreign keys survive whether
// apply writes (host has an authed CLI → rc 0, pins added) or refuses a degraded
// preset (no authed CLI → rc 1, policy.json untouched, never clobbered).
func TestRunSetup_ApplyWritesPolicy(t *testing.T) {
	project := t.TempDir()
	evolveDir := filepath.Join(project, ".evolve")
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	setupWrite(t, filepath.Join(evolveDir, "profiles", "builder.json"), `{
	  "cli":"agy-tmux","model_tier_default":"sonnet",
	  "model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"},
	  "cross_family_with":"auditor","allowed_clis":["claude","agy"]
	}`)
	setupWrite(t, filepath.Join(evolveDir, "profiles", "auditor.json"), `{
	  "cli":"codex-tmux","model_tier_default":"sonnet",
	  "model_tier_envelope":{"min":"deep","default":"deep","max":"deep"},
	  "cross_family_with":"builder","allowed_clis":["all"]
	}`)
	policyPath := filepath.Join(evolveDir, "policy.json")
	setupWrite(t, policyPath, `{"version":1,"floor":[{"id":"dossier-closeout"}]}`)

	var out, errb bytes.Buffer
	rc := runSetup([]string{"apply", "--preset", "recommended"}, nil, &out, &errb)

	raw, _ := os.ReadFile(policyPath)
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("policy.json unreadable after apply: %v", err)
	}
	// Foreign keys must survive in BOTH outcomes (write preserves; refuse leaves untouched).
	for _, k := range []string{"version", "floor"} {
		if _, ok := obj[k]; !ok {
			t.Errorf("apply (rc=%d) dropped foreign key %q", rc, k)
		}
	}
	switch rc {
	case 0:
		if _, ok := obj["pins"]; !ok {
			t.Error("apply rc=0 should have added pins")
		}
	case 1:
		// Degraded refusal (no authed CLI on this host): policy.json untouched.
		if _, ok := obj["pins"]; ok {
			t.Error("refused apply must NOT write pins")
		}
	default:
		t.Fatalf("unexpected rc=%d (%s)", rc, errb.String())
	}
}

// TestRunSetup_ApplyUnreadablePolicy_FailsLoud: a present-but-unreadable
// policy.json (here a directory) must fail loudly (exit 1), never be silently
// treated as absent and overwritten.
func TestRunSetup_ApplyUnreadablePolicy_FailsLoud(t *testing.T) {
	project := t.TempDir()
	evolveDir := filepath.Join(project, ".evolve")
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	// policy.json as a directory → os.ReadFile returns a non-ENOENT error.
	if err := os.MkdirAll(filepath.Join(evolveDir, "policy.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	rc := runSetup([]string{"apply", "--preset", "recommended"}, nil, &out, &errb)
	if rc != 1 {
		t.Fatalf("unreadable policy.json should fail loud: rc=%d want 1 (%s)", rc, errb.String())
	}
	// The failure must come from the READ step (fired before Apply), not from a
	// later write — proving the non-ENOENT read error is surfaced, not swallowed.
	if !strings.Contains(errb.String(), "reading") {
		t.Errorf("error should name the policy read failure, got %q", errb.String())
	}
}

func TestMaybePrintSetupNudge(t *testing.T) {
	// No state.json → nudge prints.
	evolveDir := t.TempDir()
	var w bytes.Buffer
	maybePrintSetupNudge(&w, evolveDir)
	if !strings.Contains(w.String(), "First run") {
		t.Errorf("fresh repo should nudge, got %q", w.String())
	}

	// Marker present → silent.
	setupWrite(t, filepath.Join(evolveDir, "state.json"), `{"setupCompletedAt":"2026-01-01T00:00:00Z","setupVersion":1}`)
	w.Reset()
	maybePrintSetupNudge(&w, evolveDir)
	if w.Len() != 0 {
		t.Errorf("onboarded repo should be silent, got %q", w.String())
	}

	// Empty marker → nudge.
	setupWrite(t, filepath.Join(evolveDir, "state.json"), `{"lastCycleNumber":3}`)
	w.Reset()
	maybePrintSetupNudge(&w, evolveDir)
	if !strings.Contains(w.String(), "First run") {
		t.Errorf("empty marker should nudge, got %q", w.String())
	}
}

func TestRunSetup_ProjectRootFlagWinsOverEnv(t *testing.T) {
	// --project-root must override EVOLVE_PROJECT_ROOT so the dispatcher can
	// point setup at the SAME root the loop uses (marker lands where loop reads).
	dirEnv := t.TempDir()
	dirFlag := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", dirEnv)

	var out, errb bytes.Buffer
	if rc := runSetup([]string{"complete", "--project-root", dirFlag}, nil, &out, &errb); rc != 0 {
		t.Fatalf("complete: rc=%d (%s)", rc, errb.String())
	}
	if _, err := os.Stat(filepath.Join(dirFlag, ".evolve", "state.json")); err != nil {
		t.Errorf("marker should land under --project-root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dirEnv, ".evolve", "state.json")); err == nil {
		t.Error("marker must NOT land under EVOLVE_PROJECT_ROOT when --project-root is given")
	}
}

func TestRunSetup_DetectJSON(t *testing.T) {
	// detect runs the real (host) doctor probe; assert it exits 0 and emits
	// parseable JSON regardless of which CLIs the host has.
	project := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	var out, errb bytes.Buffer
	if rc := runSetup([]string{"detect", "--json"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("detect: rc=%d (%s)", rc, errb.String())
	}
	var rep map[string]any
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("detect emitted non-JSON: %v\n%s", err, out.String())
	}
	if _, ok := rep["phases"]; !ok {
		t.Errorf("detect JSON missing phases: %v", rep)
	}
}

// TestRunSetup_DetectSpaceSeparatedStringFlagThenBool regresses the
// reorderArgs flag-swallow bug: a space-separated STRING flag followed by
// another flag (`--evolve-dir X --json`) must NOT consume the trailing --json
// as its value. Before the fix this emitted the human table (--json ignored).
func TestRunSetup_DetectSpaceSeparatedStringFlagThenBool(t *testing.T) {
	project := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	ev := t.TempDir()
	var out, errb bytes.Buffer
	if rc := runSetup([]string{"detect", "--evolve-dir", ev, "--json"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("detect: rc=%d (%s)", rc, errb.String())
	}
	var rep map[string]any
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("--json after a space-separated --evolve-dir must still emit JSON; got:\n%s", out.String())
	}
}
