package debugger

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// writeDecision stages a debug-decision.json under dir. An empty body
// means "write no file" so the missing-file path can be exercised.
func writeDecision(t *testing.T, dir, body string) {
	t.Helper()
	if body == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, decisionFilename), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestClassify is the load-bearing behavioral test: it pins the
// action→verdict→signals mapping and, critically, the SAFE-DEFAULT
// (BLOCK/FAIL) on any parse failure or unknown action. Never RESHIP on
// a malformed file.
func TestClassify(t *testing.T) {
	cases := []struct {
		name          string
		body          string // "" = no file
		wantVerdict   string
		wantAction    string // expected Signals["debugger.action"]; "" = key absent
		wantRerun     string // expected Signals["debugger.rerun_phase"]; "" = key absent
		wantRootCause string // expected Signals["debugger.root_cause"]; "" = not asserted
	}{
		{
			name:          "reship maps to PASS with ship next",
			body:          `{"action":"RESHIP","fix_applied":"re-ran ff-merge","root_cause":"stale ref","reasoning":"safe retry"}`,
			wantVerdict:   core.VerdictPASS,
			wantAction:    actionReship,
			wantRootCause: "stale ref",
		},
		{
			name:        "rerun_phase audit maps to PASS carrying rerun_phase signal",
			body:        `{"action":"RERUN_PHASE","rerun_phase":"audit","root_cause":"stale audit binding","reasoning":"head moved"}`,
			wantVerdict: core.VerdictPASS,
			wantAction:  actionRerunPhase,
			wantRerun:   "audit",
		},
		{
			name:        "block maps to FAIL",
			body:        `{"action":"BLOCK","root_cause":"integrity breach","reasoning":"tamper detected"}`,
			wantVerdict: core.VerdictFAIL,
			wantAction:  actionBlock,
		},
		{
			name:        "missing file is a safe block (FAIL)",
			body:        "",
			wantVerdict: core.VerdictFAIL,
			wantAction:  actionBlock,
		},
		{
			name:        "malformed JSON is a safe block (FAIL)",
			body:        `{"action": "RESHIP", `,
			wantVerdict: core.VerdictFAIL,
			wantAction:  actionBlock,
		},
		{
			name:        "unknown action string is a safe block (FAIL)",
			body:        `{"action":"YOLO","reasoning":"???"}`,
			wantVerdict: core.VerdictFAIL,
			wantAction:  actionBlock,
		},
		{
			name:        "empty action is a safe block (FAIL)",
			body:        `{"action":"","reasoning":"forgot"}`,
			wantVerdict: core.VerdictFAIL,
			wantAction:  actionBlock,
		},
		{
			name:        "rerun_phase with no phase named falls back to audit",
			body:        `{"action":"RERUN_PHASE","root_cause":"precondition","reasoning":"redo"}`,
			wantVerdict: core.VerdictPASS,
			wantAction:  actionRerunPhase,
			wantRerun:   string(core.PhaseAudit),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDecision(t, dir, tc.body)

			verdict, signals, _ := Classify(dir)

			if verdict != tc.wantVerdict {
				t.Errorf("verdict = %q, want %q", verdict, tc.wantVerdict)
			}
			if got := signals[signalAction]; got != tc.wantAction {
				t.Errorf("Signals[%q] = %q, want %q", signalAction, got, tc.wantAction)
			}
			if tc.wantRerun != "" {
				if got := signals[signalRerunPhase]; got != tc.wantRerun {
					t.Errorf("Signals[%q] = %q, want %q", signalRerunPhase, got, tc.wantRerun)
				}
			}
			if tc.wantRootCause != "" {
				if got := signals[signalRootCause]; got != tc.wantRootCause {
					t.Errorf("Signals[%q] = %q, want %q", signalRootCause, got, tc.wantRootCause)
				}
			}
		})
	}
}

// TestClassifyNeverReshipOnParseFailure is an explicit safety pin: a
// parse failure must NEVER yield a RESHIP signal, even partially. This
// is the integrity-critical invariant of the phase.
func TestClassifyNeverReshipOnParseFailure(t *testing.T) {
	dir := t.TempDir()
	writeDecision(t, dir, `{"action":"RESHIP"`) // truncated JSON

	verdict, signals, _ := Classify(dir)
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %q, want FAIL on parse failure", verdict)
	}
	if signals[signalAction] == actionReship {
		t.Fatalf("parse failure produced a RESHIP action — must default to BLOCK")
	}
}

