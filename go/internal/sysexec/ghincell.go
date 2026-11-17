package sysexec

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

var ErrGhInCell = errors.New("sysexec: gh does not run inside a phase cell: the pipeline does GitHub work outside the cell")

const ghBinary = `gh`

func refusedInCell(name string) error {
	if filepath.Base(name) == ghBinary && ipcenv.InCell(os.Getenv) {
		return ErrGhInCell
	}
	return nil
}
