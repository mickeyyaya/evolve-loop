package opscmd

// doctor_plane.go — ADR-0080 S2: `evolve doctor plane [root]` reports which
// worktree plane a checkout occupies (runtime linked worktree vs the primary
// console checkout) and whether loop state lives there. The operator-facing
// half of the boot-time tripwire in cmd_loop.
import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

func runDoctorPlane(args []string, stdout, stderr io.Writer) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor plane: %v\n", err)
		return 1
	}
	info, err := plane.Classify(abs)
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor plane: %v\n", err)
		return 1
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
		return 2
	}
	return 0
}
