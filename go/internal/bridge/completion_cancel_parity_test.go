package bridge

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
)

func waitLoopTickCanceller(stderr *bytes.Buffer, n int, onFire func()) (sleep func(time.Duration), fired *bool) {
	var done bool
	ticks := 0
	return func(time.Duration) {
		if done || !strings.Contains(stderr.String(), "prompt delivered") {
			return
		}
		ticks++
		if ticks == n {
			done = true
			onFire()
		}
	}, &done
}

func stdoutParityPanes() []string {
	return []string{
		tmuxPromptMarkerDefault,
		tmuxPromptMarkerDefault,
		tmuxPromptMarkerDefault,
		"thinking…",
		"⏺ [ done ]\n" + tmuxPromptMarkerDefault,
	}
}

func TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	tmux := &ctxHonoringTmux{&fakeTmux{paneSeq: stdoutParityPanes()}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stderr bytes.Buffer
	sleep, fired := waitLoopTickCanceller(&stderr, 5, cancel)
	eng := NewEngine(Deps{Tmux: tmux, Sleep: sleep, LookupEnv: mapLookup(nil)})

	var stdout bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--completion=stdout"), nil, &stdout, &stderr)

	if !*fired {
		t.Fatal("test harness defect: the wait-loop cancellation never fired (boot marker never seen)")
	}
	if code == ExitArtifactTimeout {
		t.Fatalf("stdout contract: a settled REPL torn down at the finish line was laundered into "+
			"ExitArtifactTimeout (%d) — the final post-cancel poll cannot capture the pane on a dead "+
			"ctx, so the stdout contract is denied the grace the artifact contract gets; stderr=%q",
			code, stderr.String())
	}
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (ExitOK — the turn had settled before the cancel); stderr=%q",
			code, ExitOK, stderr.String())
	}
}

func TestTmuxREPL_StdoutContract_CancelWhileStreaming_StillTimesOut(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	streaming := []string{
		tmuxPromptMarkerDefault, tmuxPromptMarkerDefault, tmuxPromptMarkerDefault,
		"tok a", "tok ab", "tok abc", "tok abcd", "tok abcde", "tok abcdef",
	}
	tmux := &ctxHonoringTmux{&fakeTmux{paneSeq: streaming}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stderr bytes.Buffer
	sleep, fired := waitLoopTickCanceller(&stderr, 5, cancel)
	eng := NewEngine(Deps{Tmux: tmux, Sleep: sleep, LookupEnv: mapLookup(nil)})

	var stdout bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--completion=stdout"), nil, &stdout, &stderr)

	if !*fired {
		t.Fatal("test harness defect: the wait-loop cancellation never fired (boot marker never seen)")
	}
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout — the pane never settled, so the teardown "+
			"is an honest timeout and the parity fix must not manufacture completion); stderr=%q",
			code, ExitArtifactTimeout, stderr.String())
	}
}

type gitEvidenceRunner struct {
	head    func() string
	message string
	calls   int
	blocked int
}

func (g *gitEvidenceRunner) run(ctx context.Context, name, _ string, args []string, _ []string,
	_ io.Reader, stdout, _ io.Writer) (int, error) {
	if name != "git" {
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		g.blocked++
		return 1, err
	}
	g.calls++
	switch {
	case len(args) >= 3 && args[2] == "rev-parse":
		_, _ = io.WriteString(stdout, g.head())
	case len(args) >= 3 && args[2] == "rev-list":
		_, _ = io.WriteString(stdout, g.head())
	case len(args) >= 3 && args[2] == "log":
		_, _ = io.WriteString(stdout, g.message)
	}
	return 0, nil
}

func gitFixture(t *testing.T, fx launchFixture) string {
	t.Helper()
	tok := "gitevi-" + filepath.Base(fx.ws)
	if err := os.WriteFile(filepath.Join(fx.ws, "challenge-token.txt"), []byte(tok+"\n"), 0o644); err != nil {
		t.Fatalf("write challenge-token.txt: %v", err)
	}
	return tok
}

func TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	tok := gitFixture(t, fx)
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	advanced := false
	gr := &gitEvidenceRunner{
		head: func() string {
			if advanced {
				return "beefcafe11223344"
			}
			return "0000baseline0000"
		},
		message: "build: phase deliverable\n" + evidence.Trailer{
			Phase: "build", Cycle: 1236, Challenge: tok,
		}.Build(),
	}

	var stderr bytes.Buffer
	sleep, fired := waitLoopTickCanceller(&stderr, 2, func() {
		advanced = true
		cancel()
	})
	eng := NewEngine(Deps{Tmux: tmux, Sleep: sleep, LookupEnv: mapLookup(nil), Runner: gr.run})

	var stdout bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--completion=git", "--agent=build"), nil, &stdout, &stderr)

	if !*fired {
		t.Fatal("test harness defect: the wait-loop cancellation never fired (boot marker never seen)")
	}
	if gr.calls == 0 {
		t.Fatal("test harness defect: the git-evidence detector never shelled git at all — " +
			"the git completion contract was not selected")
	}
	if code == ExitArtifactTimeout {
		t.Fatalf("git contract: a verified evidence commit was laundered into ExitArtifactTimeout (%d) — "+
			"the final post-cancel poll could not fork git (%d invocation(s) refused on the dead ctx), "+
			"so the git contract is denied the grace the artifact contract gets; stderr=%q",
			code, gr.blocked, stderr.String())
	}
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (ExitOK — the evidence commit was verified before the cancel); stderr=%q",
			code, ExitOK, stderr.String())
	}
}

func TestTmuxREPL_GitContract_CancelWithoutEvidenceCommit_StillTimesOut(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	_ = gitFixture(t, fx)
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gr := &gitEvidenceRunner{
		head:    func() string { return "0000baseline0000" },
		message: "chore: unrelated commit with no Evolve-Phase trailer\n",
	}

	var stderr bytes.Buffer
	sleep, fired := waitLoopTickCanceller(&stderr, 2, cancel)
	eng := NewEngine(Deps{Tmux: tmux, Sleep: sleep, LookupEnv: mapLookup(nil), Runner: gr.run})

	var stdout bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--completion=git", "--agent=build"), nil, &stdout, &stderr)

	if !*fired {
		t.Fatal("test harness defect: the wait-loop cancellation never fired (boot marker never seen)")
	}
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout — HEAD never advanced and no trailer verified, "+
			"so the teardown is an honest timeout); stderr=%q", code, ExitArtifactTimeout, stderr.String())
	}
}
