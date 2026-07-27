package bridge

// fatalpane_strip_c1117_test.go — cycle-1117 RED tests for
// `fatalpane-strip-agent-content` (re-attempt of the cycle-1115 diff the
// auditor FAILed on defects D1 + D2).
//
// THE ASYMMETRY BEING CLOSED. driver_tmux_repl.go's exhaustion scan reads a
// pane with agent-rendered content removed (strippedForExhaustionScan), but
// fatalpane.go hands the fatal-pane detector the RAW ev.StdoutTail. So an
// agent EDITING recovery/detector.go — its diff view literally rendering
// `+ Substr: "There's an issue with the selected model"` — can be fast-failed
// on its own edit buffer, while the exhaustion detector, one field away, is
// immune. Two detectors, two meanings of "the pane".
//
// WHY THE NAIVE FIX WAS REJECTED (cycle-1115 audit, confidence 0.90):
//
//	D1 — stripPromptEchoLines DELETES matching lines and rejoins with "\n",
//	     shifting every survivor's position. Four seeded signatures are
//	     newline-ANCHORED ("\nquote>", "\nbquote>", "\ndquote>",
//	     "\nheredoc>") precisely so a bare word cannot false-positive.
//	     Deleting the line above a continuation prompt makes the survivor
//	     line one, drops its leading "\n", and Detect silently misses —
//	     reverting the cycle-274 dead-shell fast-fail in exactly the
//	     prompt-spill scenario it was seeded for.
//	D2 — echo-stripping is substring-keyed, and two seeds are literal English
//	     sentences. Any phase prompt that QUOTES one (a detector-hardening
//	     cycle, a retro, this very todo's prose) makes the CLI's real banner
//	     indistinguishable from an echo and silences the cycle-262 fast-fail.
//
// THE CONTRACT THESE TESTS PIN (the auditor's remediation direction, and the
// production API Builder must create — RED today = compile failure):
//
//	recovery: func (d *FatalPaneDetector) Signatures() []string
//	    the seeded substrings, so the stripper's protect-list can never drift
//	    from the registry it is protecting.
//	bridge:   func strippedForFatalPaneScan(pane, injectedPrompt string, protected []string) string
//	    the fatal-pane twin of strippedForExhaustionScan. Removes agent-diff
//	    AND prompt-echo content by BLANKING each matched line in place —
//	    never deleting it — so every surviving line keeps its leading "\n"
//	    (D1). A line containing any TrimSpace'd entry of protected is left
//	    untouched by the prompt-echo half (D2): a prompt quoting a fatal
//	    signature must never suppress that signature on-pane. The diff half
//	    is NOT protect-listed — diff-prefixing is proof of agent authorship
//	    by construction (cycle-314), and suppressing agent-authored seed text
//	    is the entire point of this task (see C1117_003 below).
//	bridge:   StopEvent.InjectedPrompt string
//	    populated at the driver's checkpoint construction site from the
//	    already-resolved prompt (the same source strippedForExhaustionScan
//	    uses), and consumed by fatalPaneVerdict.
//
// DO NOT MODIFY THESE TESTS to make them pass — they are the acceptance
// criteria. The parameter NAMES of strippedForFatalPaneScan are part of the
// contract: go/acs/cycle1117 mutates the function by overlay to prove these
// tests are load-bearing, and a renamed parameter fails that predicate loudly.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// c1117Ev builds a non-busy artifact-timeout checkpoint carrying both the pane
// tail and the prompt injected into that session — the evidence pair
// fatalPaneVerdict must now consult together.
func c1117Ev(tail, injectedPrompt string) StopEvent {
	return StopEvent{
		Kind: StopArtifactTimeout, Phase: "build", Cycle: 1117,
		ElapsedS: 300, IntervalS: 300, Attempt: 0,
		Progressed:     true, // the nudge-echo trap: a dead pane CAN read as progressed
		Busy:           false,
		StdoutTail:     tail,
		InjectedPrompt: injectedPrompt, // RED: field does not exist yet
	}
}

// c1117EchoLine is an ordinary instruction line present verbatim in both the
// injected prompt and the pane — the echo the stripper is meant to neutralise.
// It carries no fatal signature of its own.
const c1117EchoLine = "Write the report to workspace/build-report.md when you are done."

// c1117AnchoredSeeds are the four newline-anchored dead-shell signatures. The
// anchor is the whole defence against a bare word false-positive, so line
// POSITION is load-bearing for every one of them (cycle-274/277).
var c1117AnchoredSeeds = []string{"\nquote>", "\nbquote>", "\ndquote>", "\nheredoc>"}

