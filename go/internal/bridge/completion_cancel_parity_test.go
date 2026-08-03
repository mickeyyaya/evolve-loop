package bridge

// completion_cancel_parity_test.go — the RED contract for cycle-1236's
// completion-contract-cancel-parity.
//
// The defect. When the wait loop's context is cancelled (orchestrator timeout,
// SIGTERM, the next phase tearing the session down) it takes ONE final
// completion poll before giving up, so a session that finished at the buzzer is
// not laundered into ExitArtifactTimeout (driver_tmux_repl.go:565-582, the
// c2258b72 fix). That final poll is handed the ALREADY-CANCELLED ctx, and the
// comment says why that was thought safe: "the artifact detector is a pure file
// stat, so the dead ctx cannot fail this last look".
//
// True for exactly ONE of the three completionDetector implementations the same
// line dispatches to (driver_tmux_repl.go:481 builds the detector from
// cfg.Completion):
//
//	artifactDetector      — os.Stat, ctx-free, plus an explicit short-circuit
//	stdoutDetector        — deps.Tmux.CapturePane(ctx, …)     (completion.go:266)
//	gitEvidenceDetector   — deps.Runner(ctx, "git", …)        (completion.go:106)
//
// exec.CommandContext REFUSES to start a process on an already-cancelled
// context, so for the latter two the final look cannot run at all: the transport
// errors, both detectors correctly swallow that as "not ready" (right policy for
// a transient mid-wait capture failure), and the finished session exits
// ExitArtifactTimeout. The benign-teardown grace exists for one contract out of
// three.
//
// The trap this contract also pins. The artifact short-circuit
// (completion.go:213) is keyed on `ctx.Err() != nil`. Handing the final poll a
// LIVE context — the obvious fix — silently switches that short-circuit off, and
// artifactDetector then demands a fresh 2-tick stability window it can never
// accrue inside one call. So the naive fix regresses the one contract that
// already works. Finality must be signalled to the detector EXPLICITLY rather
// than inferred from cancellation; AC-4 below is the guard.
//
// Test map (every case drives the REAL production wait loop via
// Engine.LaunchArgs — a detector polled in isolation proves nothing about the
// caller that starves it, and the caller IS the fault site):
//
//	AC-1 stdout parity — CancelAfterIdle_CompletesNotTimeout (+ negative)
//	AC-2 git parity    — CancelAfterEvidenceCommit_CompletesNotTimeout (+ negative)
//	AC-3 transport     — the fakes REFUSE to run on a dead ctx, so a fix that
//	                     merely re-orders code without supplying a usable context
//	                     cannot pass. This is the anti-no-op axis.
//	AC-4 artifact non-regression — the pre-existing guards
//	                     TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout and
//	                     TestArtifactDetector_CtxCancelledShortCircuitsDebounce must
//	                     still hold after the finality signal is re-keyed.

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

// waitLoopTickCanceller returns a Deps.Sleep hook that counts WAIT-LOOP ticks
// and fires at the nth one. Ticks are counted only after "prompt delivered"
// appears on stderr: the wait loop sleeps exactly once per iteration at the top
// of its body (driver_tmux_repl.go:562), immediately before the ctx.Err() check,
// so the nth call lands the cancellation precisely in the gap before poll n —
// making poll n the loop's one final post-cancel look. Counting ticks rather
// than tmux captures keeps the timing independent of how many captures boot
// happens to make.
//
// fired reports whether the injection ever ran; every test asserts it, so a
// harness that silently never cancelled can never be mistaken for a pass.
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

// --- AC-1 + AC-3: the stdout contract ---------------------------------------

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

// TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout pins AC-1.
// The REPL has gone idle on the prompt marker and the very next poll is the one
// that would complete the turn — then the orchestrator tears the session down.
// The stdout contract must get the SAME benign-teardown grace the artifact
// contract already gets: a finished advisor/router turn is a completed phase,
// not a timeout.
//
// ctxHonoringTmux (driver_tmux_repl_cancel_test.go:29) is what makes this test
// un-gameable: it reproduces exec.CommandContext's refusal to fork on a dead
// ctx, so the final capture succeeds ONLY if the fix actually hands the detector
// a usable context.
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

// TestTmuxREPL_StdoutContract_CancelWhileStreaming_StillTimesOut is AC-1's
// honest negative. The pane changes on every tick (the agent is mid-stream), so
// nothing is ready when the teardown lands. A fix that grants the final poll a
// live context must not thereby invent completion — an unfinished stdout turn
// still owes ExitArtifactTimeout. Mirrors TestTmuxREPL_CancelWithoutDeliverable_
// StillTimesOut for the artifact contract.
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

// --- AC-2 + AC-3: the git-evidence contract ---------------------------------

// gitEvidenceRunner is a Deps.Runner fake for the git-evidence contract with
// PRODUCTION ctx semantics: like exec.CommandContext, it refuses to run on an
// already-cancelled context. That refusal is the whole point — it is exactly
// what starves gitEvidenceDetector's final poll today, and no reordering of the
// detector's internals can satisfy it. Only a usable context can.
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
// construction: the challenge token it verifies commit trailers against
// (completion.go:93). Without it the detector fail-closes and can never verify,
// which would make a "still times out" assertion pass for the wrong reason.
func gitFixture(t *testing.T, fx launchFixture) string {
	t.Helper()
	tok := "gitevi-" + filepath.Base(fx.ws)
	if err := os.WriteFile(filepath.Join(fx.ws, "challenge-token.txt"), []byte(tok+"\n"), 0o644); err != nil {
		t.Fatalf("write challenge-token.txt: %v", err)
	}
	return tok
}

// TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout pins
// AC-2, the git-evidence twin of the original artifact case: the phase commits
// its deliverable and the orchestrator cancels IN THE SAME POLL GAP, so the
// final post-cancel poll is the first look that could ever observe the advance.
// Under a dead ctx `git rev-parse` cannot even start, so the verified evidence
// commit sitting in the worktree is reported as "no completion" and a delivered
// phase exits ExitArtifactTimeout.
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

// TestTmuxREPL_GitContract_CancelWithoutEvidenceCommit_StillTimesOut is AC-2's
// honest negative on TWO axes at once: HEAD never advances, AND the commit
// message carries no verifying trailer. Neither a live final context nor a
// re-keyed finality signal may turn "the phase committed nothing" into success.
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
