package bridge

import (
	"regexp"
	"testing"
)

// manifest_exhaustion_wording_test.go — regression lock for the claude-tmux
// usage.exhausted_regex against REAL captured quota-wall wording.
//
// Cycles 904–911 (2026-07-17) each burned ~20 min and shipped nothing: the
// auditor launched on `--model fable`, hit the Fable 5 per-model quota, and
// Claude Code rendered "You've reached your Fable 5 limit. Run /usage-credits
// to continue or switch models with /model." — then parked at its prompt.
// The manifest's exhausted_regex only matched the LEGACY wording
// ("reached your (usage|weekly) limit"), so the model-name-interpolated message
// slipped through, the pane never classified LivenessExhausted, the always-on
// exit-85 failover in driver_tmux_repl.go was bypassed, and the phase fell
// through to the 600s artifact-wait → exit 81 → self-heal relaunch → same
// exhausted quota → repeat (8 cycles).
//
// These fixtures are the ACTUAL strings captured in
// .evolve/runs/cycle-{904,905,908,909,910,911}/audit-escalation-report.json
// (final_pane). The prior exhaustion test used a SYNTHETIC "reached your usage
// limit" fixture that matched the old regex and passed — false confidence that
// let the upstream wording drift go uncaught. Verify against real data.
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

	// The quota WALL — must match (each is a real capture / a plausible sibling).
	walls := []string{
		// Exact string captured in cycle-910/911 final_pane. The per-model wall
		// ALWAYS carries its "/usage-credits" companion on the same line — the
		// regex requires it (see the notWalls rationale), so every real-wall
		// fixture must include it too:
		"You've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model.",
		// Same wall on other models, incl. the "You have" contraction-free form
		// and a version-numbered model name:
		"You've reached your Opus limit. Run /usage-credits to continue or switch models with /model.",
		"You have reached your Opus 4.8 limit. Run /usage-credits to continue.",
		// Legacy wordings — parity: these matched before and must keep matching:
		"reached your usage limit",
		"reached your weekly limit",
		"usage limit reached",
		"You've reached your usage limit — upgrade to continue.",
	}
	for _, w := range walls {
		if !re.MatchString(w) {
			t.Errorf("exhausted_regex must MATCH quota wall but did not:\n  %q\n  regex=%s", w, spec.ExhaustedRegex)
		}
	}

	// NOT a wall — must NOT match. Matching any of these would fast-fail a
	// WORKING agent (the cycle-254/255/314/641 false-FAIL sin). The pane is
	// scanned RAW at the stop-review checkpoint (driver_tmux_repl.go:669) with
	// no diff/prose stripping, so a false match kills a healthy agent. Two-round
	// review hardened the per-model branch: it requires BOTH Claude Code's
	// second-person chrome ("you(?:.?ve| have) reached your … limit") AND the
	// wall-specific "/usage-credits" companion adjacent on the same line — so
	// neither third-person prose NOR ordinary second-person prose NOR narration
	// that merely quotes the wall phrase (without the companion) can match.
	notWalls := []string{
		// soft "approaching" warning — still has quota:
		"You're approaching your Fable 5 limit.",
		"Approaching your usage limit soon — consider wrapping up.",
		// third-person / mid-sentence prose that merely mentions a limit:
		"if the client reached your API rate limit, retry with backoff",
		"a request that reached your daily quota limit should return 429",
		"reached your context limit",
		"reached your token limit",
		"the auditor reached your Fable 5 limit and fell back to sonnet",
		"I read the file and it mentions a rate limit in the retry code",
		// narration that quotes the wall phrase but is NOT the wall (no companion) —
		// e.g. an agent reading/discussing this very fix's artifacts (review round 2):
		"Claude Code now emits You've reached your Fable 5 limit wording",
		// ordinary SECOND-person prose — the "you've/you have" anchor alone is not
		// enough; requiring "/usage-credits" adjacency excludes it (review round 2):
		"do you think you have reached your daily commit limit for the day",
		// a bare per-model line WITHOUT the "/usage-credits" companion is
		// intentionally NOT matched: missing a companion-less wall only fails
		// over (safe), whereas matching prose kills a working agent (unsafe):
		"You've reached your Sonnet 5 limit.",
		// the wildcard pattern text itself must not self-match (the anchor prevents it):
		"reached your .{1,40}? limit",
	}
	for _, n := range notWalls {
		if re.MatchString(n) {
			t.Errorf("exhausted_regex must NOT match (would kill a working agent):\n  %q\n  regex=%s", n, spec.ExhaustedRegex)
		}
	}
}
