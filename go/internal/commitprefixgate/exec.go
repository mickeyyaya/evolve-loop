package commitprefixgate

import "os/exec"

// execGit is a tiny indirection so the test file can leave defaultGetDiffPaths
// alone (tests override via GetDiffPaths seam).
var execGit = func(args ...string) *exec.Cmd {
	return exec.Command("git", args...)
}
