package inboxstamps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type Claimed struct {
	Path      string
	ClaimPath string
}

func (w worktree) claimOf(status, path string) (Claimed, bool) {
	if status != " D" || filepath.Dir(path) != filepath.Clean(Pathspec) {
		return Claimed{}, false
	}
	claimPath, held := w.claims[filepath.Base(path)]
	return Claimed{Path: path, ClaimPath: claimPath}, held
}

func claimsByName(root string) map[string]string {
	byName := map[string]string{}
	for _, dir := range inboxbatch.ProcessingCycleDirs(filepath.Join(root, Pathspec)) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		rel := strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(dir, root)), "/")
		for _, e := range entries {
			if !e.IsDir() {
				byName[e.Name()] = rel + "/" + e.Name()
			}
		}
	}
	return byName
}
