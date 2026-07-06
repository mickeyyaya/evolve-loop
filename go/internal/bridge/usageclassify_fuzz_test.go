package bridge

// Fuzz harness for ClassifyExhausted (inbox fuzz-parser-surfaces, slice 1).
// Seeded from the TestClassifyExhausted_RealManifests goldens (real
// claude/codex/agy /usage pane shapes) plus known-adversarial family/pane
// combinations, then explores mutations. ClassifyExhausted is fail-open by
// design (unloadable manifest/pattern -> false, never invents a cap), so the
// only universal invariants are: never panic (an arbitrary family must not
// reach an unsafe filesystem path or an invalid-regex crash) and determinism
// (same input, same answer -> no hidden global state).

import "testing"

func FuzzClassifyExhausted(f *testing.F) {
	seeds := []struct{ family, pane string }{
		{"claude", "You've reached your weekly limit. Resets Mon 9:00 AM."},
		{"claude", "Current usage: 12% of weekly limit (resets in 3 days)"},
		{"codex", "5h limit: 0% left (resets 14:39)"},
		{"codex", "5h limit: 100% left (resets 14:39)\nWeekly limit: 91% left (resets 15:28 on 5 May)"},
		{"agy", "Quota exceeded for gemini-2.5-pro. Try again later."},
		{"no-such-family", "0% left"},
		{"", ""},
		{"claude", ""},
		{"../../../../etc/passwd", "0% left"},
		{"claude-tmux", "0% left"},
		{"claude", "quota exhausted: 日本語🚀"},
		{"claude\x00codex", "0% left\xff"},
	}
	for _, s := range seeds {
		f.Add(s.family, s.pane)
	}

	f.Fuzz(func(t *testing.T, family, pane string) {
		got := ClassifyExhausted(family, pane)
		again := ClassifyExhausted(family, pane)
		if got != again {
			t.Fatalf("ClassifyExhausted(%q, %q) is non-deterministic: %v then %v", family, pane, got, again)
		}
	})
}
