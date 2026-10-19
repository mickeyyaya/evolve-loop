package reachabilityprobe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHandoffTestFiles_SkipsUnusableBlocksAndFindsNoneWhenAbsent(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"no json block", "# report\n", nil},
		{"malformed block then valid", "```json\n{not json\n```\n```json\n{\"testFiles\":[\"a_test.go\"]}\n```\n", []string{"a_test.go"}},
		{"block without testFiles then valid", "```json\n{\"other\":1}\n```\n```json\n{\"testFiles\":[\"b_test.go\"]}\n```\n", []string{"b_test.go"}},
		{"only block without testFiles", "```json\n{\"other\":1}\n```\n", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := filepath.Join(t.TempDir(), "test-report.md")
			if err := os.WriteFile(report, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := HandoffTestFiles(report)
			if err != nil || len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
				t.Fatalf("HandoffTestFiles = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}
