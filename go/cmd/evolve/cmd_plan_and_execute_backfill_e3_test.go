package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPassWithEnv_AppliesTheEnvForTheCallAndRestoresBothPriorAndUnsetKeys(t *testing.T) {
	root := filepath.Join(t.TempDir(), "pass-root")
	t.Setenv("EVOLVE_PROJECT_ROOT", "/prior/root")
	t.Setenv("E3_PASS_FRESH", "")
	if err := os.Unsetenv("E3_PASS_FRESH"); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer

	code := runPassWithEnv("no-such-phase", nil, map[string]string{
		"EVOLVE_PROJECT_ROOT": root,
		"E3_PASS_FRESH":       "during",
	}, &out, &errb)

	if code != 10 {
		t.Fatalf("code = %d, want runPhase's 10 for an unknown phase passed through; stderr %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), filepath.Join(root, "docs", "architecture", "phase-registry.json")) {
		t.Errorf("the phase did not run under the pass env: stderr %q names no %s registry", errb.String(), root)
	}
	if got := os.Getenv("EVOLVE_PROJECT_ROOT"); got != "/prior/root" {
		t.Errorf("EVOLVE_PROJECT_ROOT after the pass = %q, want the prior /prior/root restored", got)
	}
	if v, ok := os.LookupEnv("E3_PASS_FRESH"); ok {
		t.Errorf("E3_PASS_FRESH after the pass = %q, want it unset again", v)
	}
}

func TestRunPassWithEnv_NoEnvRunsThePhaseUnchanged(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "/kept")
	var out, errb bytes.Buffer

	code := runPassWithEnv("no-such-phase", []byte("{}"), nil, &out, &errb)

	if code != 10 || !strings.Contains(errb.String(), `unknown phase "no-such-phase"`) {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
	if os.Getenv("EVOLVE_PROJECT_ROOT") != "/kept" {
		t.Errorf("a pass with no env changed EVOLVE_PROJECT_ROOT to %q", os.Getenv("EVOLVE_PROJECT_ROOT"))
	}
}
