package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// coverage_batch2_test.go — error-path branches to drive internal/bridge
// toward 100%: profile/manifest validation, flag parsing edges, launch
// guard failures, prompt/dir I/O errors, and tmux resume/working-dir paths.

func writeJSON(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadProfile_AllBranches(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadProfile(""); err == nil {
		t.Fatal("empty path should error")
	}
	if _, err := LoadProfile(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("missing file should error")
	}
	if _, err := LoadProfile(writeJSON(t, filepath.Join(dir, "bad.json"), "{not json")); err == nil {
		t.Fatal("invalid JSON should error")
	}
	if _, err := LoadProfile(writeJSON(t, filepath.Join(dir, "noname.json"), `{"model":"haiku"}`)); err == nil {
		t.Fatal("missing name should error")
	}
	if _, err := LoadProfile(writeJSON(t, filepath.Join(dir, "badperm.json"), `{"name":"x","permission_mode":"bogus"}`)); err == nil {
		t.Fatal("bad permission_mode should error")
	}
	long := strings.Repeat("a", 33)
	if _, err := LoadProfile(writeJSON(t, filepath.Join(dir, "longsess.json"), `{"name":"x","session_name":"`+long+`"}`)); err == nil {
		t.Fatal("session_name >32 should error")
	}
	if _, err := LoadProfile(writeJSON(t, filepath.Join(dir, "badsess.json"), `{"name":"x","session_name":"bad/slash"}`)); err == nil {
		t.Fatal("session_name bad chars should error")
	}
	p, err := LoadProfile(writeJSON(t, filepath.Join(dir, "ok.json"),
		`{"name":"n","model":"haiku","stream_output":true,"session_name":"ok-1","permission_mode":"plan","allowed_tools":["Read","Write"]}`))
	if err != nil {
		t.Fatalf("valid profile err: %v", err)
	}
	if !p.StreamOutput || p.SessionName != "ok-1" || p.PermissionMode != "plan" || len(p.AllowedTools) != 2 {
		t.Fatalf("valid profile fields wrong: %+v", p)
	}
}

func TestLoadManifest_AndParse_Errors(t *testing.T) {
	if _, err := LoadManifest(""); err == nil {
		t.Fatal("empty cli should error")
	}
	if _, err := LoadManifest("definitely-not-a-cli"); err == nil {
		t.Fatal("missing manifest should error")
	}
	if _, err := parseManifest("x", []byte("{bad json")); err == nil {
		t.Fatal("bad JSON should error")
	}
	if _, err := parseManifest("x", []byte(`{"cli":"x"}`)); err == nil {
		t.Fatal("missing binary should error")
	}
	if m, err := parseManifest("x", []byte(`{"cli":"x","binary":"b"}`)); err != nil || m.Binary != "b" {
		t.Fatalf("valid parse: m=%+v err=%v", m, err)
	}
}

func TestParseLaunchArgs_Branches(t *testing.T) {
	// space form
	r, err := parseLaunchArgs([]string{"--cli", "claude-p", "--model", "haiku", "--agent", "scout"}, nil)
	if err != nil || r.cli != "claude-p" || r.model != "haiku" || r.agent != "scout" {
		t.Fatalf("space form: %+v err=%v", r, err)
	}
	// = form, bool flags, passthrough
	r, err = parseLaunchArgs([]string{"--cli=codex", "--validate-only", "--dry-run", "--require-full",
		"--allow-bypass", "--human-input", "--stream-output", "--", "--bare", "--x"}, nil)
	if err != nil || !r.validateOnly || !r.dryRun || !r.requireFull || !r.allowBypass || !r.humanInput || r.streamOutput != "true" {
		t.Fatalf("bool/passthrough: %+v err=%v", r, err)
	}
	if len(r.extra) != 2 {
		t.Fatalf("extra = %v, want 2", r.extra)
	}
	if r, _ := parseLaunchArgs([]string{"--no-stream-output"}, nil); r.streamOutput != "false" {
		t.Fatalf("--no-stream-output → %q", r.streamOutput)
	}
	for _, bad := range [][]string{{"--bogus"}, {"--bogus=v"}, {"--cli"}, {"positional"}} {
		if _, err := parseLaunchArgs(bad, nil); err == nil {
			t.Fatalf("args %v should error", bad)
		}
	}
	// env fallbacks
	r, _ = parseLaunchArgs(nil, map[string]string{
		"BRIDGE_CLI": "agy", "PROFILE_PATH": "/p", "RESOLVED_MODEL": "opus",
		"BRIDGE_REQUIRE_FULL": "1", "BRIDGE_ALLOW_BYPASS": "1", "VALIDATE_ONLY": "1",
	})
	if r.cli != "agy" || r.profile != "/p" || r.model != "opus" || !r.requireFull || !r.allowBypass || !r.validateOnly {
		t.Fatalf("env fallbacks: %+v", r)
	}
}

func TestLaunchArgs_ErrorBranches(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	// unknown flag → parse error
	if code, se := runLookup(t, &fakeRunner{}, fx.args("claude-p", "--bogus"), nil); code != ExitBadFlags || !strings.Contains(se, "unknown flag") {
		t.Fatalf("unknown flag: code=%d se=%q", code, se)
	}
	// invalid permission-mode
	if code, se := runLookup(t, &fakeRunner{}, fx.args("claude-p", "--permission-mode=bogus"), nil); code != ExitBadFlags || !strings.Contains(se, "invalid --permission-mode") {
		t.Fatalf("bad perm-mode: code=%d se=%q", code, se)
	}
	// bad session-name
	if code, se := runLookup(t, &fakeRunner{}, fx.args("claude-p", "--session-name=bad/slash"), nil); code != ExitBadFlags || !strings.Contains(se, "invalid --session-name") {
		t.Fatalf("bad session-name: code=%d se=%q", code, se)
	}
	// prompt not readable
	badPrompt := fx.args("claude-p")
	for i, a := range badPrompt {
		if strings.HasPrefix(a, "--prompt-file=") {
			badPrompt[i] = "--prompt-file=/no/such/prompt.txt"
		}
	}
	if code, se := runLookup(t, &fakeRunner{}, badPrompt, nil); code != ExitBadFlags || !strings.Contains(se, "not readable") {
		t.Fatalf("unreadable prompt: code=%d se=%q", code, se)
	}
	// profile load fail
	badProf := fx.args("claude-p")
	for i, a := range badProf {
		if strings.HasPrefix(a, "--profile=") {
			badProf[i] = "--profile=/no/such/profile.json"
		}
	}
	if code := mustExit(t, badProf, nil); code != ExitBadFlags {
		t.Fatalf("bad profile: code=%d", code)
	}
}

func TestLaunchArgs_InvalidStreamOutputEnv(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	eng := NewEngine(Deps{Runner: (&fakeRunner{}).runner(), LookupEnv: mapLookup(nil)})
	var so, se strings.Builder
	code := eng.LaunchArgs(context.Background(), fx.args("claude-p"), map[string]string{"BRIDGE_STREAM_OUTPUT": "bogus"}, &so, &se)
	if code != ExitBadFlags || !strings.Contains(se.String(), "invalid --stream-output") {
		t.Fatalf("bad stream-output env: code=%d se=%q", code, se.String())
	}
}

func mustExit(t *testing.T, args []string, lookup map[string]string) int {
	t.Helper()
	c, _ := runLookup(t, &fakeRunner{}, args, lookup)
	return c
}

func TestPreparePrompt_Branches(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "token=$CHALLENGE_TOKEN art=$ARTIFACT_PATH")
	cfg := &Config{PromptFile: pf, Workspace: ws, Artifact: filepath.Join(ws, "a.md")}
	deps := Deps{NewChallengeToken: func() (string, error) { return "TOK99", nil }}
	out, err := preparePrompt(cfg, deps)
	if err != nil {
		t.Fatalf("preparePrompt err: %v", err)
	}
	if !strings.Contains(out, "token=TOK99") || !strings.Contains(out, "art="+cfg.Artifact) {
		t.Fatalf("substitution wrong: %q", out)
	}
	if b, _ := os.ReadFile(filepath.Join(ws, "challenge-token.txt")); !strings.Contains(string(b), "TOK99") {
		t.Fatal("challenge-token.txt should hold the minted token")
	}
	// read error: prompt file is a directory
	if _, err := preparePrompt(&Config{PromptFile: ws, Workspace: ws}, deps); err == nil {
		t.Fatal("reading a directory as prompt should error")
	}
}

