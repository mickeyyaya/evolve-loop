package tokenusage

// scanner_artifactpath_test.go — RED contract for the production token-telemetry
// attribution defect (2026-07-17). On the default tmux-LLM driver path a claude
// phase launch's transcript was NEVER attributed: attributes() hard-gated on an
// exact cwd == Window.Worktree match, but Worktree is lossy across the exec
// boundary (WORKTREE_PATH → ProjectRoot fallback ≠ the transcript's recorded
// worktree cwd). So the one tier carrying input/cache_read tokens (the
// transcript) fell through to scrollback_peak → input:0, cache_read:0, hiding
// the ~173K-token/turn context-window cost. The launch-unique ArtifactPath the
// general bridge Window already carries appears verbatim in exactly that
// launch's first user message, so it is the reliable attribution key.

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// TestTranscriptScan_AttributesByArtifactPath_WhenCwdMispropagated reproduces
// the cycle-867 production failure: the Window.Worktree the resolver received
// (the repo root, from the WORKTREE_PATH→ProjectRoot fallback) does NOT equal
// the transcript's recorded cwd (the real cycle worktree, and its /go subdir).
// The launch-unique ArtifactPath in the first user message must attribute the
// transcript anyway, recovering the real input + cache_read the exact-cwd gate
// discarded.
func TestTranscriptScan_AttributesByArtifactPath_WhenCwdMispropagated(t *testing.T) {
	repoRoot := "/repo"                                  // the mis-propagated Window.Worktree
	transcriptCwd := "/repo/.evolve/worktrees/cycle-867" // where the CLI actually ran
	artifact := ".evolve/runs/cycle-867/build-report.md"

	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo--evolve-worktrees-cycle-867")
	body := `{"type":"user","cwd":"` + transcriptCwd + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"write your report to ` + artifact + `"}]}}
{"type":"assistant","cwd":"` + transcriptCwd + `/go","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":12,"output_tokens":7506,"cache_read_input_tokens":173121,"cache_creation_input_tokens":40}}}
`
	writeTranscript(t, sessionDir, "sess1.jsonl", body)

	w := Window{
		Worktree:     repoRoot, // deliberately NOT the transcript cwd
		ArtifactPath: artifact,
		Start:        mustParse(t, launchWindowStart),
		End:          mustParse(t, launchWindowEnd),
	}
	res, err := ScanConfigRoot(root, w)
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	if res.Source != SourceTranscript {
		t.Fatalf("Source = %q, want %q — ArtifactPath must attribute the transcript even when cwd != Worktree (the production mis-propagation)", res.Source, SourceTranscript)
	}
	want := cyclestate.TokenUsage{Input: 12, Output: 7506, CacheRead: 173121, CacheWrite: 40}
	if res.Usage != want {
		t.Errorf("Usage = %+v, want %+v — the context-window cost (cache_read) must be recovered, not zeroed", res.Usage, want)
	}
}

// TestTranscriptScan_ArtifactPathNotInMessage_NotAttributed guards against
// over-attribution: a transcript whose first user message does NOT contain the
// launch's ArtifactPath must be excluded even if it is the only session under
// the config root — ArtifactPath is a unique key, not a coarse hint. (Passes
// under the old code too; it locks the new behavior against regression.)
func TestTranscriptScan_ArtifactPathNotInMessage_NotAttributed(t *testing.T) {
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo--evolve-worktrees-cycle-42")
	body := `{"type":"user","cwd":"/repo/.evolve/worktrees/cycle-42","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"write to .evolve/runs/cycle-42/OTHER-report.md"}]}}
{"type":"assistant","cwd":"/repo/.evolve/worktrees/cycle-42","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":9999,"output_tokens":9999,"cache_read_input_tokens":9999,"cache_creation_input_tokens":9999}}}
`
	writeTranscript(t, sessionDir, "sess1.jsonl", body)

	w := Window{
		Worktree:     "/repo/.evolve/worktrees/cycle-42",      // even a correct cwd match...
		ArtifactPath: ".evolve/runs/cycle-42/build-report.md", // ...must not attribute without the artifact
		Start:        mustParse(t, launchWindowStart),
		End:          mustParse(t, launchWindowEnd),
	}
	res, err := ScanConfigRoot(root, w)
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	if res.Source != SourceNone {
		t.Errorf("Source = %q, want %q — a transcript lacking the launch's ArtifactPath must not be attributed", res.Source, SourceNone)
	}
}

// TestTranscriptScan_AttributesByArtifactPath_StringContent — the first user
// message's content is a BARE JSON STRING (the common Claude Code transcript
// form for the phase prompt), not an array of {type,text} blocks. firstUserText
// must decode the string form; otherwise the ArtifactPath key never matches and
// attribution silently fails. This is the production blind spot that made the
// prior ArtifactPath-primary fix (c41fa94b) inert: unit fixtures used the block
// form, real transcripts use the string form, so cache_read read as zero.
func TestTranscriptScan_AttributesByArtifactPath_StringContent(t *testing.T) {
	worktree := "/repo/.evolve/worktrees/cycle-500"
	artifact := "/repo/.evolve/runs/cycle-500/build-report.md"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo--evolve-worktrees-cycle-500")
	// content is a JSON STRING (not an array of blocks), as real transcripts emit:
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":"## Deliverable Contract\n\nWrite it to the EXACT path: ` + artifact + `"}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":9,"output_tokens":100,"cache_read_input_tokens":88000,"cache_creation_input_tokens":12}}}
`
	writeTranscript(t, sessionDir, "sess1.jsonl", body)

	// Worktree is irrelevant to this test — ArtifactPath governs attribution
	// unconditionally; set to an arbitrary value for parity with sibling fixtures.
	w := Window{
		Worktree:     "/repo",
		ArtifactPath: artifact,
		Start:        mustParse(t, launchWindowStart),
		End:          mustParse(t, launchWindowEnd),
	}
	res, err := ScanConfigRoot(root, w)
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	if res.Source != SourceTranscript {
		t.Fatalf("Source = %q, want %q — bare-string content must be read so ArtifactPath attributes", res.Source, SourceTranscript)
	}
	want := cyclestate.TokenUsage{Input: 9, Output: 100, CacheRead: 88000, CacheWrite: 12}
	if res.Usage != want {
		t.Errorf("Usage = %+v, want %+v", res.Usage, want)
	}
}
