package phasecmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhaseInventory_BuildFromPhaseRoots(t *testing.T) {
	root := t.TempDir()
	writeUserPhase(t, root, "smell-scan", `{
  "name": "smell-scan", "kind": "llm", "optional": true, "archetype": "evaluate",
  "categories": ["refactor"],
  "outputs": { "files": ["smell-scan-report.md"] },
  "classify": { "require_sections": ["Findings"], "verdict_on_pass": "PASS" }
}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv("EVOLVE_PHASE_ROOTS", "")

	var out, errb bytes.Buffer
	code := RunPhaseInventory([]string{"build", "--force"}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("build exit = %d (stderr=%q)", code, errb.String())
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "phase-inventory.json"))
	if err != nil {
		t.Fatalf("inventory not written: %v", err)
	}
	for _, want := range []string{`"smell-scan"`, `"refactor"`, `"category_index"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("inventory missing %s; got %s", want, raw)
		}
	}
}

func TestPhaseInventory_UnknownSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := RunPhaseInventory([]string{"wat"}, nil, &out, &errb); code != 10 {
		t.Errorf("unknown subcommand exit = %d, want 10", code)
	}
}
