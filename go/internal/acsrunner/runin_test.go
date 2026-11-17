package acsrunner

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestRunIn_ChangesTheDirectoryOfGoTestAndRunKeepsTheCurrentOne(t *testing.T) {
	var argv []string
	capture := func(_ context.Context, _ []string, args ...string) (io.ReadCloser, func() error, error) {
		argv = args
		return &closingReader{strings.NewReader("")}, func() error { return nil }, nil
	}
	withCommander(capture, func() {
		_, _ = RunIn(context.Background(), "/tmp/tree/go", 7, "./acs/cycle7")
		if i := slices.Index(argv, "-C"); i != 1 || argv[2] != "/tmp/tree/go" || argv[3] != "test" {
			t.Errorf("RunIn args = %v, want go -C /tmp/tree/go test ...", argv)
		}
		_, _ = Run(context.Background(), 7, "./acs/cycle7")
		if slices.Contains(argv, "-C") {
			t.Errorf("Run args = %v, want no -C", argv)
		}
	})
}
