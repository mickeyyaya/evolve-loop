package bridge

// driver_tmux_submit_settle_test.go — the prompt-submission timing contract
// (verification wave 2026-09-14, cycles 1673–1675): a 17–35 KB prompt pasted
// into the codex TUI is still being ingested when the driver's fixed 1 s +
// three 500 ms re-sends have all fired, so the submission is declared
// "wedged" ~2.5 s after the paste and the whole dispatch is thrown away —
// while build/tdd/fault-localization in the same wave needed 2–3 re-sends
// just to land. The driver now (1) settles the first Enter by prompt size,
// (2) waits for the pane to stop changing before it presses Enter, and
// (3) backs off between re-sends instead of hammering. Vocabulary, stderr
// lines and the re-send cap are unchanged.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// recordingSleepDeps is submitVerifyDeps with every Sleep duration recorded.
func recordingSleepDeps(tm TmuxController, stderr *bytes.Buffer, sleeps *[]time.Duration) Deps {
	d := submitVerifyDeps(tm, stderr)
	d.Sleep = func(dur time.Duration) { *sleeps = append(*sleeps, dur) }
	return d
}

func writePrompt(t *testing.T, size int) string {
	t.Helper()
	pf := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(pf, bytes.Repeat([]byte("line of prompt text\n"), size/20+1), 0o644); err != nil {
		t.Fatal(err)
	}
	return pf
}

func enterCount(seq []string) int {
	n := 0
	for _, s := range seq {
		if s == "|true" { // the fake records SendKeys as "keys|enter"; a bare Enter is "|true"
			n++
		}
	}
	return n
}

// The first Enter waits for the paste to settle: the settle scales with the
// prompt size (a 35 KB prompt gets longer than a 2 KB one) and the driver then
// polls the pane until two consecutive captures agree — the TUI has finished
// rendering what it ingested — before pressing Enter exactly once.
func TestPastePrompt_WaitsForTheStablePaneBeforeEnter(t *testing.T) {
	tm := &fakeTmux{paneSeq: []string{
		"› [Pasted Content 1 +40 lines]",
		"› [Pasted Content 1 +900 lines]",
		"› [Pasted Content 1 +1750 lines]",
		"› [Pasted Content 1 +1750 lines]", // stable: two captures agree
	}}
	var stderr bytes.Buffer
	var sleeps []time.Duration
	deps := recordingSleepDeps(tm, &stderr, &sleeps)
	pf := writePrompt(t, 35_000)
	if _, err := pastePrompt(context.Background(), deps, "[t]", "s", pf, false); err != nil {
		t.Fatal(err)
	}
	if got := enterCount(tm.sentSeq); got != 1 {
		t.Errorf("exactly one Enter after the paste settled, got %d: %v", got, tm.sentSeq)
	}
	if tm.paneIdx < 4 {
		t.Errorf("Enter must wait for two agreeing captures; captures taken = %d", tm.paneIdx)
	}
	small := pasteSettleFor(2_000)
	large := pasteSettleFor(35_000)
	if !(large > small) || small < time.Second {
		t.Errorf("the settle scales with size and never drops below the old 1 s: small=%v large=%v", small, large)
	}
	if len(sleeps) == 0 || sleeps[0] != large {
		t.Errorf("the first sleep is the size-scaled settle %v: %v", large, sleeps)
	}
}

// A pane that never stops changing (a TUI redrawing a spinner) must not hold
// the Enter forever: the stability wait is bounded and the driver presses on.
func TestPastePrompt_StabilityWaitIsBounded(t *testing.T) {
	tm := &countingTmux{}
	var stderr bytes.Buffer
	var sleeps []time.Duration
	deps := recordingSleepDeps(tm, &stderr, &sleeps)
	if _, err := pastePrompt(context.Background(), deps, "[t]", "s", writePrompt(t, 4_000), false); err != nil {
		t.Fatal(err)
	}
	if got := enterCount(tm.sentSeq); got != 1 {
		t.Errorf("Enter still pressed once at the cap, got %d", got)
	}
	if tm.captures > pasteStabilityMaxPolls+1 {
		t.Errorf("the stability wait is bounded to %d polls, took %d captures", pasteStabilityMaxPolls, tm.captures)
	}
	if !strings.Contains(stderr.String(), "pane still changing") {
		t.Errorf("giving up on stability is said out loud: %q", stderr.String())
	}
}

