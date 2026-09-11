package verifyeval

import "testing"

func TestHasNarrowedGoTest_AttributesSelectorToGoCommand(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{name: "short selector", script: "go test -run TestPresent ./...", want: true},
		{name: "long selector", script: "go test --run=TestPresent ./...", want: true},
		{name: "quoted selector", script: "go test \"-run=TestPresent\" ./...", want: true},
		{name: "absolute go path", script: "/usr/local/go/bin/go test -run TestPresent ./...", want: true},
		{name: "quoted punctuation", script: "go test -ldflags '-X main.label=a;b' -run TestPresent ./...", want: true},
		{name: "redirection before selector", script: "go test 2>&1 -run TestPresent ./...", want: true},
		{name: "captured output", script: "output=$(go test -run TestPresent ./...); printf '%s\\n' \"$output\"", want: true},
		{name: "quoted captured output", script: "output=\"$(go test -run TestPresent ./...)\"; printf '%s\\n' \"$output\"", want: true},
		{name: "single-quoted substitution", script: "printf '%s\\n' '$(go test -run TestPresent ./...)'", want: false},
		{name: "substitution prints example", script: "output=\"$(printf '%s' 'go test -run TestPresent')\"", want: false},
		{name: "commented substitution", script: "# output=\"$(go test -run TestPresent ./...)\"\ngo test ./...", want: false},
		{name: "continued go command", script: "go test \\\n-run TestPresent ./...", want: true},
		{name: "later command argument", script: "go test ./...; printf '%s\\n' -run", want: false},
		{name: "next line argument", script: "go test ./...\nprintf '%s\\n' -run", want: false},
		{name: "quoted example", script: "printf '%s\\n' 'go test -run TestPresent'", want: false},
		{name: "commented example", script: "# go test -run TestPresent\ngo test ./...", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hasNarrowedGoTest(test.script); got != test.want {
				t.Fatalf("hasNarrowedGoTest(%q) = %v, want %v", test.script, got, test.want)
			}
		})
	}
}
