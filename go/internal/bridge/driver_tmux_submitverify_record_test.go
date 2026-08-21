package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// driver_tmux_submitverify_record_test.go — the guard added in #474 announces
// itself on stderr only, and Engine.Launch returns on the SUCCESS path
// (engine.go:531-534) BEFORE it persists stderr to <agent>-launch-error.txt
// (:544). So a submit-verify that DETECTS a parked prompt and RECOVERS it makes
// the phase succeed, which discards every line proving it fired. Four waves and
// eight cycles produced zero observations of the guard working.
//
// interactions.ndjson is the durable surface: interaction.Recorder.Record writes
// through to disk immediately (appendLedgerLine) regardless of phase outcome —
// cycle-1530's router-interactions.ndjson carries an auto_respond record from a
// phase that SUCCEEDED.

// readInteractions returns the decoded ndjson ledger for phase in ws.
func readInteractions(t *testing.T, ws, phase string) []interaction.Outcome {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, phase+"-interactions.ndjson"))
	if os.IsNotExist(err) {
		return nil // no ledger yet is a legitimate "nothing recorded"
	}
	if err != nil {
		t.Fatalf("interactions ledger unreadable (NOT the same as absent): %v", err)
	}
	var out []interaction.Outcome
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var o interaction.Outcome
		if err := json.Unmarshal([]byte(ln), &o); err != nil {
			t.Fatalf("interactions ndjson line is not an Outcome: %v\n%s", err, ln)
		}
		out = append(out, o)
	}
	return out
}

// TestVerifySubmitted_ReportsOutcome pins that verifySubmitted distinguishes the
// three cases the log currently conflates into silence.
func TestVerifySubmitted_ReportsOutcome(t *testing.T) {
	clean := "● done\n\n" + tmuxPromptMarkerDefault
	lp := tmuxLaunch{name: "claude-tmux", session: "s",
		promptMarker: tmuxPromptMarkerDefault, inputLineMarker: tmuxPromptMarkerDefault}

	t.Run("clean input line reports prompt_cleared with no re-sends", func(t *testing.T) {
		tm := &fakeTmux{paneSeq: []string{clean}}
		var stderr bytes.Buffer
		got := verifySubmitted(context.Background(), submitVerifyDeps(tm, &stderr), lp,
			"[claude-tmux]", "prompt", clean, guardNudge)
		if got.Resends != 0 || got.Result != interaction.ResultSubmitVerified {
			t.Errorf("outcome = %+v, want {Resends:0 Result:%s} — a verified-clean submission must be "+
				"DISTINGUISHABLE from one the guard could not check", got, interaction.ResultSubmitVerified)
		}
	})

	t.Run("recovered submission reports the re-send count", func(t *testing.T) {
		tm := &fakeTmux{paneSeq: []string{clean}}
		var stderr bytes.Buffer
		got := verifySubmitted(context.Background(), submitVerifyDeps(tm, &stderr), lp,
			"[claude-tmux]", "nudge", parkedPane(guardNudge), guardNudge)
		if got.Resends != 1 || got.Result != interaction.ResultSubmittedAfterResend {
			t.Errorf("outcome = %+v, want {Resends:1 Result:%s} — the guard SAVED this phase and that "+
				"must leave a durable trace", got, interaction.ResultSubmittedAfterResend)
		}
	})

	t.Run("an unobservable pane reports not_verified, not clean", func(t *testing.T) {
		// A failed CapturePane hands verifySubmitted "". pendingAtInputLine then
		// returns false on the absent marker, so without an explicit guard the
		// fall-through records "verified clean" for a state the driver just
		// logged as unknown — a lie in the safe-looking direction, in the very
		// ledger this change exists to make trustworthy.
		tm := &fakeTmux{paneSeq: []string{clean}}
		var stderr bytes.Buffer
		got := verifySubmitted(context.Background(), submitVerifyDeps(tm, &stderr), lp,
			"[claude-tmux]", "prompt", "", guardNudge)
		if got.Result != interaction.ResultNotVerified {
			t.Errorf("outcome = %+v, want Result:%s — an empty pane is NOT a clear input line",
				got, interaction.ResultNotVerified)
		}
		if n := len(tm.sentSeq); n != 0 {
			t.Errorf("sent %d key(s) against an unobservable pane, want 0", n)
		}
	})

	t.Run("no input-line marker reports not_verified, not clean", func(t *testing.T) {
		tm := &fakeTmux{paneSeq: []string{clean}}
		var stderr bytes.Buffer
		agy := tmuxLaunch{name: "agy-tmux", session: "s", promptMarker: "? for shortcuts"}
		got := verifySubmitted(context.Background(), submitVerifyDeps(tm, &stderr), agy,
			"[agy-tmux]", "prompt", parkedPane(guardNudge), guardNudge)
		if got.Resends != 0 || got.Result != interaction.ResultNotVerified {
			t.Errorf("outcome = %+v, want {Resends:0 Result:%s} — 'could not verify' must never read as "+
				"'verified clean'", got, interaction.ResultNotVerified)
		}
	})
}

