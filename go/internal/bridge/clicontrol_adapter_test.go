package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge/clicontrol"
)

// TestController_Do_ResolvesAndCaptures verifies the abstract Controller hides
// the CLI implementation: given the family + abstract event, it resolves the
// family's interactive driver, looks up the concrete command from the mapping
// table, and returns the captured response. The pipeline never names a slash
// command — that translation lives entirely behind Do().
func TestController_Do_ResolvesAndCaptures(t *testing.T) {
	var gotCLI, gotCmd string
	c := &cliController{
		resolve: func(cli string) (Manifest, error) {
			return Manifest{CLI: cli, Binary: "x", Controls: map[string]ControlSpec{
				"usage": {Send: "/usage", Await: "prompt_marker"},
			}}, nil
		},
		capture: func(_ context.Context, cli, command, _ string) (string, error) {
			gotCLI, gotCmd = cli, command
			return "Usage: 12% of weekly limit", nil
		},
	}
	resp, err := c.Do(context.Background(), "claude", clicontrol.EventUsage)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotCLI != "claude-tmux" {
		t.Errorf("resolved cli=%q, want claude-tmux (family→interactive driver)", gotCLI)
	}
	if gotCmd != "/usage" {
		t.Errorf("sent command=%q, want /usage (from mapping table)", gotCmd)
	}
	if !strings.Contains(resp.Pane, "12%") || resp.Family != "claude" || resp.Event != clicontrol.EventUsage {
		t.Errorf("resp=%+v, want family=claude event=usage pane~12%%", resp)
	}
}

// TestController_perFamilyConfig verifies one Controller probes any family
// correctly: Do derives the per-family driver name + launch realization from a
// family-agnostic template config (preserving the shared workspace), so a
// single Controller can fan out across all families without cross-CLI flag
// bleed.
func TestController_perFamilyConfig(t *testing.T) {
	c := &cliController{cfg: &Config{Workspace: "/tmp/ws", AllowBypass: true}}
	got := c.perFamilyConfig("claude-tmux")
	if got.CLI != "claude-tmux" {
		t.Errorf("CLI=%q, want claude-tmux", got.CLI)
	}
	if got.Workspace != "/tmp/ws" {
		t.Errorf("Workspace=%q, want the template's /tmp/ws", got.Workspace)
	}
	// bypass realization for claude resolves to its skip-permissions launch flag.
	if len(got.Realization.LaunchFlags) == 0 {
		t.Error("expected a non-empty per-family realization for claude bypass")
	}
}

// TestController_Do_UnsupportedEvent verifies a family that declares no mapping
// for the event yields ErrUnsupported WITHOUT booting a REPL (capture is never
// called) — the honest no-op for e.g. ollama + usage.
func TestController_Do_UnsupportedEvent(t *testing.T) {
	c := &cliController{
		resolve: func(cli string) (Manifest, error) {
			return Manifest{CLI: cli, Binary: "x"}, nil // no controls block
		},
		capture: func(context.Context, string, string, string) (string, error) {
			t.Fatal("capture must not run for an unsupported event")
			return "", nil
		},
	}
	_, err := c.Do(context.Background(), "ollama", clicontrol.EventUsage)
	if !errors.Is(err, clicontrol.ErrUnsupported) {
		t.Fatalf("err=%v, want ErrUnsupported", err)
	}
}

// TestNewController_UnsupportedNoBoot exercises the production NewController +
// the real resolve path: probing a usage event against ollama (a local model
// with no usage command) returns ErrUnsupported WITHOUT booting a REPL, so the
// constructor is covered with no tmux.
func TestNewController_UnsupportedNoBoot(t *testing.T) {
	ctrl := NewController(&Config{Workspace: t.TempDir()}, recipeDeps(&fakeTmux{}))
	_, err := ctrl.Do(context.Background(), "ollama", clicontrol.EventUsage)
	if !errors.Is(err, clicontrol.ErrUnsupported) {
		t.Fatalf("Do(ollama, usage) err=%v, want ErrUnsupported (no usage command, no boot)", err)
	}
}

// TestController_Do_ResolveError verifies a manifest-load failure propagates
// (a typo'd family must error loudly, not silently no-op).
func TestController_Do_ResolveError(t *testing.T) {
	wantErr := errors.New("no manifest")
	c := &cliController{
		resolve: func(string) (Manifest, error) { return Manifest{}, wantErr },
		capture: func(context.Context, string, string, string) (string, error) { return "", nil },
	}
	if _, err := c.Do(context.Background(), "nope", clicontrol.EventStatus); err == nil {
		t.Fatal("want resolve error propagated")
	}
}

// TestCaptureControl_SendsGivenCommand verifies the executor is generalized
// from the hardcoded /help to an arbitrary table-driven command: the body
// injected into the REPL is exactly the command passed, and the captured pane
// is returned. (CaptureHelp keeps delegating to this with "/help".)
func TestCaptureControl_SendsGivenCommand(t *testing.T) {
	ws := t.TempDir()
	tx := &fakeTmux{existing: map[string]bool{"evolve-bridge-named-covctl": true}, paneSeq: []string{"quota healthy ❯"}}
	cfg := &Config{
		CLI: "claude-tmux", Workspace: ws, Agent: "control",
		SessionName: "covctl", Realization: RealizeFor("claude-tmux", LaunchIntent{}),
	}
	pane, err := captureControl(context.Background(), cfg, recipeDeps(tx), "claude-tmux", "/usage", helpCaptureSettleTicks)
	if err != nil {
		t.Fatalf("captureControl: %v", err)
	}
	if !strings.Contains(pane, "quota healthy") {
		t.Errorf("pane=%q, want the captured response", pane)
	}
	// injectText writes the command body to <ws>/.bridge-inbox/<agent>-inject.txt
	// before pasting — proving the command is the one we passed, not /help.
	body, rerr := os.ReadFile(filepath.Join(ws, ".bridge-inbox", "control-inject.txt"))
	if rerr != nil {
		t.Fatalf("read inject scratch: %v", rerr)
	}
	if strings.TrimSpace(string(body)) != "/usage" {
		t.Errorf("injected body=%q, want /usage", string(body))
	}
}
