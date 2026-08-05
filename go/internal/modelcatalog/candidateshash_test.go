package modelcatalog

import (
	"testing"
	"time"
)

// TestBuildFromSnapshots_CarriesCandidatesHash: the snapshot's CandidatesHash
// flows into the built entry verbatim — BuildFromSnapshots stays a pure
// wholesale rebuild with no special case, and an absent hash stays absent
// (omitempty) so every pre-existing catalog degrades safely to classifying
// once.
func TestBuildFromSnapshots_CarriesCandidatesHash(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	cat := BuildFromSnapshots([]CLISnapshot{
		{
			CLI: "codex", Ready: true, Source: SourceLive,
			TierModels:     map[string]string{"deep": "gpt-5.5"},
			CandidatesHash: "sha256:abc",
		},
		{
			CLI: "agy", Ready: true, Source: SourceDetect,
			TierModels: map[string]string{"deep": "Gemini 3.5 Pro (High)"},
		},
	}, now)
	if got := cat.CLIs["codex"].CandidatesHash; got != "sha256:abc" {
		t.Errorf("codex CandidatesHash = %q, want sha256:abc", got)
	}
	if got := cat.CLIs["agy"].CandidatesHash; got != "" {
		t.Errorf("agy CandidatesHash = %q, want empty (absent stays absent)", got)
	}
}
