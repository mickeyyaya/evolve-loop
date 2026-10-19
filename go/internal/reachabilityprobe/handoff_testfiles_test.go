package reachabilityprobe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHandoffTestFiles_ReturnsTheFilesWhetherOrNotTheyAreFrozen(t *testing.T) {
	cases := []struct {
		name       string
		frozenFlag string
		wantFrozen bool
	}{
		{"frozen", "true", true},
		{"unfrozen", "false", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := filepath.Join(t.TempDir(), "test-report.md")
			body := "## Handoff to Builder\n\n```json\n{\n  \"testFiles\": [\"go/acs/cycle1/predicates_test.go\"],\n  \"doNotModifyTests\": " + tc.frozenFlag + "\n}\n```\n"
			if err := os.WriteFile(report, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			all, err := HandoffTestFiles(report)
			if err != nil || len(all) != 1 || all[0] != "go/acs/cycle1/predicates_test.go" {
				t.Fatalf("HandoffTestFiles = %v, %v; want the one test file", all, err)
			}
			frozen, err := FrozenTestFiles(report)
			if err != nil || (len(frozen) == 1) != tc.wantFrozen {
				t.Fatalf("FrozenTestFiles = %v, %v; want frozen=%v", frozen, err, tc.wantFrozen)
			}
		})
	}
}

func TestHandoffTestFiles_MissingReportIsAnError(t *testing.T) {
	if _, err := HandoffTestFiles(filepath.Join(t.TempDir(), "absent.md")); err == nil {
		t.Fatal("HandoffTestFiles(absent) = nil error, want non-nil")
	}
}
