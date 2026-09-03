package phasecontract

import "testing"

// TestRoundArchiveFilename pins the archive rule the writer and both readers
// share: stem + ".round<N>" + original extension, for the two artifacts the
// audit-repair loop retires.
func TestRoundArchiveFilename(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		round int
		want  string
	}{
		{ArtifactFilename("audit"), 1, "audit-report.round1.md"},
		{"acs-verdict.json", 2, "acs-verdict.round2.json"},
		{"noext", 3, "noext.round3"},
	}
	for _, c := range cases {
		if got := RoundArchiveFilename(c.name, c.round); got != c.want {
			t.Errorf("RoundArchiveFilename(%q, %d) = %q, want %q", c.name, c.round, got, c.want)
		}
	}
}

// TestParseRoundArchive pins the inverse: round-trips every archive name,
// rejects the live file, other artifacts, padded or non-numeric indices.
func TestParseRoundArchive(t *testing.T) {
	t.Parallel()
	live := ArtifactFilename("audit")
	for _, n := range []int{1, 2, 12} {
		got, ok := ParseRoundArchive(RoundArchiveFilename(live, n), live)
		if !ok || got != n {
			t.Errorf("round-trip %d = (%d,%v)", n, got, ok)
		}
	}
	for _, bad := range []string{live, "audit-report.round.md", "audit-report.round01.md", "audit-report.roundx.md", "acs-verdict.round1.json", "audit-report.round1.md.bak"} {
		if _, ok := ParseRoundArchive(bad, live); ok {
			t.Errorf("ParseRoundArchive(%q) must not match", bad)
		}
	}
}

func TestPromptArtifactFilename_IsTheBridgeRule(t *testing.T) {
	t.Parallel()
	if got := PromptArtifactFilename("build"); got != "build-prompt.txt" {
		t.Fatalf("PromptArtifactFilename(build) = %q", got)
	}
	if got := RoundArchiveFilename(PromptArtifactFilename("tdd"), 2); got != "tdd-prompt.round2.txt" {
		t.Fatalf("archived prompt name = %q", got)
	}
}
