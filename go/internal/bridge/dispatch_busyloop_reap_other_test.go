//go:build !darwin

package bridge

import (
	"os/exec"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

func fallbackLister(l proctree.Lister) proctree.Lister {
	return l
}

func psArgsOrFallback() ([]byte, error) {
	return exec.Command("ps", "-axww", "-o", "args").Output()
}
