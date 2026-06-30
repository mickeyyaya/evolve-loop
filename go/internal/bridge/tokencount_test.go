package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// tokencount_test.go — RED contract for cycle-256 task `token-counter-extraction`.
//
// The bridge already compiles `rxTokens` (↓ N.Nk tokens) only to STRIP volatile
// counter lines from cleanPane(). This task repurposes that same pattern to
// EXTRACT the peak observed token count into a structured value, write a
// `token-usage.json` sidecar after a tmux phase, and surface it on bridge.Report.
//
// These tests are behavioral: TestExtractTokenCount exercises the pure parser
// across positive/fractional/multi/no-match/malformed inputs (the strongest
// anti-no-op signal — a presence-only impl that returns a constant fails the
// peak + zero cases); TestTmuxPhase_WritesTokenUsage runs the real REPL engine
// and asserts the sidecar the agent's pane produced; TestBuildReport_TokenUsage
// asserts the report field is populated from (and tolerant of) the sidecar.

// TestExtractTokenCount pins the parser contract: a visible "↓ N.Nk tokens"
// counter → integer token count (k = thousands), the PEAK across multiple
// counters, and 0 for any no-match / malformed / empty input.
func TestExtractTokenCount(t *testing.T) {
	cases := []struct {
		name string
		pane string
		want int
	}{
		{"single integer-k", "↓ 12k tokens", 12000},
		{"single fractional-k", "↓ 5.2k tokens", 5200},
		{"half-k fractional", "↓ 3.5k tokens", 3500},
		{"quarter-k fractional", "↓ 2.5k tokens", 2500},
		{"surrounded by chrome", "❯ working\n↓ 5.2k tokens · 1m 2s\nplanning\n", 5200},
		{"multiple counters → peak", "↓ 1.2k tokens\n↓ 5.2k tokens\n↓ 3.0k tokens\n", 5200},
		{"peak is last line", "↓ 2.0k tokens\n↓ 9.9k tokens\n", 9900},
		{"no counter at all", "❯ ready\nTool: Read main.go\n", 0},
		{"empty pane", "", 0},
		{"malformed: no digits", "↓ k tokens", 0},
		// Reconciled (cycle-429 S1): the old k-only extractor returned 0 here; the
		// unified ExtractResponseTokens superset correctly yields 5200 (plain-integer
		// path). Production panes always render the k-form; this form only appears in
		// synthetic test frames where 5200 is the correct value.
		{"unified extractor: plain-integer → 5200", "↓ 5200 tokens", 5200},
		{"malformed: missing tokens word", "↓ 5.2k", 0},
		{"malformed: non-numeric", "↓ abck tokens", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := panestream.ExtractResponseTokens(c.pane); got != c.want {
				t.Fatalf("panestream.ExtractResponseTokens(%q) = %d, want %d", c.pane, got, c.want)
			}
		})
	}
}

// TestTmuxPhase_WritesTokenUsage drives the real claude-tmux REPL engine with a
// pane that carries a token counter and asserts the phase writes
// workspace/token-usage.json with the peak the pane showed. Behavioral: it
// invokes the engine end-to-end, not a string check on source.
func TestTmuxPhase_WritesTokenUsage(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// Pre-seed the artifact so the wait loop completes on its first poll.
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	// Every capture returns a pane carrying the REPL marker (so boot succeeds)
	// AND a "↓ 5.2k tokens" counter (so the extractor sees 5200).
	pane := tmuxPromptMarkerDefault + " working\n↓ 5.2k tokens · 1m 2s\n"
	tmux := &fakeTmux{paneSeq: []string{pane}}
	code, stderr := runTmux(t, fx, tmux, nil, "--allow-bypass")
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
	}

	usagePath := filepath.Join(fx.ws, "token-usage.json")
	data, err := os.ReadFile(usagePath)
	if err != nil {
		t.Fatalf("token-usage.json not written: %v", err)
	}
	var got struct {
		PeakTokens int `json:"peak_tokens"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("token-usage.json is not valid JSON: %v (raw=%q)", err, string(data))
	}
	if got.PeakTokens != 5200 {
		t.Errorf("peak_tokens = %d, want 5200 (from '↓ 5.2k tokens')", got.PeakTokens)
	}
	if got.PeakTokens < 0 {
		t.Errorf("peak_tokens = %d, must be >= 0", got.PeakTokens)
	}
}

// TestBuildReport_TokenUsage pins the report field: a valid token-usage.json
// populates Report.TokenUsage; a missing or malformed sidecar leaves it 0 and
// never breaks report generation (best-effort telemetry).
func TestBuildReport_TokenUsage(t *testing.T) {
	now := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)

	t.Run("valid sidecar populates field", func(t *testing.T) {
		ws := t.TempDir()
		mustWriteFile(t, filepath.Join(ws, "artifact.md"), "x")
		mustWriteFile(t, filepath.Join(ws, "token-usage.json"), `{"peak_tokens": 7400}`)
		r, err := BuildReport(ws, "artifact.md", now)
		if err != nil {
			t.Fatalf("BuildReport: %v", err)
		}
		if r.TokenUsage != 7400 {
			t.Errorf("Report.TokenUsage = %d, want 7400", r.TokenUsage)
		}
	})

	t.Run("missing sidecar → 0", func(t *testing.T) {
		ws := t.TempDir()
		mustWriteFile(t, filepath.Join(ws, "artifact.md"), "x")
		r, err := BuildReport(ws, "artifact.md", now)
		if err != nil {
			t.Fatalf("BuildReport: %v", err)
		}
		if r.TokenUsage != 0 {
			t.Errorf("Report.TokenUsage = %d, want 0 when sidecar absent", r.TokenUsage)
		}
	})

	t.Run("malformed sidecar → 0, no error", func(t *testing.T) {
		ws := t.TempDir()
		mustWriteFile(t, filepath.Join(ws, "artifact.md"), "x")
		mustWriteFile(t, filepath.Join(ws, "token-usage.json"), "{not json")
		r, err := BuildReport(ws, "artifact.md", now)
		if err != nil {
			t.Fatalf("malformed sidecar must not fail BuildReport: %v", err)
		}
		if r.TokenUsage != 0 {
			t.Errorf("Report.TokenUsage = %d, want 0 on malformed sidecar", r.TokenUsage)
		}
	})
}
