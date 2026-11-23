package bridge

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func HookEvolveDir(workingDir string) string { return filepath.Join(workingDir, ".evolve") }

func resolveWorkingDir(worktree string) string {
	if worktree != "" {
		return worktree
	}
	cwd, _ := os.Getwd()
	return cwd
}

type denialWindow struct {
	stream string
	cell   string
	offset int64
}

func openDenialWindow(workingDir string) denialWindow {
	w := denialWindow{cell: workingDir, stream: filepath.Join(HookEvolveDir(workingDir), signalcenter.StreamFileName)}
	if info, err := os.Stat(w.stream); err == nil {
		w.offset = info.Size()
	}
	return w
}

func (w denialWindow) refusal(code int) launchoutcome.Refusal {
	if code != launchoutcome.ExitArtifactTimeout {
		return launchoutcome.Refusal{}
	}
	chunk, err := signalcenter.ReadStream(w.stream, w.offset)
	if err != nil {
		return launchoutcome.Refusal{}
	}
	return launchoutcome.GuardRefusal(chunk.Events, w.cell)
}
