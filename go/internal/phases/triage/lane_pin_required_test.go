package triage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

const pinnedTopNArtifact = "## top_n\n- id: a\n"

func writeLanePin(t *testing.T, ws, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, committedset.LanePinFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runTriageLane(t *testing.T, env map[string]string, pin string) (core.PhaseResponse, *fakeBridge) {
	t.Helper()
	ws := t.TempDir()
	if pin != "" {
		writeLanePin(t, ws, pin)
	}
	fb := &fakeBridge{writeArtifact: pinnedTopNArtifact, writeDecision: `{"top_n":[{"id":"a"}]}`}
	resp, err := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")}).Run(context.Background(), core.PhaseRequest{
		Cycle: 42, ProjectRoot: t.TempDir(), Workspace: ws, Env: env,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return resp, fb
}

func TestRun_FleetLaneWithoutAPinIsRefusedBeforeTheAgent(t *testing.T) {
	fleet := map[string]string{ipcenv.FleetKey: "1"}
	for name, pin := range map[string]string{
		"absent":    "",
		"empty ids": `{"todo_ids":[],"goal_hash":"g"}`,
		"blank ids": `{"todo_ids":[" "],"goal_hash":"g"}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp, fb := runTriageLane(t, fleet, pin)
			if fb.gotReq.ArtifactPath != "" {
				t.Fatalf("the triage agent was launched for a fleet lane with no pin: %+v", fb.gotReq)
			}
			if resp.Verdict != core.VerdictFAIL {
				t.Errorf("Verdict = %q, want FAIL", resp.Verdict)
			}
			codes := cyclestate.ErrorCodes(resp.Diagnostics)
			if len(codes) != 1 || codes[0] != cyclestate.DiagCodeTriageLanePinMissing {
				t.Fatalf("error codes = %v, want [%s]", codes, cyclestate.DiagCodeTriageLanePinMissing)
			}
			if d := resp.Diagnostics[0]; d.Subject != "cycle-42" || !strings.Contains(d.Message, committedset.LanePinFile) {
				t.Errorf("the refusal must name the lane and the pin file: %+v", d)
			}
		})
	}
}

func TestRun_FleetLaneWithAPinReachesTheAgent(t *testing.T) {
	resp, fb := runTriageLane(t, map[string]string{ipcenv.FleetKey: "1"}, `{"todo_ids":["a"],"goal_hash":"g"}`)
	if fb.gotReq.ArtifactPath == "" {
		t.Fatal("a pinned fleet lane must reach the triage agent")
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict = %q, want PASS; diags = %+v", resp.Verdict, resp.Diagnostics)
	}
}

func TestRun_SequentialCycleWithoutAPinIsUnaffected(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"no fleet key":    nil,
		"fleet key false": {ipcenv.FleetKey: "0"},
	} {
		t.Run(name, func(t *testing.T) {
			resp, fb := runTriageLane(t, env, "")
			if fb.gotReq.ArtifactPath == "" {
				t.Fatal("a sequential cycle must reach the triage agent without a pin")
			}
			if resp.Verdict != core.VerdictPASS {
				t.Errorf("Verdict = %q, want PASS; diags = %+v", resp.Verdict, resp.Diagnostics)
			}
		})
	}
}

func TestShouldSkip_DisabledTriageSkipsBeforeThePinCheck(t *testing.T) {
	root := t.TempDir()
	regDir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte(`{"phases":[{"name":"triage","enabled":"off"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	skipped, verdict, next, diags := hooks{}.ShouldSkip(core.PhaseRequest{
		Cycle: 7, ProjectRoot: root, Workspace: t.TempDir(), Env: map[string]string{ipcenv.FleetKey: "1"},
	})
	if !skipped || verdict != core.VerdictSKIPPED || next != string(core.PhaseTDD) || len(diags) != 0 {
		t.Errorf("ShouldSkip = (%v, %q, %q, %+v), want a plain SKIPPED to tdd: a lane that runs no triage has no agent top_n to refuse", skipped, verdict, next, diags)
	}
}
