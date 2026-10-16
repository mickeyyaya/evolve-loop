package clicontrol

import (
	"context"
	"errors"
	"strings"
	"testing"
)

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

func TestParseUsage_UnrecognisedPaneIsHonestlyUnparsed(t *testing.T) {
	for _, tc := range []struct{ family, pane string }{
		{"codex", "some future redesign with no percentages at all"},
		{"claude", "Welcome back!"},
		{"agy", "Antigravity CLI 1.1.22"},
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

func TestParseUsage_UnknownFamilyIsUnparsedNotZero(t *testing.T) {
	u, ok := ParseUsage("ollama", "NAME  ID  SIZE")
	if ok {
		t.Errorf("ollama has no usage surface; want not-parsed, got %+v", u)
	}
	if u.MinPercentLeft() != -1 {
		t.Errorf("MinPercentLeft on an unparsed Usage = %d, want -1 (unknown), never 0 (exhausted)", u.MinPercentLeft())
	}
}

type fakeController struct {
	pane      string
	err       error
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

func TestQueryUsage_PropagatesUnsupportedForFamiliesWithoutTheControl(t *testing.T) {
	ctrl := &fakeController{err: ErrUnsupported}
	_, err := QueryUsage(context.Background(), ctrl, "ollama")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported so callers can branch on it", err)
	}
}

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
		{"unparsed is unknown", Usage{Parsed: false}, -1},
		{"parsed but windowless is unknown", Usage{Parsed: true}, -1},
	} {
		if got := tc.in.MinPercentLeft(); got != tc.want {
			t.Errorf("%s: MinPercentLeft = %d, want %d", tc.name, got, tc.want)
		}
	}
}

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
	for family, parser := range usageParsers {
		if parser == nil {
			t.Errorf("family %q has a nil parser registered", family)
		}
	}
}
