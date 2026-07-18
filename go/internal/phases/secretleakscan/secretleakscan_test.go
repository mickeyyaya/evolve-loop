package secretleakscan

import "testing"

func TestScanDiff_CleanDiffNoFindings(t *testing.T) {
	clean := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n" +
		"@@ -1,2 +1,3 @@\n package foo\n+\n+var answer = 42\n"
	if f := ScanDiff(clean); len(f) != 0 {
		t.Errorf("clean diff produced %d finding(s), want 0: %+v", len(f), f)
	}
}

func TestScanDiff_DetectsPlantedSecrets(t *testing.T) {
	cases := []struct {
		name, line, wantRule string
	}{
		{"pem", "+const k = \"-----BEGIN RSA PRIVATE KEY-----\"", "pem-private-key"},
		{"pem-openssh", "+key: -----BEGIN OPENSSH PRIVATE KEY-----", "pem-private-key"},
		{"aws", "+const id = \"AKIAIOSFODNN7EXAMPLE\"", "aws-access-key-id"},
		{"github", "+token = ghp_0123456789abcdefghijklmnopqrstuvwxyz", "github-token"},
		{"slack", "+t := \"xoxb-1234567890-abcdefghij\"", "slack-token"},
		{"assign", "+api_key = \"abcdefghijklmnopqrstuvwxyz0123456789\"", "generic-private-key-assign"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diff := "+++ b/x\n" + tc.line + "\n"
			got := ScanDiff(diff)
			if len(got) == 0 {
				t.Fatalf("no findings for %q", tc.line)
			}
			found := false
			for _, f := range got {
				if f.Rule == tc.wantRule {
					found = true
					if f.Match == "" {
						t.Errorf("finding has empty Match: %+v", f)
					}
				}
			}
			if !found {
				t.Errorf("rule %q did not fire; got %+v", tc.wantRule, got)
			}
		})
	}
}

func TestScanDiff_IgnoresRemovedAndContextAndHeaderLines(t *testing.T) {
	// A secret on a removed line, a context line, and the +++ header must all be ignored.
	diff := "+++ b/AKIAIOSFODNN7EXAMPLE\n" + // header, even if it looks like a key
		"-const old = \"AKIAIOSFODNN7EXAMPLE\"\n" + // removed
		" const ctx = \"AKIAIOSFODNN7EXAMPLE\"\n" // context
	if f := ScanDiff(diff); len(f) != 0 {
		t.Errorf("removed/context/header lines produced findings: %+v", f)
	}
}

func TestScanDiff_CRLFScansIdenticallyToLF(t *testing.T) {
	lf := "+const id = \"AKIAIOSFODNN7EXAMPLE\"\n"
	crlf := "+const id = \"AKIAIOSFODNN7EXAMPLE\"\r\n"
	if a, b := ScanDiff(lf), ScanDiff(crlf); len(a) != len(b) || len(a) != 1 {
		t.Errorf("CRLF (%d) and LF (%d) scans differ or missed the key", len(b), len(a))
	}
}

func TestScanDiff_Deterministic(t *testing.T) {
	diff := "+const id = \"AKIAIOSFODNN7EXAMPLE\"\n+key = ghp_0123456789abcdefghijklmnopqrstuvwxyz\n"
	a, b := ScanDiff(diff), ScanDiff(diff)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic finding count: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestVerdict(t *testing.T) {
	if got := Verdict(nil); got != "PASS" {
		t.Errorf("Verdict(nil) = %q, want PASS", got)
	}
	if got := Verdict([]Finding{{Rule: "aws-access-key-id", Match: "AKIA..."}}); got != "FAIL" {
		t.Errorf("Verdict(one) = %q, want FAIL", got)
	}
}
