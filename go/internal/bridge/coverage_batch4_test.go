package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// coverage_batch4_test.go — remaining reachable branches: driver log-file
// open errors, engine.Launch argv branches, named-session create path,
// and profile/report edge cases.

func TestDrivers_OpenLogsError(t *testing.T) {
	// --stdout-log points at the workspace dir itself → os.Create fails.
	for _, cli := range []string{"claude-p", "codex", "agy"} {
		t.Run(cli, func(t *testing.T) {
			fx := newFixture(t, cli, "")
			args := []string{
				"--cli=" + cli, "--profile=" + fx.profile, "--model=auto",
				"--prompt-file=" + fx.promptFile, "--workspace=" + fx.ws,
				"--stdout-log=" + fx.ws, // a directory → Create fails
				"--stderr-log=" + fx.stderrLog, "--artifact=" + fx.artifact,
			}
			if code, _ := runLookup(t, &fakeRunner{}, args, nil); code != ExitBadFlags {
				t.Fatalf("%s open-logs error → code=%d, want ExitBadFlags", cli, code)
			}
		})
	}
}

func TestEngineLaunch_ArgBranchesAndMissingArtifact(t *testing.T) {
	ws := t.TempDir()
	prof := writeJSON(t, filepath.Join(ws, "p.json"), `{"name":"n","model":"haiku"}`)
	art := filepath.Join(ws, "art.md")
	wt := t.TempDir()
	// fake runner does NOT write the artifact → success but empty Stdout.
	fr := &fakeRunner{}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})
	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: art,
		// Agent="" → default "agent"; Cycle>0 + Worktree exercise those argv branches.
		Cycle: 5, Worktree: wt,
	})
	if err != nil {
		t.Fatalf("Launch err: %v", err)
	}
	if resp.Stdout != "" {
		t.Fatalf("missing artifact on success → Stdout should be empty, got %q", resp.Stdout)
	}
	if _, e := os.Stat(filepath.Join(ws, "agent-prompt.txt")); e != nil {
		t.Fatalf("default agent prompt file should be written: %v", e)
	}
}

func TestRunTmuxREPL_NamedSessionCreate(t *testing.T) {
	// --session-name that does NOT exist → CREATE-NAMED path; session
	// preserved on exit (no /exit), artifact already present.
	fx := newFixture(t, "claude-tmux", "")
	writeJSON(t, fx.artifact, "done")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots (new session)
	code, se := runTmux(t, fx, tmux, nil, "--allow-bypass", "--session-name=brandnew")
	if code != ExitOK {
		t.Fatalf("named-create exit = %d, want ExitOK; se=%q", code, se)
	}
	if !strings.Contains(se, "CREATE-NAMED") {
		t.Fatalf("should log CREATE-NAMED; se=%q", se)
	}
	if tmux.sentContains("/exit") {
		t.Fatal("named session must be preserved (no /exit)")
	}
}

func TestLoadProfile_ReadDirError(t *testing.T) {
	// Passing a directory path → os.ReadFile returns a non-ENOENT error.
	if _, err := LoadProfile(t.TempDir()); err == nil {
		t.Fatal("reading a directory as a profile should error")
	}
}

func TestBuildReport_ArtifactIsDirectory(t *testing.T) {
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, "artifact.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := BuildReport(ws, "artifact.md", time.Now())
	if err != nil {
		t.Fatalf("BuildReport err: %v", err)
	}
	if r.Artifact.Exists {
		t.Fatal("a directory must not count as an existing artifact")
	}
	if r.Verdict != "incomplete" {
		t.Fatalf("verdict = %q, want incomplete", r.Verdict)
	}
}
