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

// waitLoopTickCanceller returns a Deps.Sleep hook that fires at the nth
// wait-loop tick after "prompt delivered" appears on stderr, landing the
// cancellation precisely in the gap before poll n — making poll n the loop's
// one final post-cancel look. Counting ticks rather than tmux captures keeps
// the timing independent of how many captures boot happens to make.
//
// fired reports whether the injection ever ran, so a harness that silently
// never cancelled can never be mistaken for a pass.
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

// stdoutParityPanes is the pane script from TestClaudeTmux_StdoutCompletion_
// NoArtifactNeeded, reused verbatim so the detector's state machine is driven
// through a sequence already PROVEN to complete under a live context. Detector
// polls see: "thinking…" (baseline), then the settled marker pane repeating —
// stable reaches stdoutIdlePolls on the 5th wait-loop tick.
func stdoutParityPanes() []string {
	return []string{
		tmuxPromptMarkerDefault,                  // boot loop capture: REPL ready
		tmuxPromptMarkerDefault,                  // boot-time auto-respond tick
		tmuxPromptMarkerDefault,                  // interval baseline pre-capture
		"thinking…",                              // detector poll 1 — baseline
		"⏺ [ done ]\n" + tmuxPromptMarkerDefault, // settles; repeats → idle accrues
	}
}

// ctxHonoringTmux reproduces exec.CommandContext's refusal to fork on a dead
// ctx, so this test's final capture can only succeed if the fix hands the
// detector a genuinely usable context.
func TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &ctxHonoringTmux{&fakeTmux{paneSeq: stdoutParityPanes()}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stderr bytes.Buffer
	// Tick 5 is the poll at which stable reaches stdoutIdlePolls: poll 1
	// baselines on "thinking…", poll 2 first sees the settled pane (stable
	// resets), polls 3 and 4 accrue, poll 5 crosses the threshold. Cancelling in
	// the gap before it makes that completing poll the loop's final look.
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

// The pane changes on every wait-loop tick to model an agent mid-stream, so
// nothing is ready when the teardown lands; mirrors
// TestTmuxREPL_CancelWithoutDeliverable_StillTimesOut for the artifact
// contract.
func TestTmuxREPL_StdoutContract_CancelWhileStreaming_StillTimesOut(t *testing.T) {
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

// gitEvidenceRunner is a Deps.Runner fake for the git-evidence contract with
// PRODUCTION ctx semantics: like exec.CommandContext, it refuses to run on an
// already-cancelled context. That refusal is exactly what starves
// gitEvidenceDetector's final poll, and no reordering of the detector's
// internals can satisfy it. Only a usable context can.
//
// head is read through a closure so a test can advance HEAD in the same gap it
// cancels the context, modelling "the phase committed its evidence and the
// orchestrator tore the session down between two polls".
type gitEvidenceRunner struct {
	head    func() string
	message string
	calls   int
	blocked int // invocations refused because the ctx was already dead
}

func (g *gitEvidenceRunner) run(ctx context.Context, name, _ string, args []string, _ []string,
	_ io.Reader, stdout, _ io.Writer) (int, error) {
	if name != "git" {
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		// exec.CommandContext never forks a process on a cancelled context.
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

// gitFixture prepares the workspace state gitEvidenceDetector reads at
// construction: the challenge token it verifies commit trailers against.
// Without it the detector fail-closes and can never verify, which would make
// a "still times out" assertion pass for the wrong reason.
func gitFixture(t *testing.T, fx launchFixture) string {
	t.Helper()
	tok := "gitevi-" + filepath.Base(fx.ws)
	if err := os.WriteFile(filepath.Join(fx.ws, "challenge-token.txt"), []byte(tok+"\n"), 0o644); err != nil {
		t.Fatalf("write challenge-token.txt: %v", err)
	}
	return tok
}

// The phase commits its deliverable and the context is cancelled in the same
// poll gap, so the final post-cancel poll is the first look that could ever
// observe the advance.
func TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout(t *testing.T) {
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
	// Tick 2: poll 1 established the HEAD baseline, then the evidence commit
	// lands and the teardown arrives together — the classic benign teardown.
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

// The negative on two axes at once: HEAD never advances, and the commit
// message carries no verifying trailer.
func TestTmuxREPL_GitContract_CancelWithoutEvidenceCommit_StillTimesOut(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	_ = gitFixture(t, fx)
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gr := &gitEvidenceRunner{
		head:    func() string { return "0000baseline0000" }, // never advances
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
