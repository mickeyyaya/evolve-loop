package deliverable

import "testing"

func TestCandidateCount_CountsDecoderVisibleVerdictKeys(t *testing.T) {
	const real = `{"phase":"audit","verdict":"FAIL"}`

	tests := []struct {
		name  string
		decoy string
	}{
		{"capitalized", `{"phase":"audit","Verdict":"PASS"}`},
		{"upper", `{"phase":"audit","VERDICT":"PASS"}`},
		{"mixed", `{"phase":"audit","VeRdIcT":"PASS"}`},
		// A literal \u escape of 'v': invisible to a byte-literal count, decoded to "verdict" by encoding/json.
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

func TestCandidateCount_SoleCanonicalVerdictStillCountsOne(t *testing.T) {
	const soleFenced = "## Verdict\n" +
		"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"
	if got := candidateCount(soleFenced); got != 1 {
		t.Errorf("candidateCount = %d, want 1 — a sole canonical verdict must stay salvageable", got)
	}
}

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
