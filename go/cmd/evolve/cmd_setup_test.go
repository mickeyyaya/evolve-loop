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

func TestRunSetup_ValidateAndComplete(t *testing.T) {
	project := t.TempDir()
	evolveDir := filepath.Join(project, ".evolve")
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)

	setupWrite(t, filepath.Join(evolveDir, "profiles", "auditor.json"), `{
	  "cli":"codex-tmux","model_tier_default":"sonnet",
	  "model_tier_envelope":{"min":"deep","default":"deep","max":"deep"},
	  "cross_family_with":"builder","allowed_clis":["all"]
	}`)

	cfg := filepath.Join(evolveDir, "llm_config.json")

	// Within-envelope config → validate exit 0.
	setupWrite(t, cfg, `{"phases":{"auditor":{"cli":"codex","tier":"deep"}}}`)
	var out, errb bytes.Buffer
	if rc := runSetup([]string{"validate"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("clean validate: rc=%d want 0 (%s)", rc, out.String())
	}

	// Below-envelope tier → validate exit 2.
	setupWrite(t, cfg, `{"phases":{"auditor":{"cli":"codex","tier":"balanced"}}}`)
	out.Reset()
	if rc := runSetup([]string{"validate"}, nil, &out, &errb); rc != 2 {
		t.Fatalf("envelope violation: rc=%d want 2 (%s)", rc, out.String())
	}

	// Complete stamps the marker.
	out.Reset()
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
