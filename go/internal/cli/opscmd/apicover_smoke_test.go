package opscmd

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestApicover_HandlerSmoke exercises the registry-routed handlers that have no
// dedicated behavior test, via their safe no-arg/help early-return paths — this
// names each exported handler for the apicover gate and asserts each emits
// usage/guidance text without performing real release/doctor side effects.
func TestApicover_HandlerSmoke(t *testing.T) {
	cases := []struct {
		name string
		run  func([]string, io.Reader, io.Writer, io.Writer) int
		args []string
	}{
		{"doctor", RunDoctor, nil},                                     // no subcommand → usage
		{"release-consistency", RunReleaseConsistency, []string{"-h"}}, // help
		{"version-bump", RunVersionBump, []string{"-h"}},               // help
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		_ = c.run(c.args, strings.NewReader(""), &out, &errb)
		if out.Len()+errb.Len() == 0 {
			t.Errorf("%s: expected usage/help output on the early-return path, got none", c.name)
		}
	}
}
