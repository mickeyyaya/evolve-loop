package panestream

import "testing"

const testBusyLineRegex = `^\s*([⣾⣽⣻⢿⡿⣟⣯⣷])\s+\S`

func busyLineProfile() PaneProfile {
	p := Profiles["agy"]
	p.BusyLineRegex = testBusyLineRegex
	return p
}

func TestWithoutSpinnerFrame_CutsOnlyTheCapturedFrame(t *testing.T) {
	re := compiledPattern(testBusyLineRegex)
	got, busy := withoutSpinnerFrame("⣽  Editing files...", re)
	if !busy || got != "  Editing files..." {
		t.Errorf("got (%q, %v), want the verb kept and the frame cut", got, busy)
	}
	if got, busy := withoutSpinnerFrame("  • 5", re); busy || got != "  • 5" {
		t.Errorf("content line changed: (%q, %v)", got, busy)
	}
	if got, busy := withoutSpinnerFrame("⣽  Editing files...", nil); busy || got != "⣽  Editing files..." {
		t.Errorf("no pattern must leave the line alone: (%q, %v)", got, busy)
	}
	whole := compiledPattern(`^\s*[⣾⣽]\s+\S`)
	if got, busy := withoutSpinnerFrame("⣾  Working...", whole); !busy || got != "⣾  Working..." {
		t.Errorf("a pattern with no frame group marks busy and keeps the line: (%q, %v)", got, busy)
	}
}

func TestCompiledPattern_InvalidAndEmptyPatternsMatchNothing(t *testing.T) {
	if compiledPattern("") != nil {
		t.Error("empty pattern must compile to nil")
	}
	if compiledPattern("([") != nil {
		t.Error("invalid pattern must compile to nil")
	}
	if compiledPattern("([") != nil {
		t.Error("cached invalid pattern must stay nil")
	}
	if compiledPattern(testBusyLineRegex) != compiledPattern(testBusyLineRegex) {
		t.Error("a valid pattern must be compiled once and reused")
	}
}

func TestPaneBusy_TheProfileSpinnerLineIsBusyEvidence(t *testing.T) {
	pane := "> task\n⣷  Working...\n────\n>\n────\n                Gemini 3.8 Flash · low\n"
	if !PaneBusy(pane, busyLineProfile()) {
		t.Error("a spinner line matching the profile's busy line must read busy")
	}
	if PaneBusy(pane, Profiles["agy"]) {
		t.Error("without a busy-line pattern the same pane has no busy evidence")
	}
}

func TestCleanPaneFor_SpinnerTickIsNotAChangeButAVerbChangeIs(t *testing.T) {
	p := busyLineProfile()
	a := cleanPaneFor("● Edit(x)\n⣽  Editing files...\n>", p)
	b := cleanPaneFor("● Edit(x)\n⢿  Editing files...\n>", p)
	c := cleanPaneFor("● Edit(x)\n⢿  Working...\n>", p)
	if a != b {
		t.Errorf("spinner tick changed the cleaned pane:\n%q\n%q", a, b)
	}
	if b == c {
		t.Error("a new verb is a real state change and must change the cleaned pane")
	}
}

func TestPaneDelta_SpinnerTickEmitsNoLines(t *testing.T) {
	p := busyLineProfile()
	var d PaneDelta
	d.Next("> task\n● Edit(x)\n⣽  Editing files...\n>\n", p)
	if got := d.Next("> task\n● Edit(x)\n⢿  Editing files...\n>\n", p); len(got) != 0 {
		t.Errorf("spinner tick emitted %q", got)
	}
}

func TestProgressHash_IgnoresTheSpinnerAndChromeButNotTranscript(t *testing.T) {
	p := busyLineProfile()
	tick := ProgressHash("● Edit(x)\n⣽  Editing files...\n────\n>\n────\nesc to cancel", p)
	tock := ProgressHash("● Edit(x)\n⢿  Editing files...\n────\n>\n────\nesc to cancel", p)
	done := ProgressHash("● Edit(x)\n  DONE\n────\n>\n────\n? for shortcuts", p)
	if tick != tock {
		t.Error("a spinner tick changed the progress hash")
	}
	if tick == done {
		t.Error("a new transcript line left the progress hash unchanged")
	}
	if len(tick) != 64 {
		t.Errorf("progress hash %q is not a sha256 hex digest", tick)
	}
}
