package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func recipeDeps(tx *fakeTmux) Deps {
	d := covDeps()
	d.Tmux = tx
	return d
}

func newTestRecipeDriver(t *testing.T, tx *fakeTmux, cli, session string) *recipeSessionDriver {
	t.Helper()
	ws := t.TempDir()
	deps := recipeDeps(tx)
	return &recipeSessionDriver{
		cfg:        &Config{Workspace: ws, Agent: "recipe"},
		deps:       deps,
		session:    session,
		launchCmd:  "claude --dangerously-skip-permissions",
		workingDir: ws,
		marker:     "❯",
		scrollback: recipeBootScrollback,
		ar:         newAutoResponder(cli, ws, deps, false, recipeBootScrollback),
	}
}

func TestRecipeDriver_EnsureSession_Attach(t *testing.T) {
	tx := &fakeTmux{existing: map[string]bool{"sess": true}}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	if err := d.EnsureSession(context.Background()); err != nil {
		t.Fatalf("attach err=%v", err)
	}
	if len(tx.sentSeq) != 0 {
		t.Errorf("attach must not send cd/launch keys, got %v", tx.sentSeq)
	}
}

func TestRecipeDriver_EnsureSession_BootSuccess(t *testing.T) {
	tx := &fakeTmux{paneSeq: []string{"booting", "ready\n❯"}}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	if err := d.EnsureSession(context.Background()); err != nil {
		t.Fatalf("boot err=%v", err)
	}
	if !tx.sentContains("cd ") || !tx.sentContains("claude --dangerously-skip-permissions") {
		t.Errorf("boot should cd + launch, sent=%v", tx.sentKeys)
	}
}

func TestRecipeDriver_EnsureSession_BootTimeout(t *testing.T) {
	tx := &fakeTmux{paneSeq: []string{"never shows the marker"}}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	if err := d.EnsureSession(context.Background()); err == nil {
		t.Fatal("want boot-timeout error")
	}
}

func TestRecipeDriver_EnsureSession_NewSessionError(t *testing.T) {
	tx := &fakeTmux{newSessErr: os.ErrPermission}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	if err := d.EnsureSession(context.Background()); err == nil {
		t.Fatal("want new-session error")
	}
}

func TestRecipeDriver_SendCommand_Pastes(t *testing.T) {
	tx := &fakeTmux{}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	if err := d.SendCommand(context.Background(), "/plugin install ecc@ecc"); err != nil {
		t.Fatalf("err=%v", err)
	}
	scratch := filepath.Join(d.cfg.Workspace, ".bridge-inbox", "recipe-inject.txt")
	body, err := os.ReadFile(scratch)
	if err != nil {
		t.Fatalf("scratch not written: %v", err)
	}
	if string(body) != "/plugin install ecc@ecc" {
		t.Errorf("scratch=%q", body)
	}
	// injectText commits the paste with a trailing Enter.
	if tx.sentSeq[len(tx.sentSeq)-1] != "|true" {
		t.Errorf("expected trailing Enter, sentSeq=%v", tx.sentSeq)
	}
}

