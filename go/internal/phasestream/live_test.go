package phasestream

import (
	"bufio"
	"os"
	"testing"
)

// feedFile streams a real CLI log through the classifier the way the
// live normalizer will: every stdout line, then a final FlushProgress.
// Returns all emitted envelopes and the raw line count.
func feedFile(t *testing.T, path string) ([]Envelope, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("fixture %s unavailable: %v", path, err)
	}
	defer f.Close()

	c := newTestClassifier()
	var out []Envelope
	lines := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<10), 1<<24) // 16MB: real stream-json lines are huge
	for sc.Scan() {
		lines++
		out = append(out, c.Line(sc.Bytes())...)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	if env, ok := c.FlushProgress(); ok {
		out = append(out, env)
	}
	return out, lines
}

// Live test against the real claude-p stream-json log committed for
// logfilter. Exercises authentic multi-event output (claude-p / codex /
// agy share this format).
func TestLive_ClaudeP_RealStreamJSON(t *testing.T) {
	out, lines := feedFile(t, "../logfilter/testdata/streamjson-input.log")
	if lines < 50 {
		t.Fatalf("fixture too small to be meaningful: %d lines", lines)
	}

	var progress, contentful int
	for _, e := range out {
		if e.Kind == "" {
			t.Errorf("envelope with empty kind at seq=%d", e.Seq)
		}
		if e.SchemaVersion != SchemaVersion {
			t.Errorf("seq=%d wrong schema_version %q", e.Seq, e.SchemaVersion)
		}
		switch e.Kind {
		case KindProgress:
			progress++
		case KindAssistantText, KindThinking, KindToolUse, KindResult, KindSystemHook, KindRateLimit:
			contentful++
		}
	}

	// stream_event deltas must have been coalesced into >=1 progress tick.
	if progress < 1 {
		t.Errorf("expected stream_event coalescing to yield a progress tick, got %d", progress)
	}
	// Real logs carry assembled assistant/system content.
	if contentful < 1 {
		t.Errorf("expected >=1 contentful envelope from real log, got %d", contentful)
	}
	// Noise reduction: emitted envelopes must be well below raw line count.
	if len(out) >= lines {
		t.Errorf("no noise reduction: %d envelopes from %d lines", len(out), lines)
	}
	t.Logf("claude-p: %d raw lines → %d envelopes (%d progress, %d contentful)",
		lines, len(out), progress, contentful)
}

// Live test against claude-tmux plaintext scrollback. Spinner/border
// noise must drop; recommendation prose must survive as assistant_text.
func TestLive_ClaudeTmux_PlaintextScrollback(t *testing.T) {
	out, lines := feedFile(t, "testdata/claude-tmux.txt")
	if lines < 5 {
		t.Fatalf("tmux fixture too small: %d", lines)
	}
	var keptProse bool
	for _, e := range out {
		if e.Kind != KindAssistantText {
			t.Errorf("tmux line should classify as assistant_text, got %q", e.Kind)
		}
		if txt, _ := e.Data["text"].(string); txt == "I recommend starting with the normalizer because every consumer depends on it." {
			keptProse = true
		}
	}
	if !keptProse {
		t.Errorf("recommendation prose was dropped — interactive/recommendation content must survive")
	}
	if len(out) >= lines {
		t.Errorf("no noise reduction on tmux scrollback: %d envelopes from %d lines", len(out), lines)
	}
	t.Logf("claude-tmux: %d raw lines → %d envelopes", lines, len(out))
}
