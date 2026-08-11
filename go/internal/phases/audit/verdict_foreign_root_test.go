package audit

// verdict_foreign_root_test.go — the third leg of the cycle-1434 class: the
// CLI-written acs-verdict.json (minted under the WRONG state root, 3 false
// reds) suppressed the phase's own correct-root generation because the
// verdict-exists gate honored ANY pre-staged file. A file STAMPED with a
// project_root that differs from the phase's own is a foreign-root artifact —
// regenerate. Unstamped files (operator/CI pre-stage, every pre-stamp
// verdict) stay honored untouched: absence means "unstamped", never
// "mismatch".

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeACSVerdictWithRoot(t *testing.T, ws string, redCount int, projectRoot string) {
	t.Helper()
	v := map[string]any{
		"cycle":        42,
		"red_count":    redCount,
		"total":        10,
		"predicates":   []any{},
		"project_root": projectRoot,
	}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), b, 0o644); err != nil {
		t.Fatalf("write verdict: %v", err)
	}
}

func TestRun_ACSVerdictForeignRoot_Regenerated(t *testing.T) {
	ws := t.TempDir()
	// The 1434 shape: a red verdict stamped under the WRONG root.
	writeACSVerdictWithRoot(t, ws, 3, "/console-not-plane")
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	genCalls := 0
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("body"),
		GenerateVerdict: func(req core.PhaseRequest) error {
			genCalls++
			// The regenerated (correct-root) verdict is green.
			writeACSVerdictWithRoot(t, req.Workspace, 0, req.ProjectRoot)
			return nil
		},
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if genCalls != 1 {
		t.Errorf("GenerateVerdict called %d times, want 1 (foreign-root artifact must be regenerated)", genCalls)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (correct-root regeneration is green) — the wrong-root artifact won", resp.Verdict)
	}
	// The foreign artifact is EVIDENCE — preserved, not clobbered (the
	// incident class was "the misdiagnosis was invisible from the file").
	data, err := os.ReadFile(filepath.Join(ws, "acs-verdict.foreign.json"))
	if err != nil {
		t.Fatalf("foreign verdict not preserved: %v", err)
	}
	var preserved struct {
		RedCount    int    `json:"red_count"`
		ProjectRoot string `json:"project_root"`
	}
	if err := json.Unmarshal(data, &preserved); err != nil {
		t.Fatal(err)
	}
	if preserved.RedCount != 3 || preserved.ProjectRoot != "/console-not-plane" {
		t.Errorf("preserved foreign verdict = %+v, want the original red_count=3 under /console-not-plane", preserved)
	}
}

func TestRun_ACSVerdictMatchingRoot_Honored(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdictWithRoot(t, ws, 0, "/p")
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	genCalls := 0
	phase := New(Config{
		Bridge:          fb,
		Prompts:         fakePromptsFS("body"),
		GenerateVerdict: func(core.PhaseRequest) error { genCalls++; return nil },
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if genCalls != 0 {
		t.Errorf("GenerateVerdict called %d times, want 0 (matching-root stamp is honored)", genCalls)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
}
