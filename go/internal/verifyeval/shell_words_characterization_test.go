package verifyeval

import (
	"slices"
	"testing"
)

func TestShellWordsCharacterization(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{name: "empty", command: "", want: nil},
		{name: "blank", command: " \t\r\n", want: nil},
		{name: "repeated spaces", command: " a  b ", want: []string{"a", "b"}},
		{name: "tab and carriage return separate", command: "a\tb\rc", want: []string{"a", "b", "c"}},
		{name: "backslash literal in single quotes", command: `'a\b'`, want: []string{`a\b`}},
		{name: "backslash escapes in double quotes", command: `"a\"b"`, want: []string{`a"b`}},
		{name: "single-quoted space", command: `'a b'`, want: []string{"a b"}},
		{name: "quoted edge whitespace kept", command: `" a "`, want: []string{" a "}},
		{name: "escaped space", command: `a\ b`, want: []string{"a b"}},
		{name: "escaped newline joins", command: "go\\\ntest", want: []string{"gotest"}},
		{name: "empty quotes are a word", command: `x '' ""`, want: []string{"x", "", ""}},
		{name: "quoted flag", command: `go test "-run=Test X" './a b'`, want: []string{"go", "test", "-run=Test X", "./a b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shellWords(tt.command); !slices.Equal(got, tt.want) {
				t.Errorf("shellWords(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}
