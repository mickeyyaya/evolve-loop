package clicontrol

// usage_test.go — reading each CLI's remaining quota through the abstraction.
//
// The CALLER names a family and an intent. It never names a command: `/status`
// is codex's spelling, `/usage` is claude's and agy's, and ollama has none at
// all. Those mappings live in each family's manifest `controls` table and are
// resolved by the Controller, so a CLI renaming its command is a config edit.
// The same must hold for READING the reply — each family formats its own
// answer, so each family gets its own parser, and none of that shape is
// visible to the caller.
//
// Every fixture below is captured VERBATIM from a live CLI. That is the whole
// point: a usage reader built against an imagined format is worse than none,
// because it reports a confident number for a pane it never actually saw.

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// realCodexStatusPane — codex 0.147.0, captured live 2026-08-29. Note the
// shape traps: a unicode bar before the figure, TWO "Weekly limit" rows (the
// account's, and one nested under a per-model heading), and a bodiless
// "GPT-5.3-Codex-Spark limit:" heading with no figure at all.
const realCodexStatusPane = `╭────────────────────────────────────────────────────────╮
│  >_ OpenAI Codex (v0.147.0)                            │
│  Model:                       gpt-5.6-sol (reasoning max, summaries auto)
│  Account:                     mickeyyaya@gmail.com (Pro Lite)
│
│  Weekly limit:                [████████████████████] 100% left (resets 00:45 on 5 Sep)
│  GPT-5.3-Codex-Spark limit:
│  5h limit:                    [████████████████████] 100% left (resets 05:45)
│  Weekly limit:                [████████████████████] 100% left (resets 00:45 on 5 Sep)
╰────────────────────────────────────────────────────────╯`

// realClaudeUsagePane — the figure claude reports about itself, captured live
// 2026-08-28 (Claude Code v2.1.248). claude states usage CONSUMED; codex states
// remaining. Normalising that difference is exactly what a caller should not
// have to know.
const realClaudeUsagePane = `▝▜██████▀  Opus 5 (1M context) with xhigh effort · Claude Max
You've used 97% of your weekly limit · resets Aug 30 at 9pm (Asia/Taipei)`

func TestParseUsage_Codex_ReadsRemainingNotConsumed(t *testing.T) {
	u, ok := ParseUsage("codex", realCodexStatusPane)
	if !ok {
		t.Fatalf("codex pane did not parse: %+v", u)
	}
	if u.Family != "codex" {
		t.Errorf("Family = %q, want codex", u.Family)
	}
	// Window COUNT on the full pane. Note what this does and does not prove:
	// mutation testing showed a phantom from the bodiless
	// "GPT-5.3-Codex-Spark limit:" heading can hide here behind neighbouring
	// matches, so this count is a shape check, NOT the phantom guard. The
	// phantom is pinned separately and sensitively by
	// TestParseCodexUsage_BodilessHeadingYieldsNoWindow on a minimal pane.
	if len(u.Windows) != 3 {
		t.Fatalf("got %d windows %+v, want 3 (weekly, 5h, weekly)", len(u.Windows), u.Windows)
	}
	for _, w := range u.Windows {
		if w.PercentLeft != 100 {
			t.Errorf("window %q PercentLeft = %d, want 100", w.Name, w.PercentLeft)
		}
	}
	if got := u.Windows[0].Name; got != "Weekly limit" {
		t.Errorf("window[0].Name = %q, want %q", got, "Weekly limit")
	}
	if got := u.Windows[1].Name; got != "5h limit" {
		t.Errorf("window[1].Name = %q, want %q", got, "5h limit")
	}
	if !strings.Contains(u.Windows[0].ResetHint, "5 Sep") {
		t.Errorf("window[0].ResetHint = %q, want it to carry the reset text", u.Windows[0].ResetHint)
	}
	if u.MinPercentLeft() != 100 {
		t.Errorf("MinPercentLeft = %d, want 100", u.MinPercentLeft())
	}
}

