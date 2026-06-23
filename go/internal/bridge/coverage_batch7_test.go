package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// coverage_batch7_test.go — the final uncovered branches: driver note +
// model-omit paths, preparePrompt/ensureDirs/write errors (direct), the
// runTmuxREPL extend-timeout + zero-interval paths (manifest seam), the
// LaunchArgs (0,err) driver path (registry seam), and report/stream edges.

func TestClaudeTmux_StreamOutputNote(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	_, se := runTmux(t, fx, &fakeTmux{}, nil, "--allow-bypass", "--stream-output")
	if !strings.Contains(se, "stream_output=true is no-op") {
		t.Fatalf("claude-tmux should note stream_output no-op; se=%q", se)
	}
}

func TestCodex_AutoModelOmitsM(t *testing.T) {
	fx := newFixture(t, "codex", "")
	prof := writeJSON(t, filepath.Join(t.TempDir(), "nomodel.json"), `{"name":"n"}`) // no model
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	args := []string{
		"--cli=codex", "--profile=" + prof, "--model=auto",
		"--prompt-file=" + fx.promptFile, "--workspace=" + fx.ws,
		"--stdout-log=" + fx.stdoutLog, "--stderr-log=" + fx.stderrLog, "--artifact=" + fx.artifact,
	}
	code, se := runLookup(t, fr, args, nil)
	if code != ExitOK {
		t.Fatalf("codex auto-model exit = %d", code)
	}
	for _, a := range fr.calls[0].args {
		if a == "-m" {
			t.Fatalf("auto model must omit the -m flag; args=%v", fr.calls[0].args)
		}
	}
	if !strings.Contains(se, "omitting -m") {
		t.Fatalf("should log omitting -m; se=%q", se)
	}
}

// TestPreparePrompt_ReadsExistingChallengeToken (cycle-136 lesson, PR 7):
// when workspace/challenge-token.txt already exists (orchestrator minted +
// wrote it at cycle start per PR 6), preparePrompt MUST reuse the existing
// value and NOT mint+overwrite. One token per cycle is the invariant; the
// bridge's per-phase mint was overwriting the orchestrator's token, causing
// scout-report.md to use the orchestrator's token (plumbed via Context per
// PR 6 / scout.go:64) while later phases saw the bridge's new token in the
// workspace file. Cycle 136 audit C1 surfaced the divergence.
func TestPreparePrompt_ReadsExistingChallengeToken(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "prompt body $CHALLENGE_TOKEN tail")
	// Pre-seed challenge-token.txt as if the orchestrator already minted it.
	existing := "orchestrator-minted-12345678"
	if err := os.WriteFile(filepath.Join(ws, "challenge-token.txt"), []byte(existing+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Mint would return a DIFFERENT value — proves the existing-token path
	// is taken when it returns the orchestrator's value, not the mint's.
	mintCalls := 0
	out, err := preparePrompt(&Config{PromptFile: pf, Workspace: ws}, Deps{
		NewChallengeToken: func() (string, error) {
			mintCalls++
			return "fresh-mint-should-not-be-used", nil
		},
	})
	if err != nil {
		t.Fatalf("preparePrompt: %v", err)
	}
	if !strings.Contains(out, existing) {
		t.Errorf("prompt must substitute existing token %q; got %q", existing, out)
	}
	if strings.Contains(out, "fresh-mint-should-not-be-used") {
		t.Errorf("prompt must NOT substitute fresh-mint when existing-token is on disk; got %q", out)
	}
	if mintCalls != 0 {
		t.Errorf("NewChallengeToken called %d times; should be 0 when challenge-token.txt exists", mintCalls)
	}
	// File must still contain the orchestrator's value (no overwrite).
	if b, _ := os.ReadFile(filepath.Join(ws, "challenge-token.txt")); strings.TrimSpace(string(b)) != existing {
		t.Errorf("challenge-token.txt was overwritten; got %q want %q", strings.TrimSpace(string(b)), existing)
	}
}

func TestPreparePrompt_TokenErrors(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "x $CHALLENGE_TOKEN")
	// NewChallengeToken error
	if _, err := preparePrompt(&Config{PromptFile: pf, Workspace: ws},
		Deps{NewChallengeToken: func() (string, error) { return "", errors.New("no tok") }}); err == nil {
		t.Fatal("token-mint error should propagate")
	}
	// challenge-token.txt write error (Workspace is a file)
	fileWS := writeJSON(t, filepath.Join(ws, "notadir"), "x")
	if _, err := preparePrompt(&Config{PromptFile: pf, Workspace: fileWS},
		Deps{NewChallengeToken: func() (string, error) { return "tok", nil }}); err == nil {
		t.Fatal("challenge-token write error should propagate")
	}
}

