package phasecontract

import (
	"os"
	"path/filepath"
	"testing"
)

// Cycle-1478 (batch-20260815c): the audit agent emitted a sentinel whose JSON
// payload was valid for its first complete value, followed by ONE stray '}'.
// sentinelRE's non-greedy capture extends to the last '}' before '-->', so the
// capture carried the stray byte and the whole-string json.Unmarshal rejected
// it ("invalid character '}' after top-level value"). At contract-gate enforce
// the prose fallback is gated off (ADR-0050 §3.10 Slice 5), so a one-byte
// formatting slip blocked three times (two correction re-dispatches, the
// second a salvage retry), opened the contract-gate circuit, and ended in an
// ADR-0072 verdict-incoherence HALT — while prose, sentinel, and
// acs-verdict.json all agreed on WARN.
//
// The fix: parse the LEADING complete JSON value of the captured payload and
// tolerate trailing bytes INSIDE the comment-bounded capture. Every existing
// guard is unchanged: tail-anchored candidate selection, the placeholder-echo
// rejection, the verdict-vocabulary check, and rejection of captures whose
// leading value is not a verdict-bearing object.

// The real production artifact, byte-for-byte.
func TestParseVerdictSentinelFull_Cycle1478RealArtifact(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "cycle1478-trailing-brace.md"))
	if err != nil {
		t.Fatal(err)
	}
	s, ok := ParseVerdictSentinelFull(string(raw))
	if !ok {
		t.Fatal("cycle-1478 artifact must parse: its sentinel JSON is complete and verdict-bearing; only a stray trailing '}' follows it")
	}
	if s.Verdict != "WARN" || s.Phase != "audit" {
		t.Errorf("got (phase=%q verdict=%q), want (audit, WARN)", s.Phase, s.Verdict)
	}
	if s.Failure == nil || s.Failure.Class != "code-audit-fail" {
		t.Errorf("failure block must survive the trailing-byte tolerance; got %+v", s.Failure)
	}
}

func TestParseVerdictSentinelFull_TrailingDataInsideComment(t *testing.T) {
	valid := `{"phase":"audit","verdict":"WARN","schema_version":1}`
	cases := []struct {
		name    string
		content string
		ok      bool
		verdict string
	}{
		{"one stray brace", "<!-- evolve-verdict: " + valid + "} -->", true, "WARN"},
		{"two stray braces", "<!-- evolve-verdict: " + valid + "}} -->", true, "WARN"},
		{"junk ending in brace", "<!-- evolve-verdict: " + valid + " junk} -->", true, "WARN"},
		// Over-acceptance guards: a leading value that is not a verdict-bearing
		// object must still be rejected, trailing tolerance or not.
		{"empty object then junk", "<!-- evolve-verdict: {}garbage} -->", false, ""},
		{"broken leading value", "<!-- evolve-verdict: {not json}} -->", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, ok := ParseVerdictSentinelFull(c.content)
			if ok != c.ok || s.Verdict != c.verdict {
				t.Errorf("got (%q,%v), want (%q,%v)", s.Verdict, ok, c.verdict, c.ok)
			}
		})
	}
}

// Tail-anchoring is unchanged by trailing tolerance: the LAST valid comment
// still wins, and a trailing-brace defect on the tail sentinel no longer
// knocks the read back to an earlier quoted decoy... unless the tail is
// genuinely unparseable, in which case the walk continues exactly as before.
func TestParseVerdictSentinelFull_TrailingToleranceKeepsTailAnchor(t *testing.T) {
	doc := "prose quoting an example: <!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n" +
		"real tail verdict: <!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\",\"schema_version\":1}} -->\n"
	s, ok := ParseVerdictSentinelFull(doc)
	if !ok || s.Verdict != "FAIL" {
		t.Fatalf("tail sentinel with stray brace must win over the earlier decoy; got (%q,%v)", s.Verdict, ok)
	}
}
