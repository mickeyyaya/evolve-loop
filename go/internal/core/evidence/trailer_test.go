package evidence

import "testing"

func TestTrailer_BuildParseRoundTrip(t *testing.T) {
	in := Trailer{
		Phase:        "scout",
		Cycle:        118,
		Challenge:    "tok-abc123",
		ArtifactSHA:  "deadbeef",
		ArtifactPath: ".evolve/runs/cycle-118/scout-report.md",
	}
	msg := "feat: scout discovery\n\nfound 3 tasks\n" + in.Build()
	got := Parse(msg)
	if got != in {
		t.Fatalf("round-trip mismatch:\n got=%+v\nwant=%+v", got, in)
	}
}

func TestTrailer_Build_OmitsEmptyAndZeroCycle(t *testing.T) {
	got := Trailer{Phase: "audit", Challenge: "t"}.Build()
	for _, absent := range []string{KeyCycle, KeyArtifactSHA, KeyArtifactPath} {
		if contains(got, absent) {
			t.Errorf("build should omit empty %s; got:\n%s", absent, got)
		}
	}
	if !contains(got, "Evolve-Phase: audit") || !contains(got, "Challenge-Token: t") {
		t.Errorf("build missing required fields; got:\n%s", got)
	}
}

func TestParse_OnlyTrailingBlock(t *testing.T) {
	// A body line with a colon ("Note: ...") that is NOT in the trailing block
	// must not be parsed as a trailer; only the contiguous trailing run counts.
	msg := "fix: thing\n\nNote: this mentions Evolve-Phase: build in prose\n\n" +
		Trailer{Phase: "scout", Challenge: "real"}.Build()
	got := Parse(msg)
	if got.Phase != "scout" || got.Challenge != "real" {
		t.Fatalf("parsed the wrong block: %+v (want scout/real from the trailing trailer)", got)
	}
}

func TestParse_NoTrailerYieldsZero(t *testing.T) {
	if got := Parse("just a plain message\nwith no trailers\n"); got.Phase != "" {
		t.Fatalf("plain message should yield zero Trailer; got %+v", got)
	}
}

func TestParse_BlankLineTerminatesTrailer(t *testing.T) {
	// Contract (git-trailer spec): a blank line terminates the trailing run, so
	// a trailer block split by a blank line only parses the contiguous TAIL.
	// Build() never emits such a block; this pins the boundary so PR3's emitter
	// honors it. Here the tail after the blank lacks Evolve-Phase → Phase == "".
	msg := "subject\n\nEvolve-Phase: scout\n\nChallenge-Token: tok\n"
	got := Parse(msg)
	if got.Phase != "" {
		t.Fatalf("a blank line must terminate the trailer run; got Phase=%q (want empty)", got.Phase)
	}
	if got.Challenge != "tok" {
		t.Fatalf("the contiguous tail (Challenge) should still parse; got %+v", got)
	}
}

func TestParse_ProseKeyLineFlushAgainstTrailerIsAbsorbed(t *testing.T) {
	// Known limitation of the contiguous-run heuristic: a prose line with valid
	// key grammar ("Added: ...") flush against the trailer is absorbed into the
	// block. Harmless — unknown keys are ignored, so Phase/Challenge still parse.
	msg := "subject\n\nAdded: a feature\nEvolve-Phase: scout\nChallenge-Token: tok\n"
	got := Parse(msg)
	if got.Phase != "scout" || got.Challenge != "tok" {
		t.Fatalf("the verifying keys must still parse despite an absorbed prose key; got %+v", got)
	}
}

func TestTrailer_Verify_FailClosed(t *testing.T) {
	tr := Trailer{Phase: "build", Challenge: "tok"}
	cases := []struct {
		name         string
		phase, token string
		want         bool
	}{
		{"match", "build", "tok", true},
		{"wrong phase", "audit", "tok", false},
		{"wrong token", "build", "nope", false},
		{"empty token rejected", "build", "", false},
		{"empty phase rejected", "", "tok", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := tr.Verify(c.phase, c.token); got != c.want {
				t.Errorf("Verify(%q,%q) = %v, want %v", c.phase, c.token, got, c.want)
			}
		})
	}
	// A zero-token commit can never be verified even if phase matches.
	if (Trailer{Phase: "build"}).Verify("build", "tok") {
		t.Error("a commit with no Challenge-Token must never verify")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
