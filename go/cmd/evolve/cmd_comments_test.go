package main

import (
	"bytes"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
)

func TestRunComments_IsCommentauditMain(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"bogus"}} {
		var evolveOut, evolveErr, toolOut, toolErr bytes.Buffer
		got := runComments(args, nil, &evolveOut, &evolveErr)
		want := commentaudit.Main(args, &toolOut, &toolErr, commentaudit.ExecGit{})
		if got != 2 || got != want || evolveOut.String() != toolOut.String() || evolveErr.String() != toolErr.String() {
			t.Errorf("runComments(%q) = %d %q %q; commentaudit.Main = %d %q %q", args, got, evolveOut.String(), evolveErr.String(), want, toolOut.String(), toolErr.String())
		}
	}
}
