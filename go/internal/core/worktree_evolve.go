package core

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type EvolveInvocation struct {
	Dir  string
	Args []string
	Env  []string
}

func WorktreeEvolveInvocation(worktree string, evolveArgs ...string) EvolveInvocation {
	return EvolveInvocation{
		Dir:  filepath.Join(worktree, "go"),
		Args: append([]string{"run", "./cmd/evolve"}, evolveArgs...),
		Env:  append(os.Environ(), ipcenv.WorktreeRootKey+"="+worktree),
	}
}