func TestRunTmuxREPL_EnsureDirsAndPromptWriteErrors(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hello") // valid prompt, no token

	// ensureDirs error: Workspace is a file
	fileWS := writeJSON(t, filepath.Join(ws, "wsfile"), "x")
	cfg := &Config{Model: "m", AllowBypass: true, PromptFile: pf, Workspace: fileWS,
		Artifact: filepath.Join(ws, "a"), StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	if code, _ := (claudeTmuxDriver{}).Launch(context.Background(), cfg, covDeps()); code != ExitBadFlags {
		t.Fatalf("ensureDirs error → code=%d, want ExitBadFlags", code)
	}

	// resolved-prompt write error: workspace/resolved-prompt.txt is a directory
	ws2 := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws2, "resolved-prompt.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg2 := &Config{Model: "m", AllowBypass: true, PromptFile: pf, Workspace: ws2,
		Artifact: filepath.Join(ws2, "a"), StdoutLog: filepath.Join(ws2, "o"), StderrLog: filepath.Join(ws2, "e")}
	if code, _ := (claudeTmuxDriver{}).Launch(context.Background(), cfg2, covDeps()); code != ExitBadFlags {
		t.Fatalf("resolved-prompt write error → code=%d, want ExitBadFlags", code)
	}
}

func TestEngineLaunch_PromptWriteError(t *testing.T) {
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, "scout-prompt.txt"), 0o755); err != nil { // promptFile path is a dir
		t.Fatal(err)
	}
	prof := writeJSON(t, filepath.Join(ws, "prof.json"), `{"name":"n"}`)
	_, err := NewEngine(Deps{LookupEnv: mapLookup(nil)}).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a"), Agent: "scout",
	})
	if err == nil {
		t.Fatal("prompt-file write error should propagate from Engine.Launch")
	}
}

func TestBuildReport_WorkspaceIsFile(t *testing.T) {
	f := writeJSON(t, filepath.Join(t.TempDir(), "f"), "x")
	if _, err := BuildReport(f, "artifact.md", time.Now()); err == nil {
		t.Fatal("a file workspace should error")
	}
}

func TestLaunchArgs_NoStreamOutput(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	if code, _ := runLookup(t, fr, fx.args("claude-p", "--no-stream-output"), nil); code != ExitOK {
		t.Fatalf("--no-stream-output exit = %d, want ExitOK", code)
	}
}

func TestRunTmuxREPL_ZeroBootIntervalDefaults(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	writeJSON(t, filepath.Join(ws, "a"), "done") // artifact present → quick exit
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws,
		Artifact: filepath.Join(ws, "a"), StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	deps := covDeps()
	deps.Tmux = &fakeTmux{paneSeq: []string{"❯"}}
	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 0}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("zero boot-interval should default to 1 and boot; code=%d", code)
	}
}

