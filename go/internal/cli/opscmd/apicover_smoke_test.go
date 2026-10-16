package opscmd

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestApicover_HandlerSmoke(t *testing.T) {
	cases := []struct {
		name string
		run  func([]string, io.Reader, io.Writer, io.Writer) int
		args []string
	}{
		{"doctor", RunDoctor, nil},
		{"release-consistency", RunReleaseConsistency, []string{"-h"}},
		{"version-bump", RunVersionBump, []string{"-h"}},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		_ = c.run(c.args, strings.NewReader(""), &out, &errb)
		if out.Len()+errb.Len() == 0 {
			t.Errorf("%s: expected usage/help output on the early-return path, got none", c.name)
		}
	}
}