// claude reports CONSUMED ("used 97%"), codex reports REMAINING ("100% left").
// The normalized type is remaining, so the claude parser must invert. Getting
// this backwards would report 97% headroom at 3% — the most dangerous possible
// direction for a scheduling decision.
func TestParseUsage_Claude_InvertsConsumedToRemaining(t *testing.T) {
	u, ok := ParseUsage("claude", realClaudeUsagePane)
	if !ok {
		t.Fatalf("claude pane did not parse: %+v", u)
	}
	if len(u.Windows) != 1 {
		t.Fatalf("got %d windows %+v, want 1", len(u.Windows), u.Windows)
	}
	if got := u.Windows[0].PercentLeft; got != 3 {
		t.Errorf("PercentLeft = %d, want 3 (claude said 97%% USED)", got)
	}
	if !strings.Contains(u.Windows[0].ResetHint, "Aug 30") {
		t.Errorf("ResetHint = %q, want the reset text", u.Windows[0].ResetHint)
	}
}

// An unrecognised pane must report NOT PARSED, never a fabricated figure. A
// quota reader that invents 0 (or 100) on unfamiliar output is worse than one
// that admits it cannot tell: both look identical to a caller, but one is a
// lie that will be acted on.
func TestParseUsage_UnrecognisedPaneIsHonestlyUnparsed(t *testing.T) {
	for _, tc := range []struct{ family, pane string }{
		{"codex", "some future redesign with no percentages at all"},
		{"claude", "Welcome back!"},
		{"agy", "Antigravity CLI 1.1.22"}, // agy's /usage output is NOT yet verified live
	} {
		u, ok := ParseUsage(tc.family, tc.pane)
		if ok {
			t.Errorf("%s: unrecognised pane reported as parsed: %+v", tc.family, u)
		}
		if len(u.Windows) != 0 {
			t.Errorf("%s: unparsed result carries %d phantom windows", tc.family, len(u.Windows))
		}
		if u.Raw == "" {
			t.Errorf("%s: unparsed result dropped the Raw pane — the operator loses the only evidence", tc.family)
		}
	}
}

// A family with no parser is not an error and not a zero — it is "unknown".
func TestParseUsage_UnknownFamilyIsUnparsedNotZero(t *testing.T) {
	u, ok := ParseUsage("ollama", "NAME  ID  SIZE")
	if ok {
		t.Errorf("ollama has no usage surface; want not-parsed, got %+v", u)
	}
	if u.MinPercentLeft() != -1 {
		t.Errorf("MinPercentLeft on an unparsed Usage = %d, want -1 (unknown), never 0 (exhausted)", u.MinPercentLeft())
	}
}

// --- the abstraction seam ---

type fakeController struct {
	pane string
	err  error
	// gotFamily/gotEvent record what the caller asked for, so the test can
	// prove the CALLER never names a command.
	gotFamily string
	gotEvent  Event
}

func (f *fakeController) Do(_ context.Context, family string, ev Event) (Response, error) {
	f.gotFamily, f.gotEvent = family, ev
	if f.err != nil {
		return Response{}, f.err
	}
	return Response{Family: family, Event: ev, Pane: f.pane}, nil
}

// QueryUsage must go through the Controller with the ABSTRACT event. The
// concrete command (/status for codex, /usage for claude and agy) is resolved
// from the family's manifest by the Controller — never chosen here.
func TestQueryUsage_AsksTheAbstractEventNotACommand(t *testing.T) {
	ctrl := &fakeController{pane: realCodexStatusPane}
	u, err := QueryUsage(context.Background(), ctrl, "codex")
	if err != nil {
		t.Fatalf("QueryUsage: %v", err)
	}
	if ctrl.gotEvent != EventUsage {
		t.Errorf("asked for event %q, want %q — the caller must name an intent, not a command", ctrl.gotEvent, EventUsage)
	}
	if ctrl.gotFamily != "codex" {
		t.Errorf("family = %q, want codex", ctrl.gotFamily)
	}
	if u.MinPercentLeft() != 100 {
		t.Errorf("MinPercentLeft = %d, want 100", u.MinPercentLeft())
	}
}

// A family that declares no usage control (ollama) surfaces ErrUnsupported
// UNCHANGED, so callers can branch on it with errors.Is rather than guessing
// from an empty result.
func TestQueryUsage_PropagatesUnsupportedForFamiliesWithoutTheControl(t *testing.T) {
	ctrl := &fakeController{err: ErrUnsupported}
	_, err := QueryUsage(context.Background(), ctrl, "ollama")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported so callers can branch on it", err)
	}
}

