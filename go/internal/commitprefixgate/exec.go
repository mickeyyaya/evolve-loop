package commitprefixgate

import (
	"context"
	"os/exec"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var execGit = func(args ...string) *exec.Cmd {
	return sysexec.Command(context.Background(), "git", args...)
}
