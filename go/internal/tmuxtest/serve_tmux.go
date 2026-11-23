//go:build !notmux

package tmuxtest

import (
	"fmt"
	"os"
	"os/exec"
)

func serve(m runner, socket string) int {
	if _, err := exec.LookPath("tmux"); err != nil {
		return m.Run()
	}
	if err := reapDeadTestServers(); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: reap dead test servers: %v\n", err)
	}
	if err := StartServer(socket); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
		return 1
	}
	if err := startOrphanGuard(socket, os.Getpid()); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
		_ = StopServer(socket)
		return 1
	}
	code := m.Run()
	if err := stopFamily(socket); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
		return max(code, 1)
	}
	return code
}