func TestEnsureDirsAndOpenLogs_Error(t *testing.T) {
	f := filepath.Join(t.TempDir(), "afile")
	writeJSON(t, f, "x")
	cfg := &Config{Workspace: f} // Workspace is a regular file → MkdirAll fails
	if err := ensureDirs(cfg); err == nil {
		t.Fatal("ensureDirs should fail when Workspace is a file")
	}
	if _, _, _, err := openDriverLogs(cfg); err == nil {
		t.Fatal("openDriverLogs should fail when dirs can't be made")
	}
}

func TestEngineLaunch_ValidationAndMkdir(t *testing.T) {
	eng := NewEngine(Deps{})
	reqs := []core.BridgeRequest{
		{Profile: "p", Workspace: "w", ArtifactPath: "a"},
		{CLI: "c", Workspace: "w", ArtifactPath: "a"},
		{CLI: "c", Profile: "p", ArtifactPath: "a"},
		{CLI: "c", Profile: "p", Workspace: "w"},
	}
	for i, r := range reqs {
		if _, err := eng.Launch(context.Background(), r); err == nil {
			t.Fatalf("req[%d] missing field should error", i)
		}
	}
	// mkdir failure: Workspace is a file
	f := filepath.Join(t.TempDir(), "f")
	writeJSON(t, f, "x")
	if _, err := eng.Launch(context.Background(), core.BridgeRequest{CLI: "claude-p", Profile: "p", Workspace: f, ArtifactPath: "a"}); err == nil {
		t.Fatal("Launch should fail when workspace dir can't be created")
	}
}

