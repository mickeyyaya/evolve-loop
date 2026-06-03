package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Layer 3 CLI: `evolve phase verify` — the agent-callable self-check. Same
// verifier the host gate uses (go/internal/deliverable). ADR-0034.

func runVerify(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runPhaseVerify(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestPhaseVerify_ValidArtifact_Exit0(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"),
		[]byte("## Changes\n- foo.go\nVerdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errb := runVerify(t, "build", "--workspace="+ws)
	if code != 0 {
		t.Errorf("exit=%d want 0; stderr=%s", code, errb)
	}
}

func TestPhaseVerify_MissingArtifact_NonZeroNamesPath(t *testing.T) {
	ws := t.TempDir()
	code, _, errb := runVerify(t, "build", "--workspace="+ws)
	if code == 0 {
		t.Fatal("exit=0 want non-zero for missing artifact")
	}
	wantPath := filepath.Join(ws, "build-report.md")
	if !strings.Contains(errb, wantPath) {
		t.Errorf("stderr must name the expected path %q; got %q", wantPath, errb)
	}
}

func TestPhaseVerify_JSONOutput(t *testing.T) {
	ws := t.TempDir()
	code, out, _ := runVerify(t, "build", "--workspace="+ws, "--json")
	if code == 0 {
		t.Fatal("want non-zero for missing artifact")
	}
	var res struct {
		OK         bool `json:"ok"`
		Violations []struct {
			Code string `json:"code"`
		} `json:"violations"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("--json must emit valid JSON to stdout: %v\n%s", err, out)
	}
	if res.OK {
		t.Error("ok=true, want false")
	}
}

func TestPhaseVerify_SpaceSeparatedFlags(t *testing.T) {
	// Space-separated form `verify build --workspace <dir>` must work (not just
	// the `=` form). Regression for the reorderArgs flag-swallow bug.
	ws := t.TempDir()
	code, _, errb := runVerify(t, "build", "--workspace", ws, "--json")
	if code == 0 {
		t.Fatalf("want non-zero for missing artifact; stderr=%s", errb)
	}
	// The phase must be parsed as build (not the workspace value).
	if strings.Contains(errb, "unknown phase") {
		t.Errorf("space-separated flags mis-parsed: %s", errb)
	}
}

func TestPhaseVerify_UnknownPhase_Usage(t *testing.T) {
	code, _, _ := runVerify(t, "nope", "--workspace="+t.TempDir())
	if code != 10 {
		t.Errorf("exit=%d want 10 (usage) for unknown phase", code)
	}
}

func TestPhaseVerify_MissingPhaseArg(t *testing.T) {
	code, _, _ := runVerify(t, "--workspace=/tmp")
	if code != 10 {
		t.Errorf("exit=%d want 10 when phase name omitted", code)
	}
}

func TestPhaseVerify_Advisor_EvolveDirDefault(t *testing.T) {
	// Orchestrator deliverable lives in --evolve-dir.
	ev := t.TempDir()
	if err := os.WriteFile(filepath.Join(ev, "cycle-state.json"),
		[]byte(`{"cycle_id":1,"phase":"tdd"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errb := runVerify(t, "orchestrator", "--evolve-dir="+ev)
	if code != 0 {
		t.Errorf("exit=%d want 0; stderr=%s", code, errb)
	}
}
