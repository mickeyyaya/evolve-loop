package ship

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"testing"
)

func TestRunGoTestJSON_AStdoutPipeFailureNeverStartsTheProcess(t *testing.T) {
	prev := stdoutPipe
	t.Cleanup(func() { stdoutPipe = prev })
	cause := errors.New("pipe table full")
	var seen *exec.Cmd
	stdoutPipe = func(cmd *exec.Cmd) (io.ReadCloser, error) {
		seen = cmd
		return nil, cause
	}
	var notes, raw bytes.Buffer

	out := runGoTestJSON(context.Background(), t.TempDir(), packLog{notes: &notes, raw: &raw}, []string{"test", "-json", "./..."})

	if !errors.Is(out.err, cause) || out.err.Error() != "go test stdout pipe: pipe table full" {
		t.Fatalf("err = %v, want go test stdout pipe: pipe table full", out.err)
	}
	if out.failures != nil || out.failureLog != "" {
		t.Errorf("outcome = %+v, want no failures and no failure log", out)
	}
	if seen == nil || seen.Process != nil {
		t.Errorf("cmd = %+v, want a command that was never started", seen)
	}
	if raw.Len() != 0 || notes.Len() != 0 {
		t.Errorf("log raw=%q notes=%q, want nothing written", raw.String(), notes.String())
	}
}