// TestTmuxREPL_SubmitVerify_RecordReachesLedger is the WIRING proof: it drives
// the real production entry point and asserts the record lands in the file an
// operator would read. PR #474's stderr lines were "loud" and still unreachable;
// a unit test of the outcome alone would repeat that mistake.
//
// BOTH rows matter. The recovered row proves the guard's win is durable; the
// CLEAN row is the denominator — without it a "don't spam the ledger" early
// return in recordSubmitVerify passes every other assertion here, and a
// recovered stall stays an anecdote instead of a rate.
func TestTmuxREPL_SubmitVerify_RecordReachesLedger(t *testing.T) {
	run := func(t *testing.T, tm TmuxController) []interaction.Outcome {
		t.Helper()
		fx := newFixture(t, "claude-tmux", "")
		eng := NewEngine(Deps{Tmux: tm, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)})
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		var stdout, stderr bytes.Buffer
		code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=1526"), nil, &stdout, &stderr)
		if code == ExitBadFlags {
			t.Fatalf("launch fixture broke (exit %d) — a missing record below would misattribute that to the guard\nstderr:\n%s", code, stderr.String())
		}
		var out []interaction.Outcome
		for _, o := range readInteractions(t, fx.ws, "build") {
			if o.Kind == interaction.KindSubmitVerify {
				out = append(out, o)
			}
		}
		return out
	}

	t.Run("recovered submission is recorded with its re-send count", func(t *testing.T) {
		recs := run(t, &pasteStickyTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}})
		if len(recs) == 0 {
			t.Fatalf("no %q record — the guard's outcome is still unobservable on the success path, which is the whole defect", interaction.KindSubmitVerify)
		}
		got := recs[0]
		if got.Result != interaction.ResultSubmittedAfterResend {
			t.Errorf("Result = %q, want %q — a recovered stall must be identifiable as such", got.Result, interaction.ResultSubmittedAfterResend)
		}
		if !strings.Contains(got.Payload, "site=prompt") || !strings.Contains(got.Payload, "resends=1") {
			t.Errorf("payload = %q, want site=prompt and resends=1", got.Payload)
		}
	})

	t.Run("clean submission is recorded too (the denominator)", func(t *testing.T) {
		recs := run(t, &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}})
		if len(recs) == 0 {
			t.Fatalf("clean deliveries are NOT recorded — without the denominator a recovered stall is an anecdote, not a rate")
		}
		got := recs[0]
		if got.Result != interaction.ResultSubmitVerified {
			t.Errorf("Result = %q, want %q", got.Result, interaction.ResultSubmitVerified)
		}
		if !strings.Contains(got.Payload, "resends=0") {
			t.Errorf("payload = %q, want resends=0", got.Payload)
		}
	})
}
