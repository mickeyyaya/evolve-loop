//go:build integration && (darwin || linux)

package wake

import (
	"context"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestWaiter_ProcessExitWakesTheWaiter(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := sysexec.Command(ctx, "/bin/cat")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	w := newHostWaiter(t, nil)
	if err := w.Arm(Targets{Pids: []int{pid}}); err != nil {
		t.Fatalf("Arm = %v, want nil", err)
	}

	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	got := waitFor(t, w, wakeBound)
	if err := cmd.Wait(); err != nil {
		t.Errorf("child = %v, want exit 0", err)
	}

	if !slices.Equal(got.Exited, []int{pid}) || got.Deadline {
		t.Errorf("Wait after the child exit = %+v, want the exit of %d", got, pid)
	}
}