// Re-sends back off: each settle is longer than the last (500 ms, 1.5 s,
// 2.5 s), so three re-sends span seconds, not 1.5 s, and a TUI that needed a
// moment gets it before the driver calls it wedged.
func TestVerifySubmitted_BacksOffBetweenResends(t *testing.T) {
	prompt := guardNudge
	parked := parkedPane(prompt)
	tm := &fakeTmux{paneSeq: []string{parked, parked, "● done\n\n" + tmuxPromptMarkerDefault + " "}} // the initial pane is the argument; three captures follow the three re-sends
	var stderr bytes.Buffer
	var sleeps []time.Duration
	deps := recordingSleepDeps(tm, &stderr, &sleeps)
	lp := tmuxLaunch{session: "s", name: "claude-tmux", inputLineMarker: tmuxPromptMarkerDefault}
	out := verifySubmitted(context.Background(), deps, lp, "[t]", "prompt", parked, promptSubmitEcho(prompt))
	if out.Result != "submitted_after_resend" || out.Resends != 3 {
		t.Fatalf("outcome = %+v", out)
	}
	if len(sleeps) != 3 {
		t.Fatalf("one settle per re-send: %v", sleeps)
	}
	if sleeps[0] < submitVerifySettle || sleeps[1] <= sleeps[0] || sleeps[2] <= sleeps[1] || sleeps[0]+sleeps[1]+sleeps[2] < 4*time.Second {
		t.Errorf("the settles back off (strictly increasing, ≥ 4 s in total): %v", sleeps)
	}
}

// countingTmux returns a different pane on every capture.
type countingTmux struct {
	fakeTmux
	captures int
}

func (c *countingTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	c.captures++
	return "› [Pasted Content 1 +" + strings.Repeat("x", c.captures) + "]", nil
}

// An unreadable prompt file on the Go side (tmux's own load-buffer is another
// process and may have succeeded) must not silently shrink the paste to size
// 0 — that would skip the stability wait and reproduce the wedge with no line
// to say so (go review MAJOR). The driver says it out loud and assumes a LARGE
// paste: the capped settle and the stability wait both run.
func TestPastePrompt_UnreadablePromptAssumesALargePaste(t *testing.T) {
	tm := &fakeTmux{paneSeq: []string{"› [Pasted Content 1 +40 lines]", "› [Pasted Content 1 +40 lines]"}}
	var stderr bytes.Buffer
	var sleeps []time.Duration
	deps := recordingSleepDeps(tm, &stderr, &sleeps)
	if _, err := pastePrompt(context.Background(), deps, "[t]", "s", filepath.Join(t.TempDir(), "gone.txt"), false); err != nil {
		t.Fatalf("the fake load-buffer succeeded; the paste proceeds: %v", err)
	}
	if !strings.Contains(stderr.String(), "prompt file unreadable") {
		t.Errorf("the fault is said out loud: %q", stderr.String())
	}
	if len(sleeps) == 0 || sleeps[0] != pasteSettleMax {
		t.Errorf("an unknown size settles as a large paste (%v): %v", pasteSettleMax, sleeps)
	}
	if tm.paneIdx < 2 {
		t.Errorf("the stability wait runs for an unknown size; captures = %d", tm.paneIdx)
	}
	if got := enterCount(tm.sentSeq); got != 1 {
		t.Errorf("Enter once, got %d", got)
	}
}

