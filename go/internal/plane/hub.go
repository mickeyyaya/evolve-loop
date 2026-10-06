package plane

import (
	"fmt"
	"path/filepath"
)

const OriginMainRef = "refs/remotes/origin/main"

type Hub struct {
	Root  string
	Store string
}

func ResolveHub(projectRoot string) (Hub, error) {
	info, err := Classify(projectRoot)
	if err != nil {
		return Hub{}, err
	}
	store, err := CommonGitDir(info)
	if err != nil {
		return Hub{}, err
	}
	if filepath.Base(store) == ".git" {
		return Hub{}, fmt.Errorf("%s is not a hub worktree: dev worktrees need the bare-store layout of docs/operations/workspace-layout.md", projectRoot)
	}
	return Hub{Root: filepath.Dir(store), Store: store}, nil
}

func (h Hub) DevDir() string { return filepath.Join(h.Root, "dev") }