func TestRunTmuxREPL_WorkingDirMissing(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	code, se := runTmux(t, fx, &fakeTmux{}, nil, "--allow-bypass", "--worktree=/no/such/dir-xyz")
	if code != ExitBadFlags || !strings.Contains(se, "working dir does not exist") {
		t.Fatalf("missing workdir: code=%d se=%q", code, se)
	}
}

func TestRunTmuxREPL_NamedSessionResume(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	writeJSON(t, fx.artifact, "done") // artifact already present
	tmux := &fakeTmux{existing: map[string]bool{"evolve-bridge-named-mysess": true}}
	code, se := runTmux(t, fx, tmux, nil, "--allow-bypass", "--session-name=mysess")
	if code != ExitOK {
		t.Fatalf("named resume exit = %d, want ExitOK; se=%q", code, se)
	}
	if tmux.sentContains("/exit") {
		t.Fatal("named session must be preserved (no /exit)")
	}
	if !strings.Contains(se, "RESUME") {
		t.Fatalf("should log RESUME; se=%q", se)
	}
}

func TestCodexDriver_UnrecognizedModelOmitsM(t *testing.T) {
	fx := newFixture(t, "codex", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	// --model=weird → not a codex model name → omit -m
	code, _ := runLookup(t, fr, codexArgs(fx, "weird"), nil)
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}
	if fr.argvContainsPair("-m", "weird") {
		t.Fatal("unrecognized model should NOT be passed via -m")
	}
}

func TestClaudeP_StreamOutputAndSessionNote(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	code, se := runLookup(t, fr, fx.args("claude-p", "--stream-output", "--session-name=foo"), nil)
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}
	joined := strings.Join(fr.calls[0].args, " ")
	if !strings.Contains(joined, "stream-json") {
		t.Fatalf("--stream-output should add stream-json flags; args=%v", fr.calls[0].args)
	}
	if !strings.Contains(se, "session-name") {
		t.Fatalf("claude-p should note session-name is no-op; se=%q", se)
	}
}

func TestDryRun_ChallengeToken(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	writeJSON(t, fx.promptFile, "use $CHALLENGE_TOKEN here")
	code, _ := runLookup(t, &fakeRunner{}, fx.args("claude-p", "--dry-run"), nil)
	if code != ExitOK {
		t.Fatalf("dry-run exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(fx.ws, "challenge-token.txt")); err != nil {
		t.Fatalf("dry-run should mint a challenge token: %v", err)
	}
}

func TestRequireFull_ManifestMissing(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	args := []string{
		"--cli=no-such-cli", "--profile=" + fx.profile, "--model=auto",
		"--prompt-file=" + fx.promptFile, "--workspace=" + fx.ws,
		"--stdout-log=" + fx.stdoutLog, "--stderr-log=" + fx.stderrLog,
		"--artifact=" + fx.artifact, "--require-full",
	}
	eng := NewEngine(Deps{LookupEnv: mapLookup(nil), LookPath: func(string) (string, error) { return "", errNoBin }})
	var so, se strings.Builder
	code := eng.LaunchArgs(context.Background(), args, nil, &so, &se)
	if code != ExitRequireFullUnmet {
		t.Fatalf("require-full + no manifest → code=%d, want %d", code, ExitRequireFullUnmet)
	}
}
