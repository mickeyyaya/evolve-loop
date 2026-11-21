//go:build darwin

package faketmux

import "testing"

func TestFakeTmux_PaneCommandIsArgv0OnDarwin(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.must("new-session", "-d", "-s", "rn", "/bin/bash", "-c", "exec -a renamedproc /bin/sleep 30")

	awaitCommand(t, f, "rn", "renamedproc")
}