func TestRunTmuxREPL_ExtendTimeout(t *testing.T) {
	// Inject a manifest with an extend_timeout rule via the FS seam; a pane
	// that always matches it drives the extend path until the loop guard.
	swapManifestFS(t, fakeManifestFS{files: map[string][]byte{
		"manifests/extendcli.json": []byte(`{"cli":"extendcli","binary":"x","interactive_prompts":[{"name":"w","regex":"WAITING","response_keys":"30","policy":"extend_timeout"}]}`),
	}})
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi") // no artifact → keeps polling
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws,
		Artifact: filepath.Join(ws, "a"), StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	deps := covDeps()
	deps.Tmux = &fakeTmux{paneSeq: []string{"❯ WAITING"}} // boots (❯) + matches WAITING
	lp := tmuxLaunch{name: "extendcli", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitRespondLoopGuard {
		t.Fatalf("repeated extend_timeout should trip the loop guard; code=%d", code)
	}
}

func TestRunTmuxREPL_ExtraFlagsAppended(t *testing.T) {
	// `-- --myflag` → ExtraFlags appended to the REPL launch command.
	fx := newFixture(t, "claude-tmux", "")
	writeJSON(t, fx.artifact, "done")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	code, _ := runTmux(t, fx, tmux, nil, "--allow-bypass", "--", "--myflag")
	if code != ExitOK {
		t.Fatalf("extra-flags launch exit = %d, want ExitOK", code)
	}
	if !tmux.sentContains("--myflag") {
		t.Fatalf("ExtraFlags should be appended to the launch command; sent=%v", tmux.sentKeys)
	}
}

type zeroErrDriver struct{}

func (zeroErrDriver) Name() string { return "zeroerr" }
func (zeroErrDriver) Launch(context.Context, *Config, Deps) (int, error) {
	return 0, errors.New("zero-with-error")
}

func TestEngineLaunch_EmptyModelDefaultsAuto(t *testing.T) {
	ws := t.TempDir()
	prof := writeJSON(t, filepath.Join(ws, "p.json"), `{"name":"n","model":"haiku"}`)
	art := filepath.Join(ws, "a.md")
	fr := &fakeRunner{writeArtifactPath: art, writeArtifactBody: "ok"}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})
	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "", Prompt: "x", Workspace: ws, ArtifactPath: art,
	}); err != nil {
		t.Fatalf("empty model should default to auto; err=%v", err)
	}
}

func TestBuildReport_EmptyArtifactNameDefaults(t *testing.T) {
	ws := t.TempDir()
	writeJSON(t, filepath.Join(ws, "artifact.md"), "body")
	r, err := BuildReport(ws, "", time.Now()) // "" → defaults to artifact.md
	if err != nil {
		t.Fatalf("BuildReport err: %v", err)
	}
	if !r.Artifact.Exists {
		t.Fatal("empty artifactName should default to artifact.md (which exists here)")
	}
}

func TestRunTmuxREPL_TickDuringBoot(t *testing.T) {
	// tickDuringBoot + a first pane WITHOUT the marker → the boot loop runs
	// the auto-responder tick before the marker appears on the next poll.
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	writeJSON(t, filepath.Join(ws, "a"), "done") // artifact present → quick exit after boot
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws,
		Artifact: filepath.Join(ws, "a"), StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	deps := covDeps()
	deps.Tmux = &fakeTmux{paneSeq: []string{"booting...", "❯"}} // marker only on 2nd poll
	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1, tickDuringBoot: true}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("tick-during-boot path → code=%d, want ExitOK", code)
	}
}

func TestLaunchArgs_DriverReturnsZeroWithError(t *testing.T) {
	ResetDriversForTesting()
	defer func() { ResetDriversForTesting(); registerBuiltins() }()
	Register(zeroErrDriver{})

	fx := newFixture(t, "x", "")
	args := []string{
		"--cli=zeroerr", "--profile=" + fx.profile, "--model=haiku",
		"--prompt-file=" + fx.promptFile, "--workspace=" + fx.ws,
		"--stdout-log=" + fx.stdoutLog, "--stderr-log=" + fx.stderrLog, "--artifact=" + fx.artifact,
	}
	if code, _ := runLookup(t, &fakeRunner{}, args, nil); code != ExitBadFlags {
		t.Fatalf("driver returning (0,err) → LaunchArgs code=%d, want ExitBadFlags", code)
	}
}
