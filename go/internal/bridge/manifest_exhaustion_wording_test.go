package bridge

import (
	"context"
	"regexp"
	"testing"
	"time"
)

// TestClaudeTmuxExhaustedRegex_PerModelWording locks claude-tmux's usage.exhausted_regex against the
// literal strings captured from live escalation reports rather than a hand-written fixture, since a
// synthetic fixture can keep passing while the live wording drifts out from under the regex.
func TestClaudeTmuxExhaustedRegex_PerModelWording(t *testing.T) {
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(claude-tmux): %v", err)
	}
	spec, ok := m.Control("usage")
	if !ok {
		t.Fatal("claude-tmux Control(usage) not found")
	}
	if spec.ExhaustedRegex == "" {
		t.Fatal("claude-tmux usage.exhausted_regex is empty — exhaustion detection disabled")
	}
	re, err := regexp.Compile(spec.ExhaustedRegex)
	if err != nil {
		t.Fatalf("exhausted_regex does not compile: %v", err)
	}

	// Every wall fixture is built by concatenation ("li"+"mit") so the literal wall text never appears on
	// one source line: a verbatim string would itself match the live exhausted_regex (through the
	// persistence gate) when an agent reads this file, exit-85ing a healthy session reviewing it.
	walls := []string{
		"You've reached your Fable 5 li" + "mit. Run /usage-cre" + "dits to continue or switch models with /model.",
		"You've reached your Opus li" + "mit. Run /usage-cre" + "dits to continue or switch models with /model.",
		"You have reached your Opus 4.8 li" + "mit. Run /usage-cre" + "dits to continue.",
		"reached your usage li" + "mit",
		"reached your weekly li" + "mit",
		"usage li" + "mit reached",
		"You've reached your usage li" + "mit — upgrade to continue.",
		"You've hit your weekly li" + "mit · resets Jul 26 at 9pm (Asia/Taipei)",
		"You've hit your usage li" + "mit · resets tomorrow at 9am",
	}
	for _, w := range walls {
		if !re.MatchString(w) {
			t.Errorf("exhausted_regex must MATCH quota wall but did not:\n  %q\n  regex=%s", w, spec.ExhaustedRegex)
		}
	}

	// NOT a wall — must NOT match; matching any of these would fast-fail a working agent, since non-diff
	// pane renderings (Read-tool cat-n output, plain prose, log lines) reach the regex unstripped. The
	// per-model branch requires BOTH second-person chrome ("you(?:.?ve| have) reached your … limit") AND
	// the "/usage-credits" companion adjacent on the same line, so third-person prose, ordinary
	// second-person prose, and narration that merely quotes the wall phrase (without the companion) all miss.
	notWalls := []string{
		"You're approaching your Fable 5 limit.",
		"Approaching your usage limit soon — consider wrapping up.",
		"if the client reached your API rate limit, retry with backoff",
		"a request that reached your daily quota limit should return 429",
		"reached your context limit",
		"reached your token limit",
		"the auditor reached your Fable 5 limit and fell back to sonnet",
		"I read the file and it mentions a rate limit in the retry code",
		"Claude Code now emits You've reached your Fable 5 limit wording",
		"do you think you have reached your daily commit limit for the day",
		"You've reached your Sonnet 5 limit.",
		"reached your .{1,40}? limit",
		"the retry loop hit your rate limit, backing off",
		"you hit your context limit mid-review",
		"the build hit your token limit and truncated",
		"You're approaching your weekly limit.",
	}
	for _, n := range notWalls {
		if re.MatchString(n) {
			t.Errorf("exhausted_regex must NOT match (would kill a working agent):\n  %q\n  regex=%s", n, spec.ExhaustedRegex)
		}
	}
}

// Concatenation avoids triggering agents reading this test as ordinary tool output.
func TestClaudeSessionWallUsesGuardedExhaustion(t *testing.T) {
	wall := "  ⎿  You've hit your session li" + "mit · resets 5:10am (Asia/Taipei)"
	if !ClassifyExhausted("claude", wall) {
		t.Error("usage classifier missed captured session wall")
	}
	for _, prose := range []string{"the client hit your session limit yesterday", "You've hit your session limit while testing the classifier", "fixture: " + wall, "You're approaching your session limit"} {
		if ClassifyExhausted("claude", prose) {
			t.Errorf("ordinary prose classified as wall: %q", prose)
		}
	}
	for _, tc := range []struct {
		name      string
		panes     []string
		prompt    string
		confirmed bool
		want85    bool
		probes    int
	}{
		{name: "persistent wall", panes: []string{wall}, confirmed: true, want85: true, probes: 1},
		{name: "transient quoted wall", panes: []string{wall, "Working on the next test"}, confirmed: true},
		{name: "prompt echo", panes: []string{wall}, prompt: wall, confirmed: true},
		{name: "added diff", panes: []string{"+" + wall}, confirmed: true},
		{name: "removed diff", panes: []string{"-" + wall}, confirmed: true},
		{name: "persistent quoted content healthy", panes: []string{wall}, probes: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probes := 0
			deps := Deps{Tmux: &fakeTmux{paneSeq: tc.panes}, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), CorroborateWall: func(context.Context, string) bool { probes++; return tc.confirmed }}.withDefaults()
			ar := newAutoResponder("claude-tmux", t.TempDir(), deps, false, 0)
			ar.injectedPrompt = tc.prompt
			fired := false
			for i := 0; i < exhaustionPersistObservations+3; i++ {
				_, rc := ar.tick(context.Background(), "s")
				if rc == 85 {
					if i < exhaustionPersistObservations-1 {
						t.Fatal("wall bypassed persistence guard")
					}
					fired = true
					break
				}
			}
			if fired != tc.want85 || probes != tc.probes {
				t.Fatalf("escalated=%v probes=%d, want %v/%d", fired, probes, tc.want85, tc.probes)
			}
		})
	}
}

// tmux can render Claude Code's "⎿" indentation with U+00A0 rather than a plain space or tab; the
// session-wall branch must accept both, or a live wall never classifies. Concatenation keeps agents
// reading this test from matching.
func TestClaudeSessionWall_NBSPIndentIsRecognised(t *testing.T) {
	for _, live := range []string{
		"  ⎿ \u00a0You've hit your session li" + "mit · resets 7:50pm (Asia/Taipei)",
		"\u00a0\u00a0⎿\u00a0\u00a0You’ve hit your session li" + "mit · resets 7:50pm (Asia/Taipei)",
	} {
		if !ClassifyExhausted("claude", live) {
			t.Errorf("live wall with U+00A0 indentation not classified: %q", live)
		}
	}
}
