package sessionrecord_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/sessionrecord"
)

func TestReadAllSkipsMalformedLinesAndPreservesValidRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), sessionrecord.FileName)
	content := "" +
		`{"session":"s1","run_id":"run-1","cycle":299,"agent":"builder","pid":11,"created_at":"2026-06-12T00:00:00Z"}` + "\n" +
		`{not-json` + "\n" +
		`{"session":"s2","run_id":"run-2","cycle":300,"agent":"auditor","pid":22,"created_at":"2026-06-12T00:01:00Z"}` + "\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	records, err := sessionrecord.ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll returned error for malformed JSONL line: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("ReadAll returned %d records, want 2: %+v", len(records), records)
	}
	if records[0].Session != "s1" || records[0].RunID != "run-1" || records[0].Cycle != 299 || records[0].Agent != "builder" || records[0].PID != 11 {
		t.Fatalf("first record = %+v, want first valid line decoded", records[0])
	}
	if records[1].Session != "s2" || records[1].RunID != "run-2" || records[1].Cycle != 300 || records[1].Agent != "auditor" || records[1].PID != 22 {
		t.Fatalf("second record = %+v, want second valid line decoded after malformed line", records[1])
	}
}

func TestRunScopeTokenBoundaries(t *testing.T) {
	tests := []struct {
		name string
		run  string
		want string
	}{
		{name: "empty", run: "", want: "r"},
		{name: "exactly eight", run: "12345678", want: "r12345678"},
		{name: "truncates after eight", run: "123456789abcdef", want: "r12345678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sessionrecord.RunScopeToken(tt.run); got != tt.want {
				t.Fatalf("RunScopeToken(%q) = %q, want %q", tt.run, got, tt.want)
			}
		})
	}
}
