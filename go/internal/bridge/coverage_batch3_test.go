package bridge

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// coverage_batch3_test.go — error-injection + note branches: driver
// runner failures, tmux spawn failure, the auto-responder tick layer,
// extra cost guards, and the per-driver no-op notes.

func TestDrivers_RunnerError_MissingBinary(t *testing.T) {
	for _, cli := range []string{"claude-p", "codex", "agy"} {
		t.Run(cli, func(t *testing.T) {
			fx := newFixture(t, cli, "")
			fr := &fakeRunner{err: errors.New("exec boom")}
			code, _ := runLookup(t, fr, fx.args(cli), nil)
			if code != ExitMissingBinary {
				t.Fatalf("%s runner error → code=%d, want ExitMissingBinary", cli, code)
			}
		})
	}
}

func TestRunTmuxREPL_NewSessionError(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{newSessErr: errors.New("tmux down")}
	code, se := runTmux(t, fx, tmux, nil, "--allow-bypass")
	if code != ExitBadFlags || !strings.Contains(se, "new-session") {
		t.Fatalf("new-session error → code=%d se=%q", code, se)
	}
}

func TestClaudeTmux_CostGuards_BaseURL(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// ANTHROPIC_BASE_URL without allow → cost leak
	if code, se := runTmux(t, fx, &fakeTmux{}, map[string]string{"ANTHROPIC_BASE_URL": "http://x"}, "--allow-bypass"); code != ExitCostLeak || !strings.Contains(se, "ANTHROPIC_BASE_URL") {
		t.Fatalf("base-url guard: code=%d se=%q", code, se)
	}
	// policy bridge.anthropic_base_url (via --anthropic-base-url flag) → cost leak
	if code, se := runTmux(t, fx, &fakeTmux{}, nil, "--allow-bypass", "--anthropic-base-url=http://p"); code != ExitCostLeak || !strings.Contains(se, "anthropic_base_url") {
		t.Fatalf("evolve-base-url guard: code=%d se=%q", code, se)
	}
}

func TestCodex_Notes(t *testing.T) {
	fx := newFixture(t, "codex", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	_, se := runLookup(t, fr, fx.args("codex", "--stream-output", "--session-name=foo"), nil)
	if !strings.Contains(se, "stream_output") || !strings.Contains(se, "session-name") {
		t.Fatalf("codex notes missing; se=%q", se)
	}
}

func TestAgy_NotesAndModelWarn(t *testing.T) {
	fx := newFixture(t, "agy", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	// non-tier model → WARN; + stream/session notes
	args := []string{
		"--cli=agy", "--profile=" + fx.profile, "--model=some-weird-model",
		"--prompt-file=" + fx.promptFile, "--workspace=" + fx.ws,
		"--stdout-log=" + fx.stdoutLog, "--stderr-log=" + fx.stderrLog,
		"--artifact=" + fx.artifact, "--stream-output", "--session-name=foo",
	}
	code, se := runLookup(t, fr, args, nil)
	if code != ExitOK {
		t.Fatalf("agy exit = %d", code)
	}
	if !strings.Contains(se, "WARN") || !strings.Contains(se, "stream_output") || !strings.Contains(se, "session-name") {
		t.Fatalf("agy notes/warn missing; se=%q", se)
	}
}

func TestCodexTmux_StreamOutputNote(t *testing.T) {
	fx := newFixture(t, "codex-tmux", "")
	_, se := runTmuxCLI(t, fx, "codex-tmux", &fakeTmux{}, nil, "--allow-bypass", "--stream-output")
	if !strings.Contains(se, "stream_output=true is not supported") {
		t.Fatalf("codex-tmux should note stream_output no-op; se=%q", se)
	}
}

func TestAutoResponder_Tick(t *testing.T) {
	mkAR := func(prompts []ManifestPrompt, tmux *fakeTmux) *autoResponder {
		// no-op Sleep: the auto_respond path now paces multi-keystroke sends
		// via deps.Sleep (production sets it via withDefaults).
		deps := Deps{Tmux: tmux, Stderr: io.Discard, Sleep: func(time.Duration) {}}
		return &autoResponder{prompts: prompts, workspace: t.TempDir(), cli: "x", counts: map[string]int{}, deps: deps}
	}
	// extend_timeout → ("extend:N", 2)
	ar := mkAR([]ManifestPrompt{{Name: "e", Regex: "slow", ResponseKeys: "30", Policy: "extend_timeout"}}, &fakeTmux{paneSeq: []string{"slow op"}})
	if a, rc := ar.tick(context.Background(), "s"); a != "extend:30" || rc != 2 {
		t.Fatalf("tick extend = (%q,%d)", a, rc)
	}
	// auto_respond send → ("",1) + keys sent
	tmux := &fakeTmux{paneSeq: []string{"Continue?"}}
	ar = mkAR([]ManifestPrompt{{Name: "s", Regex: "Continue", ResponseKeys: "y,Enter", Policy: "auto_respond"}}, tmux)
	if a, rc := ar.tick(context.Background(), "s"); a != "" || rc != 1 {
		t.Fatalf("tick send = (%q,%d)", a, rc)
	}
	if !tmux.sentContains("y") {
		t.Fatalf("tick send should deliver keys; sent=%v", tmux.sentKeys)
	}
	// noop
	ar = mkAR(nil, &fakeTmux{paneSeq: []string{"nothing"}})
	if a, rc := ar.tick(context.Background(), "s"); a != "" || rc != 0 {
		t.Fatalf("tick noop = (%q,%d)", a, rc)
	}
}

func TestDecideAutoRespond_InvalidRegexSkipped(t *testing.T) {
	prompts := []ManifestPrompt{
		{Name: "bad", Regex: "[unclosed", Policy: "escalate"}, // invalid regex → skipped
		{Name: "ok", Regex: "match", Policy: "escalate"},
	}
	if a, rc := decideAutoRespond("match here", prompts, map[string]int{}, false); a != "escalate:ok" || rc != 85 {
		t.Fatalf("invalid-regex skip = (%q,%d), want escalate:ok,85", a, rc)
	}
}

func TestRunDryRun_EnsureDirsError(t *testing.T) {
	f := writeJSON(t, t.TempDir()+"/file", "x") // a regular file used as Workspace
	eng := NewEngine(Deps{})
	cfg := &Config{Workspace: f, StdoutLog: f + "/s", StderrLog: f + "/e", Artifact: f + "/a", PromptFile: f}
	if code := eng.runDryRun(cfg, io.Discard, io.Discard); code != ExitBadFlags {
		t.Fatalf("runDryRun ensureDirs error → code=%d, want ExitBadFlags", code)
	}
}
