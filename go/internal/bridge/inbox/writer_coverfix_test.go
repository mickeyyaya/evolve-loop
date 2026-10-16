package inbox

import (
	"errors"
	"strings"
	"testing"
)

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteLine_ASurfacedWriteErrorIsWrappedNotSwallowed(t *testing.T) {
	t.Parallel()
	diskFull := errors.New("no space left on device")

	err := writeLine(failingWriter{err: diskFull}, []byte("x\n"))

	if !errors.Is(err, diskFull) {
		t.Fatalf("writeLine error = %v, want it to wrap %v", err, diskFull)
	}
	if !strings.HasPrefix(err.Error(), "inbox: write: ") {
		t.Fatalf("writeLine error = %q, want the inbox: write: prefix", err)
	}
}

func TestWriteLine_ASuccessfulWriteReturnsNil(t *testing.T) {
	t.Parallel()
	var sb strings.Builder

	if err := writeLine(&sb, []byte("line\n")); err != nil {
		t.Fatalf("writeLine error = %v, want nil", err)
	}
	if sb.String() != "line\n" {
		t.Fatalf("written = %q, want the line verbatim", sb.String())
	}
}
