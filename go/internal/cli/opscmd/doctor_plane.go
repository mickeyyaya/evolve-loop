package opscmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

var absPath = filepath.Abs

func runDoctorPlane(args []string, stdout, stderr io.Writer) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	abs, err := absPath(root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor plane: %v\n", err)
		return exitFail
	}
	info, err := plane.Classify(abs)
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor plane: %v\n", err)
		return exitFail
	}
	kind := "PRIMARY checkout (console plane)"
	if info.IsLinkedWorktree {
		kind = "linked worktree (runtime plane)"
	}
	branch := info.Branch
	if branch == "" {
		branch = "(detached)"
	}
	fmt.Fprintf(stdout, "plane: %s\nbranch: %s\ngitdir: %s\n", kind, branch, info.GitDir)
	if _, serr := os.Stat(filepath.Join(abs, ".evolve", "state.json")); serr == nil && !info.IsLinkedWorktree {
		fmt.Fprintln(stderr, "WARN: loop state lives in the PRIMARY checkout — operator activity here can kill lanes; migrate the loop to a dedicated runtime worktree (ADR-0080)")
		return exitRuntime
	}
	return exitOK
}