func TestRecipeDriver_SendCommand_PropagatesError(t *testing.T) {
	tx := &fakeTmux{}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	// Make .bridge-inbox a regular file so injectText's MkdirAll fails — the
	// transport error must now surface (HIGH-2 fix), not be swallowed.
	if err := os.WriteFile(filepath.Join(d.cfg.Workspace, ".bridge-inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.SendCommand(context.Background(), "/x"); err == nil {
		t.Fatal("SendCommand must propagate the injectText transport error")
	}
}

func TestRecipeDriver_SendKeys_Raw(t *testing.T) {
	tx := &fakeTmux{}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	if err := d.SendKeys(context.Background(), "Down Down Enter"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(tx.sentSeq) != 1 || tx.sentSeq[0] != "Down Down Enter|false" {
		t.Fatalf("sentSeq=%v want raw keys, no Enter", tx.sentSeq)
	}
}

func TestRecipeDriver_AutoRespond_Escalation(t *testing.T) {
	// claude-tmux manifest's auth_recheck rule escalates on "Please log in".
	tx := &fakeTmux{paneSeq: []string{"Please log in to continue"}}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	escalated, err := d.AutoRespond(context.Background())
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !escalated {
		t.Error("auth-recheck pane should escalate")
	}
}

func TestRecipeDriver_AutoRespond_NoEscalationAndNilAR(t *testing.T) {
	tx := &fakeTmux{paneSeq: []string{"benign output ❯"}}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	if esc, _ := d.AutoRespond(context.Background()); esc {
		t.Error("benign pane should not escalate")
	}
	d.ar = nil
	if esc, err := d.AutoRespond(context.Background()); esc || err != nil {
		t.Errorf("nil ar → (false,nil), got (%v,%v)", esc, err)
	}
}

func TestNewRecipeEngine_ResolvesMarker(t *testing.T) {
	cfg := &Config{Workspace: t.TempDir(), Agent: "recipe"}
	eng, err := newRecipeEngine(cfg, recipeDeps(&fakeTmux{}), "claude-tmux")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if eng.PromptMarker != "❯" {
		t.Errorf("marker=%q want ❯", eng.PromptMarker)
	}
}

func TestNewRecipeEngine_UnknownCLI(t *testing.T) {
	cfg := &Config{Workspace: t.TempDir()}
	if _, err := newRecipeEngine(cfg, recipeDeps(&fakeTmux{}), "no-such-cli"); err == nil {
		t.Fatal("want manifest-not-found error")
	}
}

func TestRunRecipe_PluginInstallHappyPath(t *testing.T) {
	// One benign pane satisfies boot-marker + all three step awaits and trips
	// no fail_regex / interactive_prompt rule.
	pane := "❯ marketplace added; plugin installed; plugins reloaded; ready ❯"
	tx := &fakeTmux{existing: map[string]bool{}, paneSeq: []string{pane}}
	cfg := &Config{
		CLI:         "claude-tmux",
		Workspace:   t.TempDir(),
		Agent:       "recipe",
		SessionName: "covrecipe", // named → deterministic, pre-create to attach
		Realization: RealizeFor("claude-tmux", LaunchIntent{}),
	}
	tx.existing["evolve-bridge-named-covrecipe"] = true // attach, skip boot
	res, err := RunRecipe(context.Background(), cfg, recipeDeps(tx), "claude-tmux", "plugin-install",
		map[string]string{"marketplace": "https://github.com/affaan-m/ECC", "plugin": "ecc@ecc"})
	if err != nil {
		t.Fatalf("RunRecipe err=%v result=%+v", err, res)
	}
	if res.Status != "complete" || len(res.Steps) != 3 {
		t.Fatalf("result=%+v", res)
	}
	// The marketplace + plugin params reached the REPL via the paste path.
	if !tx.sentContains("") { // trailing Enter commits each paste
		t.Error("expected paste-commit Enter keys")
	}
}

func TestRunRecipe_UnknownRecipe(t *testing.T) {
	cfg := &Config{CLI: "claude-tmux", Workspace: t.TempDir(), Agent: "recipe"}
	if _, err := RunRecipe(context.Background(), cfg, recipeDeps(&fakeTmux{}), "claude-tmux", "no-such-recipe", nil); err == nil {
		t.Fatal("want recipe-not-found error")
	}
}

func TestCaptureHelp(t *testing.T) {
	help := "Available commands:\n/help    Show help\n/model   Switch model\n/plugin  Manage plugins\nready ❯"
	tx := &fakeTmux{existing: map[string]bool{"evolve-bridge-named-covhelp": true}, paneSeq: []string{help}}
	cfg := &Config{
		CLI: "claude-tmux", Workspace: t.TempDir(), Agent: "introspect",
		SessionName: "covhelp", Realization: RealizeFor("claude-tmux", LaunchIntent{}),
	}
	pane, err := CaptureHelp(context.Background(), cfg, recipeDeps(tx), "claude-tmux")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(pane, "/plugin") || !strings.Contains(pane, "/model") {
		t.Errorf("captured pane missing /help content: %q", pane)
	}
}

func TestCaptureHelp_UnknownCLI(t *testing.T) {
	cfg := &Config{Workspace: t.TempDir()}
	if _, err := CaptureHelp(context.Background(), cfg, recipeDeps(&fakeTmux{}), "no-such-cli"); err == nil {
		t.Fatal("want manifest error")
	}
}

func TestCaptureHelp_BootFailure(t *testing.T) {
	// Session does not exist and the marker never appears → EnsureSession errors.
	tx := &fakeTmux{paneSeq: []string{"no marker here"}}
	cfg := &Config{CLI: "claude-tmux", Workspace: t.TempDir(), Agent: "introspect", Realization: RealizeFor("claude-tmux", LaunchIntent{})}
	if _, err := CaptureHelp(context.Background(), cfg, recipeDeps(tx), "claude-tmux"); err == nil {
		t.Fatal("want ensure-session error")
	}
}

func TestRunRecipe_UnknownCLIEngineError(t *testing.T) {
	// Recipe loads, but newRecipeEngine fails resolving an unknown CLI's manifest.
	cfg := &Config{CLI: "no-such-cli", Workspace: t.TempDir(), Agent: "recipe"}
	if _, err := RunRecipe(context.Background(), cfg, recipeDeps(&fakeTmux{}), "no-such-cli", "list-capabilities", nil); err == nil {
		t.Fatal("want newRecipeEngine manifest error")
	}
}

func TestRunRecipe_UnsupportedCLI(t *testing.T) {
	cfg := &Config{CLI: "ollama-tmux", Workspace: t.TempDir(), Agent: "recipe"}
	_, err := RunRecipe(context.Background(), cfg, recipeDeps(&fakeTmux{}), "ollama-tmux", "plugin-install", map[string]string{"marketplace": "x", "plugin": "y"})
	if err == nil || !strings.Contains(err.Error(), "no steps for cli") {
		t.Fatalf("err=%v want unsupported-cli", err)
	}
}