// H1 — the timing that precedes the artifact wait must never sleep exactly
// artifactWaitInterval: the wedge short-circuit pins count Sleeps of that value
// as artifact-wait polls, so a colliding settle would be counted as a poll and
// red a pin about an unrelated subsystem (with the 1 s/10 KB formula the
// 10–20 KB band — the incident's own scout prompt — landed on exactly 2 s).
func TestPasteTiming_NeverEqualsTheArtifactWaitInterval(t *testing.T) {
	for size := 0; size <= 120_000; size += 100 {
		if d := pasteSettleFor(size); d == artifactWaitInterval {
			t.Fatalf("pasteSettleFor(%d) = %v == artifactWaitInterval", size, d)
		}
	}
	for i, d := range submitVerifyBackoff {
		if d == artifactWaitInterval {
			t.Errorf("submitVerifyBackoff[%d] = %v == artifactWaitInterval", i, d)
		}
	}
	if pasteStabilityPoll == artifactWaitInterval || pasteSettleMax == artifactWaitInterval {
		t.Error("the stability poll and the settle cap must not equal the artifact-wait interval either")
	}
}

// M5 — under the stability floor the delivery is byte-for-byte the old one: the
// old 1 s settle, no capture at all, Enter once (the no-regression floor every
// existing fixture relies on).
func TestPastePrompt_SmallPromptTakesTheCaptureFreePath(t *testing.T) {
	tm := &fakeTmux{paneSeq: []string{"› [Pasted Content 1 +2 lines]"}}
	var stderr bytes.Buffer
	var sleeps []time.Duration
	deps := recordingSleepDeps(tm, &stderr, &sleeps)
	pf, _ := writePromptSized(t, 500)
	out, err := pastePrompt(context.Background(), deps, "[t]", "s", pf, false)
	if err != nil {
		t.Fatal(err)
	}
	if tm.paneIdx != 0 || len(sleeps) != 1 || sleeps[0] != time.Second || enterCount(tm.sentSeq) != 1 {
		t.Errorf("captures=%d sleeps=%v enters=%d", tm.paneIdx, sleeps, enterCount(tm.sentSeq))
	}
	if out.Stability != pasteSkipped || out.Settle != time.Second {
		t.Errorf("outcome = %+v", out)
	}
}

