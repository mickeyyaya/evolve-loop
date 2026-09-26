package tokenusage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func eventsLogFixture(t *testing.T, in, out, cacheR, cacheC int) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "scout-events.ndjson")
	writeFile(t, log,
		`{"kind":"result","data":{"cost_usd":0.5,"tokens":{"in":`+itoa(in)+`,"out":`+itoa(out)+
			`,"cache_r":`+itoa(cacheR)+`,"cache_c":`+itoa(cacheC)+`}}}`+"\n")
	return log
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// transcriptFixture writes a cwd-attributed, in-window transcript reporting Input=200 Output=40.
func transcriptFixture(t *testing.T, root, worktree string) {
	t.Helper()
	sessionDir := filepath.Join(root, "projects", "-repo-worktrees-cycle-754")
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"start task"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":200,"output_tokens":40,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", sessionDir, err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "sess1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture transcript: %v", err)
	}
}

func TestDefaultResolver_EventsLogTier_WinsWhenNoTranscript(t *testing.T) {
	root := t.TempDir()
	log := eventsLogFixture(t, 1200, 340, 80, 16)

	resolver := DefaultResolver(root)
	res, err := resolver(Window{
		Worktree:      "/repo/worktrees/cycle-754",
		EventsLogPath: log,
		Start:         mustParse(t, launchWindowStart),
		End:           mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatalf("resolver errored on a valid events log: %v (telemetry is best-effort, must not error)", err)
	}
	if res.Source != SourceEventsResult {
		t.Fatalf("Source = %q, want %q (no transcript + present events log must resolve via the eventsResult tier)",
			res.Source, SourceEventsResult)
	}
	want := cyclestate.TokenUsage{Input: 1200, Output: 340, CacheRead: 80, CacheWrite: 16}
	if res.Usage != want {
		t.Errorf("eventsResult usage mismatch: got %+v want %+v (must match cyclecost's envelope extraction)", res.Usage, want)
	}
}

func TestDefaultResolver_ScrollbackTier_OutputOnlyFloor(t *testing.T) {
	root := t.TempDir()

	resolver := DefaultResolver(root)
	res, err := resolver(Window{
		Worktree:   "/repo/worktrees/cycle-754",
		Scrollback: "boot output\nsome REPL noise\n↓ 7k tokens\n",
		Start:      mustParse(t, launchWindowStart),
		End:        mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatalf("resolver errored on a scrollback-only window: %v", err)
	}
	if res.Source != SourceScrollbackPeak {
		t.Fatalf("Source = %q, want %q (scrollback marker is the last-resort tier and must fire)",
			res.Source, SourceScrollbackPeak)
	}
	if res.Usage.Output != 7000 {
		t.Errorf("Output = %d, want 7000 (ExtractResponseTokens peak)", res.Usage.Output)
	}
	if res.Usage.Input != 0 || res.Usage.CacheRead != 0 || res.Usage.CacheWrite != 0 {
		t.Errorf("scrollbackPeak is an output-only floor; non-output fields must stay zero, got %+v", res.Usage)
	}
}

func TestDefaultResolver_TranscriptTier_StillWinsOverLowerTiers(t *testing.T) {
	worktree := "/repo/worktrees/cycle-754"
	root := t.TempDir()
	transcriptFixture(t, root, worktree)
	log := eventsLogFixture(t, 9999, 9999, 9, 9)

	resolver := DefaultResolver(root)
	res, err := resolver(Window{
		Worktree:      worktree,
		EventsLogPath: log,
		Scrollback:    "↓ 99k tokens\n",
		Start:         mustParse(t, launchWindowStart),
		End:           mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatalf("resolver errored with all tiers populated: %v", err)
	}
	if res.Source != SourceTranscript {
		t.Fatalf("Source = %q, want %q (transcript > eventsResult > scrollbackPeak fidelity order)",
			res.Source, SourceTranscript)
	}
	if res.Usage.Input != 200 || res.Usage.Output != 40 {
		t.Errorf("transcript usage not propagated: got %+v, want Input=200 Output=40 (not the events log's 9999s)", res.Usage)
	}
}

func TestDefaultResolver_AllTiersEmpty_SourceNoneNilError(t *testing.T) {
	root := t.TempDir()

	resolver := DefaultResolver(root)
	res, err := resolver(Window{
		Worktree:      "/repo/worktrees/cycle-754",
		EventsLogPath: filepath.Join(t.TempDir(), "does-not-exist-events.ndjson"),
		Scrollback:    "",
		Start:         mustParse(t, launchWindowStart),
		End:           mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatalf("resolver must fail open (nil error) when no tier has data, got: %v", err)
	}
	if res.Source != SourceNone {
		t.Errorf("Source = %q, want %q when every tier is empty", res.Source, SourceNone)
	}
	if res.Usage != (cyclestate.TokenUsage{}) {
		t.Errorf("usage must be zero when no tier has data (no fabrication), got %+v", res.Usage)
	}
}

func TestDefaultResolver_MalformedEventsLog_FallsThroughCleanly(t *testing.T) {
	dir := t.TempDir()
	garbage := filepath.Join(dir, "scout-events.ndjson")
	writeFile(t, garbage, "{not json at all\x00\xff\ntruncated\n")

	t.Run("falls through to scrollback", func(t *testing.T) {
		resolver := DefaultResolver(t.TempDir())
		res, err := resolver(Window{
			Worktree:      "/repo/worktrees/cycle-754",
			EventsLogPath: garbage,
			Scrollback:    "↓ 3k tokens\n",
			Start:         mustParse(t, launchWindowStart),
			End:           mustParse(t, launchWindowEnd),
		})
		if err != nil {
			t.Fatalf("malformed events log must not error: %v", err)
		}
		if res.Source != SourceScrollbackPeak {
			t.Errorf("Source = %q, want %q (garbage tier-2 must fall through to tier 3)", res.Source, SourceScrollbackPeak)
		}
		if res.Usage.Output != 3000 {
			t.Errorf("Output = %d, want 3000 from the scrollback floor", res.Usage.Output)
		}
	})

	t.Run("falls through to none", func(t *testing.T) {
		resolver := DefaultResolver(t.TempDir())
		res, err := resolver(Window{
			Worktree:      "/repo/worktrees/cycle-754",
			EventsLogPath: garbage,
			Start:         mustParse(t, launchWindowStart),
			End:           mustParse(t, launchWindowEnd),
		})
		if err != nil {
			t.Fatalf("malformed events log must not error: %v", err)
		}
		if res.Source != SourceNone {
			t.Errorf("Source = %q, want %q (garbage tier-2, empty tier-3)", res.Source, SourceNone)
		}
		if res.Usage != (cyclestate.TokenUsage{}) {
			t.Errorf("usage must stay zero, got %+v", res.Usage)
		}
	})
}
