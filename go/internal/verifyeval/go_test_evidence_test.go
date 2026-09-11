package verifyeval

import "testing"

func TestExecutionEvidenceReason(t *testing.T) {
	tests := []struct {
		name   string
		result CommandResult
		want   string
	}{
		{
			name: "local warning on stderr",
			result: CommandResult{
				Command: "go test -run TestMissing",
				Stdout:  "PASS\nok  example.com/pkg  0.1s\n",
				Stderr:  noTestsWarning,
			},
			want: "narrowed go test matched no tests",
		},
		{
			name:   "package has no test files",
			result: CommandResult{Command: "go test -run TestMissing ./empty", Stdout: "?  example.com/empty  [no test files]\n"},
			want:   "narrowed go test matched no tests",
		},
		{
			name: "recursive command has matching package",
			result: CommandResult{
				Command: "go test -run TestPresent ./...",
				Stdout: "ok  example.com/pkg  0.1s\n" +
					"ok  example.com/empty  0.1s [no tests to run]\n",
			},
		},
		{
			name: "JSON event proves named test execution",
			result: CommandResult{
				Command: "go test -json -run TestPresent ./...",
				Stdout:  "{\"Action\":\"run\",\"Package\":\"example.com/pkg\",\"Test\":\"TestPresent\"}\n",
				Stderr:  noTestsWarning,
			},
		},
		{
			name:   "wide command may compile empty package",
			result: CommandResult{Command: "go test ./empty", Stdout: "?  example.com/empty  [no test files]\n"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := executionEvidenceReason(test.result); got != test.want {
				t.Fatalf("executionEvidenceReason() = %q, want %q", got, test.want)
			}
		})
	}
}
