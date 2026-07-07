package tokenusage

// defaultresolver_test.go — RED contract for cycle-623 task
// token-resolver-production-wiring (inbox
// 2026-07-08T02-10-00Z-token-resolver-production-wiring.json, weight 0.96;
// scout-report.md hypothesis 1: "Wiring tokenusage.Chain(...) into both
// composition roots via one shared tokenusage.DefaultResolver(configRoot)
// helper").
//
// DefaultResolver is the SINGLE shared helper both production composition
// roots (internal/adapters/bridge.Adapter and internal/subagent's
// defaultExecAdapter) must call to build their Deps.TokenResolver — the fix
// for the confirmed bug (grep: 0 non-test hits for TokenResolver in either
// composition root) that has made token telemetry silently all-zero since at
// least cycle 612. DefaultResolver is undefined today, so this package fails
// to compile — the intended RED signal. Builder implements:
//
//	func DefaultResolver(configRoot string) func(Window) (Result, error) {
//	    return func(w Window) (Result, error) {
//	        return Chain(TranscriptCollector(configRoot, w)), nil
//	    }
//	}
//
// (S4/S5 tiers — EventsResultCollector, ScrollbackPeakCollector — are out of
// scope: Window carries no logPath/pane, only Worktree/ArtifactPath/Start/
// End, so only the transcript tier is derivable generically from a Window
// alone. See test-report.md Coverage Map for this scope decision.)

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultResolver_TranscriptFixture_ReturnsTranscriptSource — AC:
// "fixture Launch appends one llm-calls.ndjson record with source != 'none'"
// (scout verifiableBy). A resolver built by DefaultResolver, invoked with a
// Window matching a real on-disk transcript fixture, must recover the SAME
// usage ScanConfigRoot would (DefaultResolver must not re-implement or
// approximate the scan) and report SourceTranscript — never SourceNone.
func TestDefaultResolver_TranscriptFixture_ReturnsTranscriptSource(t *testing.T) {
	worktree := "/repo/worktrees/cycle-623"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo-worktrees-cycle-623")
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-08T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"start task"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-08T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":200,"output_tokens":40,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", sessionDir, err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "sess1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture transcript: %v", err)
	}

	resolver := DefaultResolver(root)
	if resolver == nil {
		t.Fatal("DefaultResolver returned a nil func — telemetry stays silently off")
	}
	res, err := resolver(Window{
		Worktree: worktree,
		Start:    mustParse(t, launchWindowStart),
		End:      mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatalf("resolver returned error for a valid fixture: %v", err)
	}
	if res.Source != SourceTranscript {
		t.Errorf("Source = %q, want %q (fixture transcript exists and matches the window)", res.Source, SourceTranscript)
	}
	if res.Source == SourceNone {
		t.Error("Source == SourceNone: a real transcript fixture must never resolve to the DI-off marker")
	}
}

// TestDefaultResolver_EmptyConfigRoot_ReturnsSourceNoneNotError — negative:
// a configRoot with no matching transcript (the common case — most launches
// have no Claude Code transcript to recover from) must fail OPEN: SourceNone,
// zero usage, nil error. It must never fabricate usage and must never error
// (token telemetry is documented best-effort; an error here would make a
// resolver failure visible to recordTokenUsage's WARN path for every launch,
// not just genuine failures).
func TestDefaultResolver_EmptyConfigRoot_ReturnsSourceNoneNotError(t *testing.T) {
	root := t.TempDir() // no projects/ dir at all

	resolver := DefaultResolver(root)
	res, err := resolver(Window{
		Worktree: "/repo/worktrees/cycle-999",
		Start:    mustParse(t, launchWindowStart),
		End:      mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatalf("resolver returned error for a root with no transcripts: %v (must fail open)", err)
	}
	if res.Source != SourceNone {
		t.Errorf("Source = %q, want %q for a root with no matching transcript", res.Source, SourceNone)
	}
}