// A captured-but-unreadable pane is NOT an error: the control ran, we simply
// could not read it. The caller gets Parsed=false plus the raw pane, and can
// tell that apart from "the CLI has no usage surface" (ErrUnsupported) and
// from "the control failed" (a real error).
func TestQueryUsage_UnreadablePaneIsNotAnError(t *testing.T) {
	ctrl := &fakeController{pane: "totally unfamiliar output"}
	u, err := QueryUsage(context.Background(), ctrl, "codex")
	if err != nil {
		t.Fatalf("an unreadable pane must not be an error: %v", err)
	}
	if u.Parsed {
		t.Error("Parsed = true on unfamiliar output")
	}
	if u.Raw == "" {
		t.Error("Raw pane dropped — the operator loses the only evidence of what was actually shown")
	}
}

// A bodiless heading must yield NOTHING. codex prints
// "GPT-5.3-Codex-Spark limit:" with no bar and no figure, and a reader that
// turns that into a window reports 0% left for a limit the CLI never
// quantified — indistinguishable from genuine exhaustion, and the direction
// that would wrongly stop work.
//
// Asserted on a MINIMAL pane rather than inside the full-status test: in the
// full pane a phantom can hide behind neighbouring matches, so that test's
// window count proved less sensitive than it looked. This one cannot be
// satisfied by accident — there is nothing else here to match.
func TestParseCodexUsage_BodilessHeadingYieldsNoWindow(t *testing.T) {
	for _, pane := range []string{
		"  GPT-5.3-Codex-Spark limit:",
		"  GPT-5.3-Codex-Spark limit:\n",
		"  Weekly limit:",
		"  Weekly limit:\n  5h limit:\n",
	} {
		got, ok := parseCodexUsage(pane)
		if ok || len(got) != 0 {
			t.Errorf("pane %q produced %d window(s) %+v — a heading with no figure must not become a quota reading", pane, len(got), got)
		}
	}
}

// MinPercentLeft must report the TIGHTEST window, because that is the one that
// will stop work first. codex reports several at once (weekly and 5h), and a
// reader that averaged them — or took the first — would cheerfully report
// headroom while the 5h window was empty.
//
// Constructed directly rather than parsed, so the selection logic is pinned
// independently of any CLI's output format.
func TestUsage_MinPercentLeftPicksTheTightestWindow(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Usage
		want int
	}{
		{"tightest is last", Usage{Parsed: true, Windows: []Window{{Name: "weekly", PercentLeft: 80}, {Name: "5h", PercentLeft: 12}}}, 12},
		{"tightest is first", Usage{Parsed: true, Windows: []Window{{Name: "5h", PercentLeft: 4}, {Name: "weekly", PercentLeft: 91}}}, 4},
		{"single window", Usage{Parsed: true, Windows: []Window{{Name: "weekly", PercentLeft: 55}}}, 55},
		{"genuinely exhausted is 0, not unknown", Usage{Parsed: true, Windows: []Window{{Name: "weekly", PercentLeft: 0}}}, 0},
		// Unknown must never masquerade as exhausted: a caller throttling on
		// -1 can hold, but a caller throttling on 0 would stop work that could
		// have run.
		{"unparsed is unknown", Usage{Parsed: false}, -1},
		{"parsed but windowless is unknown", Usage{Parsed: true}, -1},
	} {
		if got := tc.in.MinPercentLeft(); got != tc.want {
			t.Errorf("%s: MinPercentLeft = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// The registry is the extension point: adding a CLI means adding a parser, not
// touching a caller. Pinned as a UsageParser value so the signature a
// contributor must implement is fixed by a test rather than by convention.
func TestUsageParser_RegistryShapeIsStable(t *testing.T) {
	var p UsageParser = func(pane string) ([]Window, bool) {
		if pane == "" {
			return nil, false
		}
		return []Window{{Name: "weekly", PercentLeft: 42}}, true
	}
	got, ok := p("anything")
	if !ok || len(got) != 1 || got[0].PercentLeft != 42 {
		t.Fatalf("UsageParser contract changed: got %+v ok=%v", got, ok)
	}
	if _, ok := p(""); ok {
		t.Error("a parser returning no windows must report ok=false")
	}
	// Every registered family must satisfy the same shape.
	for family, parser := range usageParsers {
		if parser == nil {
			t.Errorf("family %q has a nil parser registered", family)
		}
	}
}
