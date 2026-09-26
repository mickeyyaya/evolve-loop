package phasecmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestPhaseVerify_UserPhaseParity(t *testing.T) {
	project := t.TempDir()
	// Minimal registry so mergedCatalog's built-in Load succeeds.
	reg := filepath.Join(project, "docs", "architecture")
	if err := os.MkdirAll(reg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reg, "phase-registry.json"), []byte(`{"schema_version":4,"phases":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	phaseDir := filepath.Join(project, ".evolve", "phases", "widget-check")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	phaseJSON := `{
  "name": "widget-check",
  "kind": "llm",
  "optional": true,
  "archetype": "evaluate",
  "outputs": { "files": [".evolve/runs/cycle-{cycle}/widget-check-report.md"] },
  "classify": { "require_sections": ["Findings"] }
}`
	if err := os.WriteFile(filepath.Join(phaseDir, "phase.json"), []byte(phaseJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(project, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	// At the enforce default a verdict-declaring deliverable needs the machine-readable sentinel.
	deliverable := "## Findings\n- ok\n" + phasecontract.RenderVerdictSentinel("widget-check", "PASS") + "\n"
	if err := os.WriteFile(filepath.Join(ws, "widget-check-report.md"), []byte(deliverable), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	var out, errb bytes.Buffer
	code := runPhaseVerify([]string{"widget-check", "--workspace", ws}, &out, &errb)
	if code != 0 {
		t.Fatalf("verify user phase: exit=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}

	if err := os.WriteFile(filepath.Join(ws, "widget-check-report.md"), []byte("no heading\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := runPhaseVerify([]string{"widget-check", "--workspace", ws}, &out, &errb); code != 1 {
		t.Fatalf("malformed user phase should exit 1; got %d (stderr=%q)", code, errb.String())
	}
}
