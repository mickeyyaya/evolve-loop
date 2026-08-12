package deliverable

import "testing"

// salvage_keycase_test.go — regression for the ambiguity guard's blind spot on
// verdict keys the RE-VERIFY pass reads but the byte-literal count did not:
// encoding/json matches struct fields case-insensitively and decodes \uXXXX
// escapes, so `"Verdict":` / `"VERDICT":` / `"verdict":` are all verdict
// keys to the decoder. Counting them with a case-sensitive byte literal made a
// decoy invisible to the guard while remaining actionable by the decoder
// (cycle-1432 audit d4fa6591dcd07c365884c64925a8e3dbe, CRITICAL C1).
//
// The assertion is on candidateCount — the guard's own decision input — and it
// is directional: every case here must count MORE than one candidate, because
// candidateCount is consulted only as `> 1 ⇒ refuse`. Widening a count can add
// refusals and never remove one.
func TestCandidateCount_CountsDecoderVisibleVerdictKeys(t *testing.T) {
	const real = `{"phase":"audit","verdict":"FAIL"}`

	tests := []struct {
		name  string
		decoy string
	}{
		{"capitalized", `{"phase":"audit","Verdict":"PASS"}`},
		{"upper", `{"phase":"audit","VERDICT":"PASS"}`},
		{"mixed", `{"phase":"audit","VeRdIcT":"PASS"}`},
		// A literal backslash-u escape of 'v' — what a report author writes to
		// hide the key from a byte-literal count while encoding/json still
		// decodes it to "verdict".
		{"unicode-escaped", `{"phase":"audit","` + `\` + `u0076erdict":"PASS"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := "## Verdict\n\n" + tc.decoy + "\n\n" + real + "\n"
			if got := candidateCount(content); got <= 1 {
				t.Errorf("candidateCount = %d for a decoy the re-verify decoder WILL read as a verdict; the guard must see at least 2 candidates and refuse\ncontent:\n%s", got, content)
			}
		})
	}
}

// TestCandidateCount_SoleCanonicalVerdictStillCountsOne is the discriminator:
// the widened count must not turn every ordinary sole-verdict deliverable into
// a refusal. Without this, the test above is satisfiable by counting anything.
func TestCandidateCount_SoleCanonicalVerdictStillCountsOne(t *testing.T) {
	const soleFenced = "## Verdict\n" +
		"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"
	if got := candidateCount(soleFenced); got != 1 {
		t.Errorf("candidateCount = %d, want 1 — a sole canonical verdict must stay salvageable", got)
	}
}

// TestUnescapeJSONShort_LeavesNonASCIIAndPlainTextAlone pins the narrow
// contract of the normalizer: it rewrites only ASCII \uXXXX escapes (the ones
// that can spell a key), and is a no-op on everything else.
func TestUnescapeJSONShort_LeavesNonASCIIAndPlainTextAlone(t *testing.T) {
	tests := []struct{ in, want string }{
		{`no escapes here`, `no escapes here`},
		{`"verdict":`, `"verdict":`},
		{`"éclair"`, `"éclair"`}, // non-ASCII: left as written
		{`"\uZZZZ"`, `"\uZZZZ"`}, // malformed: left as written
		{`trailing backslash \u`, `trailing backslash \u`},
	}
	for _, tc := range tests {
		if got := unescapeJSONShort(tc.in); got != tc.want {
			t.Errorf("unescapeJSONShort(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
