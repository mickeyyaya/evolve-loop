package topngate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTDDScopeGate_HandoffDeclaration(t *testing.T) {
	for _, tc := range []struct {
		name, header, handoff string
		approve               bool
	}{
		{"JSON omission overrides complete header", "alpha, beta", `{"slugs":["alpha"],"testFiles":["real_test.go"]}`, false},
		{"JSON complete overrides single label", "alpha", `{"slugs":["beta","alpha"],"testFiles":["real_test.go"]}`, true},
		{"extra member", "alpha, beta", `{"slugs":["alpha","beta","extra"],"testFiles":["real_test.go"]}`, false},
		{"explicit empty declaration", "alpha, beta", `{"slugs":[],"testFiles":["real_test.go"]}`, false},
		{"header fallback", "alpha, beta", `{"testFiles":["real_test.go"]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			writeTriageReport(t, ws, "alpha", "beta")
			body := "## Task: " + tc.header + "\n## Handoff to Builder\n```json\n" + tc.handoff + "\n```\n"
			if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			got := reviewTDD(t, ws)
			if got.Approve != tc.approve || (!tc.approve && !strings.Contains(got.Reason, "scope-mismatch")) {
				t.Fatalf("review = %+v, want approve=%v", got, tc.approve)
			}
		})
	}
}

func TestHandoffTestFiles_IgnoresExamples(t *testing.T) {
	body := "~~~markdown\n## Handoff to Builder\n```json\n{\"testFiles\":[\"fake.go\"]}\n```\n~~~\n## Handoff to Builder\n````json\n{\"testFiles\":[\"real.go\"]}\n````\n"
	_, _, got := parseTDDReport(body)
	if len(got) != 1 || got[0] != "real.go" {
		t.Fatalf("example mistaken for handoff: %v", got)
	}
}
