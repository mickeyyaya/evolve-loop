package cycleoutcome

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/acspathscan"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func proveProtectedRoute(in FailureInputs, committed []string) ([]acspathscan.Need, string) {
	route, ok := committedset.ReadProtectedRoute(in.Workspace)
	switch {
	case !ok:
		return nil, "no route artifact in the workspace"
	case route.Cycle != in.Cycle:
		return nil, fmt.Sprintf("the artifact is for cycle %d", route.Cycle)
	case route.ItemID != in.RefusalSubject || !slices.Equal(committed, []string{route.ItemID}):
		return nil, "the artifact item is not the one committed item"
	case in.Protected == nil:
		return nil, "no membership test is wired"
	}
	needs, err := acspathscan.Needs(filepath.Join(in.Workspace, committedset.ProtectedPredicatesDir), in.Protected)
	if err != nil {
		return nil, fmt.Sprintf("the kept predicates did not scan: %v", err)
	}
	if len(needs) == 0 {
		return nil, "no kept predicate re-proves a protected path"
	}
	return needs, ""
}

func protectedPaths(needs []acspathscan.Need) []string {
	paths := make([]string, 0, len(needs))
	for _, n := range needs {
		if !slices.Contains(paths, n.Path) {
			paths = append(paths, n.Path)
		}
	}
	return paths
}

func addProtectedFiles(item map[string]json.RawMessage, paths []string) {
	var files []string
	_ = json.Unmarshal(item[inboxbatch.ProtectedFilesField], &files)
	for _, p := range paths {
		if !slices.Contains(files, p) {
			files = append(files, p)
		}
	}
	item[inboxbatch.ProtectedFilesField], _ = json.Marshal(files)
}

func addItemProtectedFiles(projectRoot, id string, paths []string) error {
	loc, err := inboxmover.Locate(filepath.Join(projectRoot, ".evolve", "inbox"), id)
	if err != nil {
		return err
	}
	return inboxmover.UpdateItemJSON(loc.Path, func(item map[string]json.RawMessage) { addProtectedFiles(item, paths) })
}
