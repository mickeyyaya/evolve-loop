package phasecontract

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestParseVerdictSentinelFull_TrailingToleranceKeepsTailAnchor(t *testing.T) {
	doc := "prose quoting an example: <!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n" +
		"real tail verdict: <!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\",\"schema_version\":1}} -->\n"
	s, ok := ParseVerdictSentinelFull(doc)
	if !ok || s.Verdict != "FAIL" {
		t.Fatalf("tail sentinel with stray brace must win over the earlier decoy; got (%q,%v)", s.Verdict, ok)
	}
}
