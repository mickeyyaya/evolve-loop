package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeUserPhase(t *testing.T, root, name, phaseJSON string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "phases", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phase.json"), []byte(phaseJSON), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPhaseLint_CleanPhaseExitsZeroNoWarnings(t *testing.T) {
	root := t.TempDir()
	writeUserPhase(t, root, "widget-check", `{
  "name": "widget-check", "kind": "llm", "optional": true, "archetype": "evaluate",
  "outputs": { "files": [".evolve/runs/cycle-{cycle}/widget-check-report.md"] },
  "classify": { "require_sections": ["Findings"], "verdict_on_pass": "PASS" },
  "routing": { "insert_when": [ { "field": "build.files_touched", "op": "gt", "value": 0 } ] }
}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	if code := runPhaseLint([]string{"widget-check"}, &out, &errb); code != 0 {
		t.Fatalf("clean phase should exit 0; got %d (stderr=%q)", code, errb.String())
	}
}

func TestPhaseLint_AlwaysFailOpenExitZero(t *testing.T) {
	root := t.TempDir()
	// Evaluate archetype with NO require_sections → a warning, but still exit 0.
	writeUserPhase(t, root, "weak-eval", `{
  "name": "weak-eval", "kind": "llm", "optional": true, "archetype": "evaluate",
  "outputs": { "files": ["weak-eval-report.md"] }
}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	code := runPhaseLint([]string{"weak-eval"}, &out, &errb)
	if code != 0 {
		t.Fatalf("lint must be fail-open (exit 0); got %d", code)
	}
	combined := out.String() + errb.String()
	if !strings.Contains(combined, "require_sections") {
		t.Errorf("expected a warning naming require_sections for an evaluate phase; got %q", combined)
	}
}

func TestPhaseLint_InvalidSpecWarnsButExitsZero(t *testing.T) {
	root := t.TempDir()
	// Non-optional user phase is a hard ValidateUserSpec violation — surfaced as a
	// warning, but lint never blocks.
	writeUserPhase(t, root, "bad-floor", `{
  "name": "bad-floor", "kind": "llm", "optional": false,
  "outputs": { "files": ["bad-floor-report.md"] }
}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	if code := runPhaseLint([]string{"bad-floor"}, &out, &errb); code != 0 {
		t.Fatalf("lint must be fail-open; got %d", code)
	}
	if !strings.Contains(out.String()+errb.String(), "optional") {
		t.Errorf("expected the optional-floor violation surfaced; got %q / %q", out.String(), errb.String())
	}
}

func TestPhaseLint_UnknownCategoryWarnsButExitsZero(t *testing.T) {
	root := t.TempDir()
	writeUserPhase(t, root, "cat-check", `{
  "name": "cat-check", "kind": "llm", "optional": true, "archetype": "evaluate",
  "categories": ["bugfix", "fooly"],
  "outputs": { "files": ["cat-check-report.md"] },
  "classify": { "require_sections": ["Findings"], "verdict_on_pass": "PASS" }
}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	if code := runPhaseLint([]string{"cat-check"}, &out, &errb); code != 0 {
		t.Fatalf("lint must be fail-open; got %d", code)
	}
	combined := out.String() + errb.String()
	if !strings.Contains(combined, "fooly") {
		t.Errorf("expected a warning naming the unknown category; got %q", combined)
	}
	if strings.Contains(combined, "bugfix") && strings.Contains(combined, `"bugfix"`) {
		t.Errorf("known category bugfix must not be flagged; got %q", combined)
	}
}

func TestPhaseLint_KnownCategoriesNoWarning(t *testing.T) {
	root := t.TempDir()
	writeUserPhase(t, root, "good-cats", `{
  "name": "good-cats", "kind": "llm", "optional": true, "archetype": "evaluate",
  "categories": ["security", "performance"],
  "outputs": { "files": ["good-cats-report.md"] },
  "classify": { "require_sections": ["Findings"], "verdict_on_pass": "PASS" }
}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	if code := runPhaseLint([]string{"good-cats"}, &out, &errb); code != 0 {
		t.Fatalf("lint must be fail-open; got %d", code)
	}
	if !strings.Contains(out.String(), "OK") {
		t.Errorf("valid categories should lint clean (OK line); got %q", out.String())
	}
}

func TestPhaseLint_MissingName(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runPhaseLint(nil, &out, &errb); code != 10 {
		t.Errorf("missing name should be a usage error (10); got %d", code)
	}
}

func TestPhaseLint_UnknownPhaseFailOpen(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "phases"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	if code := runPhaseLint([]string{"ghost"}, &out, &errb); code != 0 {
		t.Errorf("unknown phase is a fail-open warning (exit 0); got %d", code)
	}
}