// TestC1117_AnchoredSeedSurvivesEchoStripping — AC1 + AC4, the D1 regression.
// A zsh continuation prompt (prompt spill → the REPL is gone) sitting directly
// below an echoed prompt line must STILL fast-fail after stripping. Asserted at
// two levels: the helper preserves line structure (same newline count, anchor
// intact), and the real fatalPaneVerdict still preempts at enforce.
//
// The second variant is the D2 half for anchored seeds: when the prompt also
// quotes the continuation token itself, the echo-strip must leave that line
// alone — blanking it would destroy the "\nquote>" match just as surely as
// deleting the line above it.
func TestC1117_AnchoredSeedSurvivesEchoStripping(t *testing.T) {
	t.Parallel()
	det := recovery.SeedDetector()
	for _, seed := range c1117AnchoredSeeds {
		token := strings.TrimPrefix(seed, "\n") // "quote>", "bquote>", …
		for _, tc := range []struct {
			name   string
			prompt string
		}{
			{"prompt echoes an ordinary line", c1117EchoLine},
			{"prompt also quotes the continuation token", c1117EchoLine + "\nWatch for a " + token + " spill."},
		} {
			pane := c1117EchoLine + "\n" + token + "\n"
			if _, _, ok := det.Detect(pane); !ok {
				t.Fatalf("%s: raw pane does not detect — fixture is wrong, not the code", token)
			}

			stripped := strippedForFatalPaneScan(pane, tc.prompt, det.Signatures()) // RED: neither symbol exists yet
			if got, want := strings.Count(stripped, "\n"), strings.Count(pane, "\n"); got != want {
				t.Errorf("%s / %s: stripped pane has %d newlines, want %d — lines were DELETED, not blanked; every anchored seed below the cut loses its leading newline (D1)",
					token, tc.name, got, want)
			}
			if !strings.Contains(stripped, seed) {
				t.Errorf("%s / %s: anchored seed %q lost from the stripped pane (%q) — the cycle-274 dead-shell fast-fail is reverted for prompt-spill panes (D1)",
					token, tc.name, seed, stripped)
			}

			var buf bytes.Buffer
			v, preempted := fatalPaneVerdict(det, c1117Ev(pane, tc.prompt), "enforce", nil, &buf, "[c1117]")
			if !preempted {
				t.Errorf("%s / %s: enforce did not preempt a dead shell — the phase burns the full maxExtends backstop on a REPL that no longer exists", token, tc.name)
				continue
			}
			if v.Action != ReviewStop {
				t.Errorf("%s / %s: action=%s, want stop", token, tc.name, v.Action)
			}
		}
	}
}

// TestC1117_PromptQuotingSeedDoesNotSuppressBanner — AC2, the D2 regression.
// The pane shows the CLI's REAL model-invalid banner while the injected prompt
// happens to quote that same sentence (a detector-hardening cycle, a retro,
// this todo). Substring-keyed echo stripping would eat the banner and silence
// the cycle-262 fast-fail; the protect-list must keep it.
func TestC1117_PromptQuotingSeedDoesNotSuppressBanner(t *testing.T) {
	t.Parallel()
	det := recovery.SeedDetector()
	pane := "⏺ booting\n" + fatalTail + "\n"
	// The whole pane line appears verbatim in the prompt — the worst case for a
	// substring-keyed stripper, and an entirely realistic one.
	prompt := "Task: harden the fatal-pane registry.\n" + fatalTail + "\nAuthor the predicate."

	stripped := strippedForFatalPaneScan(pane, prompt, det.Signatures())
	if !strings.Contains(stripped, "There's an issue with the selected model") {
		t.Errorf("the CLI's real banner was stripped because the prompt quotes it — a prompt that mentions a fatal signature must never suppress that signature on-pane (D2); stripped=%q", stripped)
	}

	var buf bytes.Buffer
	v, preempted := fatalPaneVerdict(det, c1117Ev(pane, prompt), "enforce", nil, &buf, "[c1117]")
	if !preempted {
		t.Fatal("enforce did not preempt a self-describing model-invalid boot because the prompt quoted the banner (D2) — cycle-262's 40-minute burn returns")
	}
	if !strings.Contains(v.Reason, string(recovery.CauseModelInvalid)) {
		t.Errorf("reason must carry the typed cause for the justification trail; got %q", v.Reason)
	}
}