func TestNextPhaseFor(t *testing.T) {
	cases := []struct {
		action string
		want   string
	}{
		{actionReship, "ship"},
		{actionBlock, ""},
		{actionRerunPhase, ""},
		{"UNKNOWN", ""},
	}
	for _, tc := range cases {
		if got := nextPhaseFor(tc.action); got != tc.want {
			t.Errorf("nextPhaseFor(%q) = %q, want %q", tc.action, got, tc.want)
		}
	}
}

func TestHooksMethods(t *testing.T) {
	h := hooks{}
	if got := h.PhaseName(); got != "debugger" {
		t.Errorf("PhaseName() = %q, want %q", got, "debugger")
	}
	if got := h.AgentPromptName(); got != "evolve-debugger" {
		t.Errorf("AgentPromptName() = %q, want %q", got, "evolve-debugger")
	}
	req := core.PhaseRequest{}
	if got := h.ArtifactFilename(req); got != "debug-decision.json" {
		t.Errorf("ArtifactFilename() = %q, want %q", got, "debug-decision.json")
	}
	if got := h.DefaultModel(); got != "opus" {
		t.Errorf("DefaultModel() = %q, want %q", got, "opus")
	}
}

func TestHooksComposePrompt(t *testing.T) {
	h := hooks{}
	req := core.PhaseRequest{
		Cycle:       197,
		GoalHash:    "goalhash123",
		ProjectRoot: "/root",
		Workspace:   "/workspace",
		Worktree:    "/worktree",
		Context: map[string]string{
			"ship_error_code":  "ERR_456",
			"ship_error_class": "runtime_panic",
			"ship_error_stage": "validation",
			"ship_error_debug": "nil pointer deref",
		},
	}
	prompt := h.ComposePrompt("original body", req)
	expectedSubstrings := []string{
		"original body",
		"cycle: 197",
		"goal_hash: goalhash123",
		"project_root: /root",
		"workspace: /workspace",
		"worktree: /worktree",
		"ship_error_code: ERR_456",
		"ship_error_class: runtime_panic",
		"ship_error_stage: validation",
		"ship_error_debug: nil pointer deref",
	}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(prompt, sub) {
			t.Errorf("ComposePrompt() output missing %q", sub)
		}
	}
}

type fakeBridge struct {
	resp          core.BridgeResponse
	err           error
	writeArtifact string

	gotReq core.BridgeRequest
}

func (f *fakeBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.gotReq = req
	if f.writeArtifact != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(f.writeArtifact), 0o644)
		f.resp.Stdout = f.writeArtifact
	}
	return f.resp, f.err
}

func (f *fakeBridge) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func TestPhase_Run(t *testing.T) {
	ws := t.TempDir()
	
	decisionBody := `{"action":"RERUN_PHASE","rerun_phase":"audit","root_cause":"stale audit binding","reasoning":"head moved"}`
	fb := &fakeBridge{
		writeArtifact: decisionBody,
		resp:          core.BridgeResponse{CostUSD: 0.15},
	}
	
	clock := func() time.Time {
		return time.Unix(1_700_000_000, 0)
	}
	
	pl := prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-debugger.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-debugger\n---\n# Debugger body"),
		},
	})
	
	phase := New(Config{
		Bridge:  fb,
		Prompts: pl,
		NowFn:   clock,
	})
	
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       197,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
		GoalHash:    "deadbeef",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict = %q, want PASS", resp.Verdict)
	}
	
	if resp.Signals == nil {
		t.Fatal("expected response to carry signals, got nil")
	}
	
	if got := resp.Signals[signalAction]; got != actionRerunPhase {
		t.Errorf("Signals[%s] = %q, want %q", signalAction, got, actionRerunPhase)
	}
	
	if got := resp.Signals[signalRerunPhase]; got != "audit" {
		t.Errorf("Signals[%s] = %q, want %q", signalRerunPhase, got, "audit")
	}
}
