package commitprefixgate

import (
	"context"
	"os/exec"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// execGit is a tiny indirection so the test file can leave defaultGetDiffPaths
// alone (tests override via GetDiffPaths seam).
var execGit = func(args ...string) *exec.Cmd {
	return sysexec.Command(context.Background(), "git", args...)
}
