//go:build notmux

package tmuxtest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

var strayCheck = strayServers

func serve(m runner, socket string) int {
	code := m.Run()
	stray, err := strayCheck(socket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: check for a stray tmux server: %v\n", err)
		return max(code, 1)
	}
	if len(stray) == 0 {
		return code
	}
	fmt.Fprintf(os.Stderr, "tmuxtest: tmux reached at the floor: a test started a server on %s\n", strings.Join(stray, ", "))
	if err := stopFamily(socket); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
	}
	return max(code, 1)
}

func strayServers(socket string) ([]string, error) {
	entries, err := os.ReadDir(bridge.TmuxSocketDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var found []string
	for _, e := range entries {
		if e.Name() == socket || strings.HasPrefix(e.Name(), socket+"-") {
			found = append(found, e.Name())
		}
	}
	return found, nil
}
