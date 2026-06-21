package guardcmd

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestApicover_HandlerSmoke exercises the registry-routed handlers that have no
// dedicated behavior test, via their safe help/no-arg early-return paths — this
// names each exported handler for the apicover gate and asserts each emits
// usage/guidance text without triggering real side effects (no git, no lint, no
// host probe is reached on these paths).
func TestApicover_HandlerSmoke(t *testing.T) {
	cases := []struct {
		name string
		run  func([]string, io.Reader, io.Writer, io.Writer) int
		args []string
	}{
		{"commit-gate", RunCommitGate, nil},                         // no subcommand → usage
		{"commit-prefix-gate", RunCommitPrefixGate, []string{"-h"}}, // help
		{"preflight", RunPreflight, []string{"-h"}},                 // help
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		_ = c.run(c.args, strings.NewReader(""), &out, &errb)
		if out.Len()+errb.Len() == 0 {
			t.Errorf("%s: expected usage/help output on the early-return path, got none", c.name)
		}
	}
}
