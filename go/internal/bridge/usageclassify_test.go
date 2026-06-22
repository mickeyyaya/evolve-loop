package bridge

import "testing"

// TestClassifyExhausted_RealManifests verifies the manifest-backed cap
// classifier on real /usage|/status output shapes. The critical guard is the
// HEALTHY "100% left" / "91% left" line: a naive "0% left" pattern matches the
// "0% left" substring inside "100% left" and would falsely bench a healthy CLI,
// so the pattern must be word-boundaried. Conservative by design — only an
// unambiguous exhaustion phrase benches.
func TestClassifyExhausted_RealManifests(t *testing.T) {
	cases := []struct {
		name, family, pane string
		want               bool
	}{
		{
			name:   "claude weekly limit reached",
			family: "claude",
			pane:   "You've reached your weekly limit. Resets Mon 9:00 AM.",
			want:   true,
		},
		{
			name:   "claude healthy usage",
			family: "claude",
			pane:   "Current usage: 12% of weekly limit (resets in 3 days)",
			want:   false,
		},
		{
			name:   "codex zero left is capped",
			family: "codex",
			pane:   "5h limit: 0% left (resets 14:39)",
			want:   true,
		},
		{
			name:   "codex healthy 100% / 91% left is NOT capped",
			family: "codex",
			pane:   "5h limit: 100% left (resets 14:39)\nWeekly limit: 91% left (resets 15:28 on 5 May)",
			want:   false,
		},
		{
			name:   "agy quota exceeded",
			family: "agy",
			pane:   "Quota exceeded for gemini-2.5-pro. Try again later.",
			want:   true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyExhausted(tc.family, tc.pane); got != tc.want {
				t.Errorf("ClassifyExhausted(%q)=%v, want %v\npane=%q", tc.family, got, tc.want, tc.pane)
			}
		})
	}
}

// TestClassifyExhausted_FallsBackToRateLimitPattern verifies that a usage
// control with no exhausted_regex falls back to the manifest's maintained
// rate_limit interactive-prompt regex (single source for "what a wall looks
// like"), and that a manifest that cannot be loaded fails open to "not capped".
func TestClassifyExhausted_FallsBackToRateLimitPattern(t *testing.T) {
	// No exhausted_regex on the control → use the rate_limit prompt regex.
	m := Manifest{
		CLI:    "x-tmux",
		Binary: "x",
		Controls: map[string]ControlSpec{
			"usage": {Send: "/usage"}, // no ExhaustedRegex
		},
		InteractivePrompts: []ManifestPrompt{
			{Name: "rate_limit", Regex: "(?i)too many requests"},
		},
	}
	if !matchExhausted(manifestExhaustedPattern(m), "Error: too many requests, slow down") {
		t.Error("fallback to rate_limit pattern did not match a wall line")
	}
	if matchExhausted(manifestExhaustedPattern(m), "all good here") {
		t.Error("fallback pattern matched a healthy line")
	}

	// Unloadable family → fail open (not capped), never panic.
	if ClassifyExhausted("no-such-family", "0% left") {
		t.Error("unknown family classified as capped; want fail-open false")
	}
}