// M5 — a tmux that cannot be read during the stability wait is said out loud
// and the driver presses Enter anyway (a dead session must become a verify
// outcome, never a hang).
func TestPastePrompt_CaptureFaultFailsOpen(t *testing.T) {
	tm := &captureErrTmux{fakeTmux: &fakeTmux{paneSeq: []string{"› x"}}, errAfter: 1}
	var stderr bytes.Buffer
	var sleeps []time.Duration
	deps := recordingSleepDeps(tm, &stderr, &sleeps)
	pf, _ := writePromptSized(t, 20_000)
	out, err := pastePrompt(context.Background(), deps, "[codex-tmux]", "s", pf, false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Stability != pasteCaptureFault || enterCount(tm.fakeTmux.sentSeq) != 1 {
		t.Errorf("outcome = %+v enters=%d", out, enterCount(tm.fakeTmux.sentSeq))
	}
	if !strings.Contains(stderr.String(), "[codex-tmux] paste settle: capture failed") {
		t.Errorf("the fault carries the driver prefix: %q", stderr.String())
	}
}

// L1/L2 — the contract, not the mechanism: the Enter is pressed only after the
// last two captures AGREED, and the expected settle is computed from the
// fixture's real size.
func TestPastePrompt_EnterFollowsTwoAgreeingCaptures(t *testing.T) {
	tm := &fakeTmux{paneSeq: []string{"› [Pasted Content 1 +40 lines]", "› [Pasted Content 1 +900 lines]", "› [Pasted Content 1 +900 lines]"}}
	var stderr bytes.Buffer
	var sleeps []time.Duration
	deps := recordingSleepDeps(tm, &stderr, &sleeps)
	pf, size := writePromptSized(t, 17_700)
	out, err := pastePrompt(context.Background(), deps, "[t]", "s", pf, false)
	if err != nil {
		t.Fatal(err)
	}
	if tm.paneIdx < 2 || tm.paneSeq[tm.paneIdx-1] != tm.paneSeq[tm.paneIdx-2] {
		t.Errorf("Enter only after two agreeing captures; consumed %d, last two %q / %q", tm.paneIdx, tm.paneSeq[tm.paneIdx-1], tm.paneSeq[tm.paneIdx-2])
	}
	if out.Stability != pasteStable || out.Settle != pasteSettleFor(size) || sleeps[0] != pasteSettleFor(size) {
		t.Errorf("outcome = %+v (size %d) sleeps=%v", out, size, sleeps)
	}
}

// H2 — the inject path is the same delivery: a multi-KB inject body settles by
// size and waits for the pane; a small one keeps the old 1 s and no capture.
func TestInjectText_SharesTheDeliveryTail(t *testing.T) {
	cfg := fixtureConfig(t)
	tm := &fakeTmux{paneSeq: []string{"› a", "› a"}}
	var stderr bytes.Buffer
	var sleeps []time.Duration
	deps := recordingSleepDeps(tm, &stderr, &sleeps)
	big := strings.Repeat("inject body line\n", 400) // ≈ 6.8 KB
	if err := injectText(context.Background(), cfg, deps, "s", big); err != nil {
		t.Fatal(err)
	}
	if tm.paneIdx < 2 || sleeps[0] != pasteSettleFor(len(big)) || enterCount(tm.sentSeq) != 1 {
		t.Errorf("big inject: captures=%d sleeps=%v enters=%d", tm.paneIdx, sleeps, enterCount(tm.sentSeq))
	}
	tm2 := &fakeTmux{paneSeq: []string{"› a"}}
	var sleeps2 []time.Duration
	deps2 := recordingSleepDeps(tm2, &stderr, &sleeps2)
	if err := injectText(context.Background(), cfg, deps2, "s", "small"); err != nil {
		t.Fatal(err)
	}
	if tm2.paneIdx != 0 || len(sleeps2) != 1 || sleeps2[0] != time.Second || enterCount(tm2.sentSeq) != 1 {
		t.Errorf("small inject keeps the old delivery: captures=%d sleeps=%v", tm2.paneIdx, sleeps2)
	}
}

// H3 — the submit-verify ledger carries the paste evidence on the success path
// (a rate needs a denominator); a nudge, which is a SendKeys and not a paste,
// carries none.
func TestRecordSubmitVerify_CarriesThePasteEvidence(t *testing.T) {
	rec := interaction.NewRecorder(t.TempDir())
	recordSubmitVerify(rec, "build", 7, "prompt", submitVerifyOutcome{Result: interaction.ResultSubmitVerified}, pasteOutcome{Settle: 4 * time.Second, Stability: pasteStable})
	recordSubmitVerify(rec, "build", 7, "nudge", submitVerifyOutcome{Resends: 1, Result: interaction.ResultSubmittedAfterResend}, pasteOutcome{})
	got := rec.Outcomes()
	if len(got) != 2 || got[0].Event.Payload != "site=prompt resends=0 paste_settle=4s stability=stable" || got[1].Event.Payload != "site=nudge resends=1" {
		t.Errorf("payloads = %+v", got)
	}
}

// writePromptSized writes a prompt of at least size bytes and returns its path
// and EXACT size, so expectations are computed from the real fixture.
func writePromptSized(t *testing.T, size int) (string, int) {
	t.Helper()
	pf := writePrompt(t, size)
	fi, err := os.Stat(pf)
	if err != nil {
		t.Fatal(err)
	}
	return pf, int(fi.Size())
}
