package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dyingServerTmux serves scripted frames, then reports the session (and any
// capture) gone: a server killed under a live launch.
type dyingServerTmux struct {
	*FakeTmuxController
	captures  int
	aliveCaps int // captures while the server still lives
}

func (d *dyingServerTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	d.captures++
	if d.captures > d.aliveCaps {
		return "", nil // dead server: nothing to capture
	}
	return d.FakeTmuxController.CapturePane(ctx, session, scrollback)
}

func (d *dyingServerTmux) HasSession(ctx context.Context, session string) bool {
	return d.captures <= d.aliveCaps
}

// This scenario's baseline capture comes back empty; the first interval
// checkpoint is the only good frame before the server dies, so lastGoodPane
// must come from the checkpoint, not only the baseline.
func TestEscalationFinalPaneFromEmptyBaseline(t *testing.T) {
	cfg := fixtureConfig(t)
	const evidence = "Working… (esc to interrupt) — mid-turn frame"
	// Frame budget (the auto-respond tick consumes one capture per loop
	// iteration before the checkpoint): boot marker → empty baseline →
	// tick(iter1) → tick(iter2) → checkpoint(iter2)=evidence → dead after.
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "", evidence, evidence, evidence}}
	tm := &dyingServerTmux{FakeTmuxController: base, aliveCaps: 5}

	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "claude-tmux", session: "evidence-late-baseline", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1,
	})
	if err != nil || code != ExitArtifactTimeout {
		t.Fatalf("runTmuxREPL = (%d,%v), want ExitArtifactTimeout", code, err)
	}
	b, rerr := os.ReadFile(filepath.Join(cfg.Workspace, "build-escalation-report.json"))
	if rerr != nil {
		t.Fatalf("escalation report missing: %v", rerr)
	}
	var rep struct {
		FinalPane string `json:"final_pane"`
	}
	if uerr := json.Unmarshal(b, &rep); uerr != nil {
		t.Fatalf("unmarshal: %v", uerr)
	}
	if !strings.Contains(rep.FinalPane, evidence) {
		t.Errorf("final_pane missed the checkpoint-time evidence; got:\n%s", rep.FinalPane)
	}
}

func TestEscalationFinalPaneSurvivesServerDeath(t *testing.T) {
	cfg := fixtureConfig(t)
	const evidence = "TOOL CALL: go test ./... — the last real work the agent did"
	base := &FakeTmuxController{CaptureFrames: []string{"❯", evidence}}
	tm := &dyingServerTmux{FakeTmuxController: base, aliveCaps: 2}

	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "claude-tmux", session: "evidence-survives", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1,
	})
	if err != nil || code != ExitArtifactTimeout {
		t.Fatalf("runTmuxREPL = (%d,%v), want ExitArtifactTimeout (artifact never written, session died)", code, err)
	}

	b, rerr := os.ReadFile(filepath.Join(cfg.Workspace, "build-escalation-report.json"))
	if rerr != nil {
		t.Fatalf("escalation report missing: %v", rerr)
	}
	var rep struct {
		FinalPane string `json:"final_pane"`
	}
	if uerr := json.Unmarshal(b, &rep); uerr != nil {
		t.Fatalf("unmarshal escalation report: %v", uerr)
	}
	if !strings.Contains(rep.FinalPane, evidence) {
		t.Errorf("final_pane lost the last real pane after server death (cycle-286 masked-evidence class); got:\n%s", rep.FinalPane)
	}
}
