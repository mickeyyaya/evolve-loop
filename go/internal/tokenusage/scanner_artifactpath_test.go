package tokenusage

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestTranscriptScan_AttributesByArtifactPath_WhenCwdMispropagated(t *testing.T) {
	repoRoot := "/repo"                                  // the mis-propagated Window.Worktree
	transcriptCwd := "/repo/.evolve/worktrees/cycle-867" // where the CLI actually ran
	artifact := ".evolve/runs/cycle-867/build-report.md"

	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo--evolve-worktrees-cycle-867")
	body := `{"type":"user","cwd":"` + transcriptCwd + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"Artifact path: ` + artifact + `"}]}}
{"type":"assistant","cwd":"` + transcriptCwd + `/go","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":12,"output_tokens":7506,"cache_read_input_tokens":173121,"cache_creation_input_tokens":40}}}
`
	writeTranscript(t, sessionDir, "sess1.jsonl", body)

	w := Window{
		Worktree:     repoRoot,
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

func TestTranscriptScan_ArtifactPathNotInMessage_NotAttributed(t *testing.T) {
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo--evolve-worktrees-cycle-42")
	body := `{"type":"user","cwd":"/repo/.evolve/worktrees/cycle-42","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"write to .evolve/runs/cycle-42/OTHER-report.md"}]}}
{"type":"assistant","cwd":"/repo/.evolve/worktrees/cycle-42","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":9999,"output_tokens":9999,"cache_read_input_tokens":9999,"cache_creation_input_tokens":9999}}}
`
	writeTranscript(t, sessionDir, "sess1.jsonl", body)

	w := Window{
		Worktree:     "/repo/.evolve/worktrees/cycle-42", // matches the transcript cwd, yet must not attribute
		ArtifactPath: ".evolve/runs/cycle-42/build-report.md",
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

func TestTranscriptScan_AttributesByArtifactPath_StringContent(t *testing.T) {
	worktree := "/repo/.evolve/worktrees/cycle-500"
	artifact := "/repo/.evolve/runs/cycle-500/build-report.md"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo--evolve-worktrees-cycle-500")
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":"## Deliverable Contract\n\nArtifact path: ` + artifact + `"}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":9,"output_tokens":100,"cache_read_input_tokens":88000,"cache_creation_input_tokens":12}}}
`
	writeTranscript(t, sessionDir, "sess1.jsonl", body)

	// Worktree is arbitrary here: ArtifactPath alone decides attribution.
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