// TestC1117_AgentDiffSeedTextDoesNotFastFail — AC1, the NEGATIVE axis and the
// load-bearing proof that fatalPaneVerdict consults a STRIPPED pane at all.
// An agent editing the fatal registry renders seed text on numbered diff lines;
// that is the agent's own content, not the CLI's chrome, and must not kill it.
// On the raw pane (today's code, or any pass-through "strip") this fast-fails.
//
// Note the deliberate asymmetry with C1117_002: diff lines are NOT protect-
// listed. Diff-prefixing is proof of agent authorship by construction
// (cycle-314), whereas "appears in the prompt" is the weak signal D2 showed can
// swallow a genuine banner.
func TestC1117_AgentDiffSeedTextDoesNotFastFail(t *testing.T) {
	t.Parallel()
	det := recovery.SeedDetector()
	pane := strings.Join([]string{
		"  editing go/internal/recovery/detector.go",
		"   223 +\t\t{",
		"   224 +\t\t\tSubstr: \"There's an issue with the selected model\",",
		"   225 +\t\t\tCause:  CauseModelInvalid,",
		"  ⏺ Working… (esc to interrupt)",
	}, "\n")
	if _, _, ok := det.Detect(pane); !ok {
		t.Fatal("raw pane does not detect — fixture is wrong; this test only means something if the RAW pane would fast-fail")
	}

	// Empty prompt: the diff half alone must carry this, and an empty prompt is
	// the fail-open case for the echo half.
	stripped := strippedForFatalPaneScan(pane, "", det.Signatures())
	if _, _, ok := det.Detect(stripped); ok {
		t.Errorf("seed text on an agent DIFF line still detects after stripping — the fatal-pane path is reading agent-authored content as CLI chrome; stripped=%q", stripped)
	}
	if got, want := strings.Count(stripped, "\n"), strings.Count(pane, "\n"); got != want {
		t.Errorf("stripped pane has %d newlines, want %d — diff lines must be BLANKED in place too, or an anchored seed below them loses its leading newline (D1)", got, want)
	}

	var buf bytes.Buffer
	if _, preempted := fatalPaneVerdict(det, c1117Ev(pane, ""), "enforce", nil, &buf, "[c1117]"); preempted {
		t.Fatal("enforce fast-failed a WORKING agent on its own edit buffer — this is the false-FAIL the task exists to remove, and the asymmetry with strippedForExhaustionScan")
	}
}

// TestC1117_StopEventCarriesInjectedPromptFromDriver — AC3, the wiring proof.
// The behavioral tests above pass on a field production never populates (the
// exact gap that left cycle-1115's helper inert). This drives the real
// production path — Engine.LaunchArgs → runTmuxREPL → the checkpoint's
// StopEvent construction — with a reviewer that records the event, and asserts
// the prompt actually arrived.
func TestC1117_StopEventCarriesInjectedPromptFromDriver(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots; artifact never appears
	rev := &scriptedReviewer{}

	code, _ := runTmuxRev(t, fx, tmux, rev, Deps{ArtifactTimeoutS: 2}, "--allow-bypass")
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout", code)
	}
	if len(rev.events) == 0 {
		t.Fatal("reviewer never consulted — no checkpoint ran")
	}
	got := rev.events[0].InjectedPrompt
	if got == "" {
		t.Fatal("StopEvent.InjectedPrompt is empty at the real checkpoint — the driver never threads the resolved prompt in, leaving the echo half of the fatal-pane strip inert in production (the cycle-1115 gap)")
	}
	if !strings.Contains(got, fx.token) {
		t.Errorf("StopEvent.InjectedPrompt = %q, want the resolved prompt containing %q — some other string is being threaded through", got, fx.token)
	}
}

// TestC1117_SignaturesAccessorMatchesRegistry — AC2's no-drift half. The
// stripper's protect-list must come FROM the registry, not a hand-copied
// literal that silently rots when Slice-5 promotes a new signature. Exercises
// the accessor against Detect itself: every returned entry must be a substring
// the detector actually fires on.
func TestC1117_SignaturesAccessorMatchesRegistry(t *testing.T) {
	t.Parallel()
	det := recovery.SeedDetector()
	sigs := det.Signatures()
	if len(sigs) == 0 {
		t.Fatal("Signatures() returned nothing — the protect-list would be empty and D2 unfixed")
	}
	for _, s := range sigs {
		if _, _, ok := det.Detect("preamble" + s + "\ntail"); !ok {
			t.Errorf("Signatures() returned %q but Detect does not fire on it — the accessor is out of sync with the registry it reports", s)
		}
	}
	for _, seed := range c1117AnchoredSeeds {
		found := false
		for _, s := range sigs {
			if s == seed {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("anchored seed %q missing from Signatures() — it would be unprotected against a prompt that quotes it", seed)
		}
	}
	// A promoted signature must show up too: the accessor reports the live
	// registry, not the seed list frozen at construction.
	det.Promote(recovery.FatalSignature{Substr: "c1117 promoted marker", Cause: recovery.CauseDeadShell, Note: "c1117 accessor liveness"})
	live := det.Signatures()
	if len(live) <= len(sigs) {
		t.Errorf("Signatures() returned %d entries after Promote, was %d — the accessor snapshots instead of reporting the live registry, so promoted signatures go unprotected", len(live), len(sigs))
	}
}
