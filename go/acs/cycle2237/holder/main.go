//go:build acs

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
)

type packageRun struct {
	unlinkSocketDir bool
	holdAfter       bool
}

func (p packageRun) Run() int {
	in := bufio.NewReader(os.Stdin)
	fmt.Printf("ready %s\n", bridge.DeriveTestSocket(os.Getpid()))
	if _, err := in.ReadString('\n'); err != nil {
		return 2
	}
	if p.unlinkSocketDir {
		if err := os.RemoveAll(filepath.Join(os.Getenv("TMUX_TMPDIR"), "tmux-"+strconv.Itoa(os.Getuid()))); err != nil {
			return 3
		}
	}
	fmt.Println("proceeded")
	if p.holdAfter {
		_, _ = in.ReadString('\n')
	}
	return 0
}

func main() {
	run := packageRun{}
	for _, arg := range os.Args[1:] {
		switch arg {
		case "unlink":
			run.unlinkSocketDir = true
		case "hold":
			run.holdAfter = true
		}
	}
	os.Exit(tmuxtest.Main(run))
}
